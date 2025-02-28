package solana

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	budget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/time/rate"
)

var (
	// tokenProgram2022 is the address of the token program with extensions
	tokenProgram2022 = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")

	// memoProgram is the address of the memo program
	memoProgram = solana.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")

	// computeBudgetProgram is the address of the compute budget program
	computeBudgetProgram = solana.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111")

	// systemSig is an (apparant) system generated signature (this is the all 0 sig)
	systemSig = Signature{}

	// ErrSolanaNetworkNotSupported is returned when an unknown Solana network name is requested
	ErrSolanaNetworkNotSupported = errors.New("the provided network is not a valid Solana network")
	// ErrBurnTxNotFound is returned when we are trying to find a burn transaction
	ErrBurnTxNotFound = errors.New("burn transaction for the provided signature not found")
	// ErrMintSubmitFailed is returned when submitting (and waiting for confirmation of finalization)
	// of a solana mint tx fails
	ErrMintSubmitFailed = errors.New("failed to submit mint transaction to solana network")
)

// Override the default "old" token program to the token program 2022
func init() {
	token.SetProgramID(tokenProgram2022)
}

type (
	// Address of an account on the solana network
	Address = solana.PublicKey
	// Signature of a transaction on the solana network
	Signature = solana.Signature
	// Transaction on the solana network
	Transaction = solana.Transaction
	// ShortTxID is a shortened (hashed) solana tx hash made to fit in 32 bytes. This is a one way conversion.
	ShortTxID struct {
		hash [32]byte
	}
)

type Solana struct {
	network string

	rpcClient *rpc.Client
	wsClient  *ws.Client

	account solana.PrivateKey

	// The address of the token to use
	tokenAddress solana.PublicKey

	// txCache for GetTransaction result caching
	txCache *transactionCache
}

// New Solana client connected to the provided network
func New(ctx context.Context, cfg *SolanaConfig) (*Solana, error) {
	account, err := solana.PrivateKeyFromSolanaKeygenFile(cfg.KeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not load solana key file")
	}

	parsedTokenAddress, err := solana.PublicKeyFromBase58(cfg.TokenAddress)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse token address")
	}

	rpcClient, wsClient, err := getSolanaClient(ctx, cfg.NetworkName)
	if err != nil {
		return nil, errors.Wrap(err, "could not create Solana RPC client")
	}

	txCache := newTransactionCache()

	return &Solana{network: cfg.NetworkName, rpcClient: rpcClient, wsClient: wsClient, account: account, tokenAddress: parsedTokenAddress, txCache: txCache}, nil
}

// Address of the solana wallet
func (sol *Solana) Address() Address {
	return sol.account.PublicKey()
}

// GetTransaction loads the transaction for a given signature. If the tx exists, it is added to a cache to avoid future network calls for this sig.
func (sol *Solana) GetTransaction(ctx context.Context, sig Signature) (*rpc.GetTransactionResult, error) {
	// First check the cache
	if cachedTx := sol.txCache.getTransaction(sig); cachedTx != nil {
		return cachedTx, nil
	}

	maxTxVersion := uint64(0)
	// Not found in cache, load from rpc server
	txRes, err := sol.rpcClient.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		// This is the default commitment but set it explicitly to be sure
		Commitment:                     rpc.CommitmentFinalized,
		MaxSupportedTransactionVersion: &maxTxVersion,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to load mint transaction")
	}

	// If we have data, add it to the cache
	if txRes != nil {
		sol.txCache.addTransaction(sig, *txRes)
	}

	return txRes, nil
}

// IsMintTxID checks if a transaction ID is a known mint transaction.
//
// In other words, this checks if a given stellar tx ID has been used as a memo on solana to mint new tokens.
func (sol *Solana) IsMintTxID(ctx context.Context, txID string) (bool, error) {
	sigs, err := sol.listTransactionSigs(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to load token transaction signatures")
	}

	for _, sig := range sigs {
		txRes, err := sol.GetTransaction(ctx, sig)
		if err != nil {
			return false, err
		}
		tx, err := txRes.Transaction.GetTransaction()
		if err != nil {
			return false, errors.Wrap(err, "failed to decode tranasction")
		}
		if memoFromTx(*tx) == txID {
			return true, nil
		}
	}

	return false, nil
}

// GetRequiresSignatureCount to create a solana transaction
func (sol *Solana) GetRequiresSignatureCount(ctx context.Context) (int64, error) {
	var mint token.Mint
	err := sol.rpcClient.GetAccountDataInto(ctx, sol.tokenAddress, &mint)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get token account info")
	}

	if mint.MintAuthority == nil {
		return 0, errors.New("can't mint token without mint authority")
	}

	var ma token.Multisig
	err = sol.rpcClient.GetAccountDataInto(ctx, *mint.MintAuthority, &ma)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get mint authority multisig info")
	}

	// TODO: is this correct to figure out if the mint is multisig?
	if !ma.IsInitialized {
		return 1, nil
	}

	return int64(ma.M), nil
}

func (sol *Solana) GetSigners(ctx context.Context) ([]Address, error) {
	var mint token.Mint
	err := sol.rpcClient.GetAccountDataInto(ctx, sol.tokenAddress, &mint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get token account info")
	}

	if mint.MintAuthority == nil {
		return nil, errors.New("can't mint token without mint authority")
	}

	var ma token.Multisig
	err = sol.rpcClient.GetAccountDataInto(ctx, *mint.MintAuthority, &ma)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mint authority multisig info")
	}

	// TODO: is this correct to figure out if the mint is multisig?
	if !ma.IsInitialized {
		return []Address{*mint.MintAuthority}, nil
	}

	return ma.Signers[:ma.N], nil
}

// IsValidReceiver checks if the target address is a valid PDA set up to receive our specific Mint
func (sol *Solana) IsValidReceiver(ctx context.Context, receiver Address) (bool, error) {
	resp, err := sol.rpcClient.GetAccountInfo(ctx, receiver)
	if err != nil {
		if errors.Is(err, rpc.ErrNotFound) {
			// Account does not exist, so it's definitely not a valid account
			return false, nil
		}

		return false, errors.Wrap(err, "could not load account info")
	}

	var account token.Account
	err = ag_binary.NewBinDecoder(resp.Value.Data.GetBinary()).Decode(&account)
	if err != nil {
		log.Info().Err(err).Str("Account", receiver.String()).Msg("Could not decode associated token account, invalid account")
		return false, nil
	}

	if account.Mint != sol.tokenAddress {
		return false, nil
	}

	return true, nil
}

func (sol *Solana) CreateTokenSignature(tx Transaction) (Signature, int, error) {
	// First clear possible existing signatures so we can isolate the signature we generated
	tx.Signatures = nil
	sigs, err := tx.PartialSign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if sol.account.PublicKey().Equals(key) {
				return &sol.account
			}

			return nil
		})
	if err != nil {
		return Signature{}, 0, errors.Wrap(err, "could not sign transaction")
	}

	sigCount := 0
	idx := 0
	signature := Signature{}
	for i, sig := range sigs {
		if sig != [64]byte{} {
			signature = sig
			sigCount++
			idx = i
		}
	}

	switch sigCount {
	case 0:
		return Signature{}, 0, errors.New("no transaction signatures generated")
	case 1:
		return signature, idx, nil
	default:
		return Signature{}, 0, errors.New("generated more than 1 transaction signature")
	}
}

// GetBurnTransaction on the solona network with the provided txId
func (sol *Solana) GetBurnTransaction(ctx context.Context, txID ShortTxID) (Burn, error) {
	sigs, err := sol.listTransactionSigs(ctx)
	if err != nil {
		return Burn{}, errors.Wrap(err, "failed to load token transaction signatures")
	}

	for _, sig := range sigs {
		if shortenTxID(sig) == txID {
			txRes, err := sol.GetTransaction(ctx, sig)
			if err != nil {
				return Burn{}, errors.Wrap(err, "failed to load burn transaction")
			}
			tx, err := txRes.Transaction.GetTransaction()
			if err != nil {
				return Burn{}, errors.Wrap(err, "failed to decode tranasction")
			}
			burn, err := burnFromTransaction(*tx)
			if err != nil {
				return Burn{}, errors.Wrap(err, "failed to parse burn transaction")
			}
			return burn, nil
		}
	}

	return Burn{}, ErrBurnTxNotFound
}

// Mint tokens on the solana network
func (sol *Solana) Mint(ctx context.Context, tx *Transaction) error {
	sig, err := confirm.SendAndConfirmTransaction(ctx, sol.rpcClient, sol.wsClient, tx)
	if err != nil {
		log.Error().Err(err).Msg("could not submit solana mint transaction")

		return errors.Wrap(err, "failed to submit mint transaction")
	}

	log.Info().Str("txID", sig.String()).Msg("Submitted mint tx")

	return nil
}

// Converts a base58 encoded transaction signature to shorter 32 byte ShortTxId.
func shortenTxID(sig Signature) ShortTxID {
	// rawSig := solana.MustSignatureFromBase58(input)
	return ShortTxID{hash: blake2b.Sum256(sig[:])}
}

// listTransactionSigs for the token address.
func (sol *Solana) listTransactionSigs(ctx context.Context) ([]Signature, error) {
	sigs, err := sol.rpcClient.GetSignaturesForAddress(ctx, sol.tokenAddress)
	if err != nil {
		return nil, errors.Wrap(err, "could not load token signatures")
	}

	signatures := make([]Signature, 0, len(sigs))

	for _, sig := range sigs {
		// Skip transactions which errored
		if sig.Err == nil {
			signatures = append(signatures, sig.Signature)
		}
	}

	return signatures, nil
}

// AddressFromHex decodes a hex encoded Solana address
func AddressFromHex(encoded string) (Address, error) {
	b, err := hex.DecodeString(encoded)
	if err != nil {
		return Address{}, errors.Wrap(err, "could not decode hex encoded address")
	}
	var address Address
	copy(address[:], b)
	return address, nil
}

// AddressFromB64 decodes a base 64 encoded Solana address
func AddressFromB64(encoded string) (Address, error) {
	var address Address
	n, err := base64.StdEncoding.Decode(address[:], []byte(encoded))
	if err != nil {
		return Address{}, errors.Wrap(err, "could not decode hex encoded address")
	}
	if n != len(Address{}) {
		return Address{}, errors.New("incomplete address")
	}
	return address, nil
}

// PrepareMintTx creates a new mint transaction on solana with the provided values.
func (sol *Solana) PrepareMintTx(ctx context.Context, info MintInfo) (*Transaction, error) {
	to := solana.PublicKeyFromBytes(info.To[:])

	if len(info.TxID) != 64 {
		return nil, errors.New("invalid txid length")
	}

	recent, err := sol.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest finalized block hash")
	}

	var mint token.Mint
	err = sol.rpcClient.GetAccountDataInto(ctx, sol.tokenAddress, &mint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get token account info")
	}

	if mint.MintAuthority == nil {
		return nil, errors.New("can't mint token without mint authority")
	}

	signers, err := sol.GetSigners(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get solana mint signers")
	}

	filteredSigners := make([]Address, 0, len(info.OnlineSigners))
	for _, signer := range signers {
		for _, os := range info.OnlineSigners {
			if signer.Equals(os) {
				filteredSigners = append(filteredSigners, signer)
			}
		}
	}

	txID := [64]byte{}
	copy(txID[:], []byte(info.TxID))

	tx, err := solana.NewTransaction([]solana.Instruction{
		// TODO: Compute actual limit
		budget.NewSetComputeUnitLimitInstruction(40000).Build(),
		CustomMemoInstruction(txID, sol.account.PublicKey()),
		// memo.NewMemoInstruction(txID, sol.account.PublicKey()).Build(),
		token.NewMintToCheckedInstruction(info.Amount, mint.Decimals, sol.tokenAddress, to, *mint.MintAuthority, filteredSigners).Build(),
	}, recent.Value.Blockhash, solana.TransactionPayer(sol.account.PublicKey()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create mint transaction")
	}

	return tx, nil
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

	extractBurnFromSig := func(sig solana.Signature) *Burn {
		log.Debug().Str("signature", sig.String()).Msg("Fetch tx with sig")
		res, err := sol.GetTransaction(ctx, sig)
		if err != nil {
			if errors.Is(err, rpc.ErrNotFound) {
				// TODO: Considering we get the actual log twice, there might be a better way to handle this
				log.Info().Str("signature", sig.String()).Msg("Skipping tx which can't be found")
				return nil
			}
			log.Error().Err(err).Str("signature", sig.String()).Msg("Could not fetch transaction")
			// TODO: Perhaps we should retry here?
			return nil
		}

		tx, err := res.Transaction.GetTransaction()
		if err != nil {
			log.Err(err).Str("signature", sig.String()).Msg("Failed to decode transaction")
			return nil
		}

		// TODO: Compute limit is optional
		ixLen := len(tx.Message.Instructions)
		if len(tx.Message.Instructions) != 3 {
			log.Debug().Int("ixLen", ixLen).Str("signature", sig.String()).Msg("Skipping Tx which did not have the expected 3 instructions")
			return nil
		}

		memoText := ""
		burnAmount := uint64(0)
		tokenDecimals := uint8(0)
		source := Address{}
		illegalOp := false

	outer:
		for _, ix := range tx.Message.Instructions {
			switch tx.Message.AccountKeys[ix.ProgramIDIndex] {
			case memoProgram:
				// TODO: verify encoding
				if len(ix.Data) == 0 {
					log.Debug().Msg("Empty memo instruction")
					illegalOp = true
					break
				}
				memoText = string(ix.Data[:])
			case tokenProgram2022:
				accounts, err := ix.ResolveInstructionAccounts(&tx.Message)
				if err != nil {
					log.Error().Err(err).Str("signature", sig.String()).Msg("Failed to resolve token accounts")
					break
				}
				tokenIx, err := token.DecodeInstruction(accounts, ix.Data)
				if err != nil {
					// TODO: Is this technically an error?
					log.Error().Err(err).Str("signature", sig.String()).Msg("Failed to decode token instruction")
					illegalOp = true
					break
				}

				// At this point, verify its a burn
				// TODO: it seems burnchecked is returned but maybe we also need to check for regular `burn`
				burn, ok := tokenIx.Impl.(*token.BurnChecked)
				if !ok {
					log.Info().Str("signature", sig.String()).Msg("Skipping tx since token IX is not of type burnChecked")
					// Since we validate IX len, if this is not a valid burn operation there can't be another one.
					illegalOp = true
					break outer
				}
				if burn.Amount == nil {
					log.Info().Str("signature", sig.String()).Msg("Skipping tx since token IX is burnChecked, but without an amount set")
					illegalOp = true
					break outer
				}
				if burn.Decimals == nil {
					log.Info().Str("signature", sig.String()).Msg("Skipping tx since token IX is burnChecked, but without decimals set")
					illegalOp = true
					break outer
				}
				source = burn.GetSourceAccount().PublicKey
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
			return &Burn{amount: burnAmount, decimals: tokenDecimals, memo: memoText, caller: source, signature: sig}
		}

		return nil
	}

	ch := make(chan Burn, 10)
	go func() {
		// Close the channel in case the goroutine exits
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			sub, err := sol.wsClient.LogsSubscribeMentions(sol.tokenAddress, rpc.CommitmentFinalized)
			if err != nil {
				log.Error().Err(err).Msg("Failed to open solana log subscription")
				// Reconnect the websocket, in case the websocket itself is closed.
				sol.wsClient.Close()
				sol.wsClient, err = newSolanaWsClient(ctx, sol.network)
				if err != nil {
					log.Error().Err(err).Str("network", sol.network).Msg("Failed to create new solana websocket connection")
				}
				// Wait 10 seconds in case it is a transient network error, then try again
				time.Sleep(time.Second * 10)
				// Restart the loop
				continue
			}

			for {
				got, err := sub.Recv(ctx)
				if err != nil {
					log.Error().Err(err).Msg("Failed to get new tx logs from subscription")
					break
				}

				if got.Value.Signature.Equals(systemSig) {
					log.Debug().Msg("Skipping logs for system tx")
					continue
				}

				burn := extractBurnFromSig(got.Value.Signature)
				if burn != nil {
					ch <- *burn
				}

			}

			// Also close the subscription now that we are done with it
			sub.Unsubscribe()
		}
	}()

	// Start loading historic signatures
	// NOTE: This is done after starting the subscription to avoid a possible minor gap.
	sigs, err := sol.listTransactionSigs(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not load historic token tx signatures")
	}
	go func() {
		for _, sig := range sigs {
			burn := extractBurnFromSig(sig)
			if burn != nil {
				ch <- *burn
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
	config, err := getNetworkConfig(network)
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

func getNetworkConfig(network string) (rpc.Cluster, error) {
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

	return config, err
}

func newSolanaWsClient(ctx context.Context, network string) (*ws.Client, error) {
	config, err := getNetworkConfig(network)
	if err != nil {
		return nil, errors.Wrap(err, "could not open websocket connection")
	}
	wsClient, err := ws.Connect(ctx, config.WS)
	if err != nil {
		return nil, errors.Wrap(err, "failed to establish websocket connection")
	}

	return wsClient, nil
}

func NewShortTxID(hash [32]byte) ShortTxID {
	return ShortTxID{hash: hash}
}

// String implements the Stringer interface
func (stid ShortTxID) String() string {
	return hex.EncodeToString(stid.hash[:])
}

// Hash returns the inner hash
func (stid ShortTxID) Hash() [32]byte {
	return stid.hash
}

// ExtractMintValues extracts the amount in lamports, and destination of a mint on solana
func ExtractMintvalues(tx Transaction) (int64, string, Address, error) {
	var amount int64
	var memostring string
	var receiver Address

	// Validate other request params
	if len(tx.Message.Instructions) != 3 {
		return amount, memostring, receiver, errors.New("invalid transaction instruction count")
	}

	for _, ix := range tx.Message.Instructions {
		switch tx.Message.AccountKeys[ix.ProgramIDIndex] {
		case memoProgram:
			// TODO: verify encoding
			if memostring != "" {
				return amount, memostring, receiver, errors.New("mint memo already set, duplicate instruction")
			}
			if len(ix.Data) != 64 {
				return amount, memostring, receiver, errors.New(fmt.Sprintf("mint memo has invalid length %d", len(ix.Data)))
			}

			memostring = string(ix.Data[:])
		case tokenProgram2022:
			accounts, err := ix.ResolveInstructionAccounts(&tx.Message)
			if err != nil {
				return amount, memostring, receiver, errors.Wrap(err, "could not resolve instruction accounts")
			}
			tokenIx, err := token.DecodeInstruction(accounts, ix.Data)
			if err != nil {
				// TODO: Is this technically an error?
				return amount, memostring, receiver, errors.Wrap(err, "could not decode token instruction")
			}

			// At this point, verify its a mint
			mint, ok := tokenIx.Impl.(*token.MintToChecked)
			if !ok {
				// Since we validate IX len, if this is not a valid mint operation there can't be another one.
				return amount, memostring, receiver, errors.Wrap(err, "could not decode token instruction to mint instruction")
			}
			if mint.Amount == nil {
				return amount, memostring, receiver, errors.New("mint has no value set")
			}
			if mint.Decimals == nil {
				return amount, memostring, receiver, errors.New("mint has no decimals set")
			}
			if amount != 0 {
				return amount, memostring, receiver, errors.New("mint amount already set, duplicate instruction")
			}
			amount = int64(*mint.Amount)
			receiver = mint.GetDestinationAccount().PublicKey
		case computeBudgetProgram:
		// Nothing really to do here, we only care that this is ineed a compute budget program ix
		default:
			// We don't allow for other instructions at this time, so this condition is terminal for the tx validation.
			return amount, memostring, receiver, errors.New("unknown instruction")
		}
	}

	return amount, memostring, receiver, nil
}

func memoFromTx(tx Transaction) string {
	for _, ix := range tx.Message.Instructions {
		switch tx.Message.AccountKeys[ix.ProgramIDIndex] {
		case memoProgram:
			// TODO: verify encoding
			return string(ix.Data[1:])
		default:
			continue
		}
	}

	return ""
}

func CustomMemoInstruction(txID [64]byte, signer solana.PublicKey) *memo.MemoInstruction {
	nd := CustomMemoVariant{
		AccountMetaSlice: make(solana.AccountMetaSlice, 1),
	}

	nd.AccountMetaSlice[0] = solana.Meta(signer).SIGNER()
	nd.Message = txID

	return &memo.MemoInstruction{
		BaseVariant: ag_binary.BaseVariant{
			Impl:   nd,
			TypeID: ag_binary.NoTypeIDDefaultID,
		},
	}
}

type CustomMemoVariant struct {
	// The memo message
	Message [64]byte

	// [0] = [SIGNER] Signer
	// ··········· The account that will pay for the transaction
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}
