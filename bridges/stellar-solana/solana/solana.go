package solana

import (
	"context"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gagliardetto/solana-go"
	budget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

var (
	// ErrSolanaNetworkNotSupported is returned when an unknown Solana network name is requested
	ErrSolanaNetworkNotSupported = errors.New("the provided network is not a valid Solana network")

	// tokenProgram2022 is the address of the token program with extensions
	tokenProgram2022 = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")

	// memoProgram is the address of the memo program
	memoProgram = solana.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")

	// tftAddress is the address of the tft token on chain, hardcoded for now
	// tftAddress = solana.MustPublicKeyFromBase58("tftu9NtpEyxfsT1ggw3e5ZEyctC8yYz4CVz9GyAyGV7")
	tftAddress = solana.MustPublicKeyFromBase58("TFT7gjfh2yatov3nnuwHmG8pEU5Y9xAditVymo74iag")

	// computeBudgetProgram is the address of the compute budget program
	computeBudgetProgram = solana.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111")

	// systemSig is an (apparant) system generated signature
	systemSig = solana.MustSignatureFromBase58("1111111111111111111111111111111111111111111111111111111111111111")
)

// Override the default "old" token program to the token program 2022
func init() {
	token.SetProgramID(tokenProgram2022)
}

type Solana struct {
	rpcClient *rpc.Client
	wsClient  *ws.Client

	account solana.PrivateKey
}

// New Solana client connected to the provided network
func New(ctx context.Context, network string, keyFile string) (*Solana, error) {
	account, err := solana.PrivateKeyFromSolanaKeygenFile(keyFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not load solana key file")
	}

	rpcClient, wsClient, err := getSolanaClient(ctx, network)
	if err != nil {
		return nil, errors.Wrap(err, "could not create Solana RPC client")
	}

	return &Solana{rpcClient: rpcClient, wsClient: wsClient, account: account}, nil
}

// MintTokens tries to mint new tokens with the given mint context.
func (sol *Solana) MintTokens(ctx context.Context, info MintInfo) error {
	to := solana.PublicKeyFromBytes(info.To[:])

	recent, err := sol.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return errors.Wrap(err, "failed to get latest finalized block hash")
	}

	var mint token.Mint
	err = sol.rpcClient.GetAccountDataInto(ctx, tftAddress, &mint)
	if err != nil {
		return errors.Wrap(err, "failed to get token account info")
	}

	if mint.MintAuthority == nil {
		return errors.New("can't mint token without mint authority")
	}

	spew.Dump(mint)

	tx, err := solana.NewTransaction([]solana.Instruction{
		// TODO: Compute actual limit
		budget.NewSetComputeUnitLimitInstruction(40000).Build(),
		memo.NewMemoInstruction([]byte(info.TxID), sol.account.PublicKey()).Build(),
		token.NewMintToCheckedInstruction(info.Amount, mint.Decimals, tftAddress, to, *mint.MintAuthority, nil).Build(),
	}, recent.Value.Blockhash, solana.TransactionPayer(sol.account.PublicKey()))
	if err != nil {
		return errors.Wrap(err, "failed to create mint transaction")
	}

	spew.Dump(tx)

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if sol.account.PublicKey().Equals(key) {
			return &sol.account
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to sign mint transaction")
	}

	spew.Dump(tx)

	_, err = confirm.SendAndConfirmTransaction(ctx, sol.rpcClient, sol.wsClient, tx)
	if err != nil {
		return errors.Wrap(err, "failed to submit mint transaction")
	}

	log.Info().Msg("Submitted mint tx")

	return nil
}

// SubscribeTokenBurns creates a subscription for **NEW** token burn events on the current token. This does not return any previous burns.
func (sol *Solana) SubscribeTokenBurns(ctx context.Context) (<-chan Burn, error) {
	// There isn't really a direct way to get just burns. Instead we do the following:
	// - Subscribe to logs, which mention the token address
	// - For every event, extract the signature. The signature can be used to load the full transaction.
	// - It seems there are 3 log events being emitted
	//    - The first one carries the systemSig, and should be ignored as we can't load a meaningfull transaction with this.
	//    - The second one carries the actual signature. However if we immediately try and fetch the transaction with this signature,
	//      there is a chance the transaction is not found.
	//    - The third seems to be a duplicate of the second, but in the next slot. At this point in time, the signature can be used to
	//      load the transaction.
	//      TODO: Check if adding a small delay after receiving the second log but before fetching the tx causes it to succeed.
	//
	// - Once we have the tx, check if its a burn tx. Notice that we will __require__ a memo to be sent to identify the token destination.
	//    - Validate there are 3 instruction in the TX:
	//      - One will be the memo instruction, the data is the actual memo.
	//      - One will be a token instruction. Try to parse this as a burn instruction to extract the value.
	//      - One is compute budget, we don't care for this.
	sub, err := sol.wsClient.LogsSubscribeMentions(tftAddress, rpc.CommitmentFinalized)
	if err != nil {
		return nil, errors.Wrap(err, "failed to subscribe to token program errors")
	}

	ch := make(chan Burn, 10)
	go func() {
		// Close the channel in case the goroutine exits
		defer close(ch)
		// Also close the subscription in this case
		defer sub.Unsubscribe()

		for {
			got, err := sub.Recv(ctx)
			if err != nil {
				log.Error().Err(err).Msg("Failed to get new tx logs from subscription")
				return
			}

			if got.Value.Signature.Equals(systemSig) {
				log.Debug().Msg("Skipping logs for system tx")
				continue
			}

			log.Debug().Str("signature", got.Value.Signature.String()).Msg("Fetch tx with sig")
			res, err := sol.rpcClient.GetTransaction(ctx, got.Value.Signature, nil)
			if err != nil {
				if errors.Is(err, rpc.ErrNotFound) {
					// TODO: Considering we get the actual log twice, there might be a better way to handle this
					log.Info().Str("signature", got.Value.Signature.String()).Msg("Skipping tx which can't be found")
					continue
				}
				log.Error().Err(err).Str("signature", got.Value.Signature.String()).Msg("Could not fetch transaction")
				// TODO: Perhaps we should retry here?
				continue
			}

			tx, err := res.Transaction.GetTransaction()
			if err != nil {
				log.Err(err).Str("signature", got.Value.Signature.String()).Msg("Failed to decode transaction")
				continue
			}

			spew.Dump(tx)

			// TODO: Compute limit is optional
			ixLen := len(tx.Message.Instructions)
			if len(tx.Message.Instructions) != 3 {
				log.Debug().Int("ixLen", ixLen).Str("signature", got.Value.Signature.String()).Msg("Skipping Tx which did not have the expected 3 instructions")
				continue
			}

			memoText := ""
			burnAmount := uint64(0)
			tokenDecimals := uint8(0)
			illegalOp := false

		outer:
			for _, ix := range tx.Message.Instructions {
				accounts, err := tx.AccountMetaList()
				if err != nil {
					log.Error().Err(err).Str("signature", got.Value.Signature.String()).Msg("Failed to resolve account meta list")
					break
				}

				switch tx.Message.AccountKeys[ix.ProgramIDIndex] {
				case memoProgram:
					// TODO: verify encoding
					memoText = string(ix.Data)
				case tokenProgram2022:
					tokenIx, err := token.DecodeInstruction(accounts, ix.Data)
					if err != nil {
						// TODO: Is this technically an error?
						log.Error().Err(err).Str("signature", got.Value.Signature.String()).Msg("Failed to decode token instruction")
						illegalOp = true
						break
					}

					// At this point, verify its a burn
					// TODO: it seems burnchecked is returned but maybe we also need to check for regular `burn`
					burn, ok := tokenIx.Impl.(*token.BurnChecked)
					if !ok {
						log.Info().Str("signature", got.Value.Signature.String()).Msg("Skipping tx since token IX is not of type burnChecked")
						// Since we validate IX len, if this is not a valid burn operation there can't be another one.
						illegalOp = true
						break outer
					}
					if burn.Amount == nil {
						log.Info().Str("signature", got.Value.Signature.String()).Msg("Skipping tx since token IX is burnChecked, but without an amount set")
						illegalOp = true
						break outer
					}
					if burn.Decimals == nil {
						log.Info().Str("signature", got.Value.Signature.String()).Msg("Skipping tx since token IX is burnChecked, but without decimals set")
						illegalOp = true
						break outer
					}
					burnAmount = *burn.Amount
					tokenDecimals = *burn.Decimals
				case computeBudgetProgram:
				// Nothing really to do here, we only care that this is ineed a compute budget program ix
				default:
					// We don't allow for other instructions at this time, so this condition is terminal for the tx validation.
					illegalOp = true
					break outer

				}
			}

			if memoText != "" && burnAmount != 0 && !illegalOp {
				ch <- Burn{amount: burnAmount, decimals: tokenDecimals, memo: memoText}
			}

		}
	}()

	return ch, nil
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
