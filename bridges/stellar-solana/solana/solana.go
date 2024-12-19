package solana

import (
	"context"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

var (
	// ErrSolanaNetworkNotSupported is returned when an unknown Solana network name is requested
	ErrSolanaNetworkNotSupported = errors.New("the provided network is not a valid Solana network")

	// tokenProgram2022 is the address of the token program with extensions
	tokenProgram2022 = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
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
	sub, err := sol.wsClient.ProgramSubscribeWithOpts(tokenProgram2022, rpc.CommitmentFinalized, solana.EncodingBase64Zstd, nil)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to token program errors")
	}
	defer sub.Unsubscribe()

	for {
		got, err := sub.Recv(ctx)
		if err != nil {
			return err
		}
		spew.Dump(got)

		decodedBinary := got.Value.Account.Data.GetBinary()
		if decodedBinary != nil {
			spew.Dump(decodedBinary)
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
