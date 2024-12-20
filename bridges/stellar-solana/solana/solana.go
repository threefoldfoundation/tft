package solana

import (
	"context"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gagliardetto/solana-go"

	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

const ()

var (
	// ErrSolanaNetworkNotSupported is returned when an unknown Solana network name is requested
	ErrSolanaNetworkNotSupported = errors.New("the provided network is not a valid Solana network")

	// tokenProgram2022 is the address of the token program with extensions
	tokenProgram2022 = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")

	// memoProgram is the address of the memo program
	memoProgram = solana.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")

	// tftAddress is the address of the tft token on chain, hardcoded for now
	tftAddress = solana.MustPublicKeyFromBase58("tftu9NtpEyxfsT1ggw3e5ZEyctC8yYz4CVz9GyAyGV7")

	// systemSig is an (apparant) system generated signature
	systemSig = solana.MustSignatureFromBase58("1111111111111111111111111111111111111111111111111111111111111111")
)

type Solana struct {
	rpcClient *rpc.Client
	wsClient  *ws.Client
}

// New Solana client connected to the provided network
func New(ctx context.Context, network string) (*Solana, error) {
	rpcClient, wsClient, err := getSolanaClient(ctx, network)
	if err != nil {
		return nil, errors.Wrap(err, "could not create Solana RPC client")
	}

	return &Solana{rpcClient: rpcClient, wsClient: wsClient}, nil
}

func (sol *Solana) SubscribeTokenBurns(ctx context.Context) error {
	//sub, err := sol.wsClient.ProgramSubscribeWithOpts(tokenProgram2022, rpc.CommitmentFinalized, solana.EncodingBase64Zstd, nil)
	sub, err := sol.wsClient.LogsSubscribeMentions(tftAddress, rpc.CommitmentFinalized)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to token program errors")
	}
	defer sub.Unsubscribe()

	for {
		got, err := sub.Recv(ctx)
		if err != nil {
			return err
		}
		// spew.Dump(got)

		if got.Value.Signature.Equals(systemSig) {
			fmt.Println()
			fmt.Println("Skipping logs for system tx")
			fmt.Println()
			continue
		}

		fmt.Println()
		// fmt.Printf("%+#v\n", got)
		fmt.Println()

		// decodedBinary := got.Value.Account.Data.GetBinary()
		logs := got.Value.Logs
		for _, log := range logs {
			fmt.Println(log)
		}
		// decodedBinary := got.Value.Logs
		// if decodedBinary != nil {
		// 	spew.Dump(decodedBinary)
		// }

		fmt.Println("Fetch tx with sig", got.Value.Signature)
		res, err := sol.rpcClient.GetTransaction(ctx, got.Value.Signature, nil)
		if err != nil {
			if errors.Is(err, rpc.ErrNotFound) {
				fmt.Println("Skipping tx which can't be found")
				continue
			}
			fmt.Println(err)
			return err
		}

		spew.Dump(got)
		fmt.Println("full res dump")
		spew.Dump(res)

		tx, err := res.Transaction.GetTransaction()
		if err != nil {
			fmt.Println("failed to decode transaction", err)
			continue
		}

		for _, ix := range tx.Message.Instructions {
			accounts, err := tx.AccountMetaList()
			if err != nil {
				fmt.Println("could not resolve account meta list", err)
				break
			}

			switch tx.Message.AccountKeys[ix.ProgramIDIndex] {
			case memoProgram:
				fmt.Println()
				fmt.Println("Attempt to decode memo instruction")
				fmt.Println()
				// NOTE: for the memo program, data is the raw data and does not need to be decoded into an instruction
				memoIx, err := memo.DecodeInstruction(accounts, ix.Data)
				if err != nil {
					fmt.Println()
					fmt.Println("failed to decode memo instruction", string(ix.Data), err)
					fmt.Println()
					spew.Dump(ix)
					break
				}
				spew.Dump(memoIx)
			case tokenProgram2022:
				fmt.Println()
				fmt.Println("Attempt to decode token instruction")
				fmt.Println()
				tokenIx, err := token.DecodeInstruction(accounts, ix.Data)
				if err != nil {
					fmt.Println("failed to decode token instruction", err)
					spew.Dump(ix)
					break
				}

				spew.Dump(tokenIx)
				// At this point, verify its a burn
				// TODO: it seems burnchecked is returned but maybe we also need to check for regular `burn`
				burn, ok := tokenIx.Impl.(*token.BurnChecked)
				if !ok {
					// TODO: continue here in case there are multiple token ops in the same transaction? is that possible?
					continue
				}
				spew.Dump(burn)
			default:

			}
			// fmt.Println(ix)
			// fmt.Println()

		}

	}

	return nil
}

// Close the client terminating all subscriptions and open connections
func (sol *Solana) Close() error {
	sol.wsClient.Close()
	return sol.rpcClient.Close()
}

// getSolanaClient gets an RPC client and websocket client for a specific solana network
func getSolanaClient(ctx context.Context, network string) (*rpc.Client, *ws.Client, error) {
	var config rpc.Cluster
	var err error

	switch network {
	case "local":
		config = rpc.LocalNet
	case "devnet":
		config = rpc.DevNet
	case "testnet":
		config = rpc.TestNet
	case "production":
		config = rpc.MainNetBeta
	default:
		err = ErrSolanaNetworkNotSupported
	}

	if err != nil {
		return nil, nil, err
	}

	rpcClient := rpc.NewWithCustomRPCClient(rpc.NewWithLimiter(config.RPC, rate.Every(time.Second), 10))

	wsClient, err := ws.Connect(ctx, config.WS)
	if err != nil {
		rpcClient.Close()
		return nil, nil, errors.Wrap(err, "failed to establish websocket connection")
	}

	return rpcClient, wsClient, nil
}
