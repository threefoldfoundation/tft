package bridge

import (
	"context"
	"math/big"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/faults"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/p2p"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/solana"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/state"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/stellar"
)

const (
	// EthBlockDelay is the amount of blocks to wait before
	// pushing eth transaction to the stellar network
	EthBlockDelay = 3
	// Withdrawing from smartchain to Stellar fee
	WithdrawFee   = int64(1 * stellar.Precision) // WithdrawFeeof 1 TFT in Stroops
	BridgeNetwork = "stellar"
)

// Bridge is a high lvl structure which listens on contract events and bridge-related
// stellar transactions, and handles them
type Bridge struct {
	solanaWallet     *solana.Solana
	wallet           *stellar.Wallet
	blockPersistency *state.ChainPersistency
	mut              sync.Mutex
	config           *BridgeConfig
	synced           bool
	signersClient    *SignersClient
}

type BridgeConfig struct {
	RescanBridgeAccount bool
	RescanFromHeight    int64 // TODO: change to uint64
	PersistencyFile     string
	Follower            bool
	Relay               string
	Psk                 string
	// deposit fee in TFT units
	DepositFee int64
}

// NewBridge creates a new Bridge.
// TODO: context is not used
func NewBridge(ctx context.Context, wallet *stellar.Wallet, sol *solana.Solana, config *BridgeConfig, host host.Host, router routing.PeerRouting) (bridge *Bridge, err error) {
	blockPersistency := state.NewChainPersistency(config.PersistencyFile)

	bridge = &Bridge{
		solanaWallet:     sol,
		blockPersistency: blockPersistency,
		wallet:           wallet,
		config:           config,
	}
	// Only create the signer client if the bridge is running in master mode
	if !config.Follower {
		relayAddrInfo, addrErr := peer.AddrInfoFromString(config.Relay)
		if addrErr != nil {
			return nil, addrErr
		}
		cosigners, requiredSignatures, err := wallet.GetSigningRequirements()
		if err != nil {
			return nil, err
		}
		log.Info().Int("signatures", requiredSignatures).Msg("required Stellar signature count")
		wallet.SetRequiredSignatures(requiredSignatures)
		cosignerPeerIDs, err := p2p.GetPeerIDsFromStellarAddresses(cosigners)
		if err != nil {
			return nil, err
		}
		bridge.signersClient = NewSignersClient(host, router, cosignerPeerIDs, relayAddrInfo)

		wallet.SetSignerClient(bridge.signersClient)
	}

	if config.RescanBridgeAccount {
		log.Info().Msg("rescan triggered")
		// setting the cursor to 0 will trigger the bridge
		// to scan for every transaction ever made on the bridge account
		// and mint accordingly
		err = blockPersistency.SaveStellarCursor("0")
		if err != nil {
			return
		}
	}

	return
}

// Close bridge
// TODO: drop the error return value
func (bridge *Bridge) Close() error {
	bridge.mut.Lock()
	bridge.solanaWallet.Close()
	defer bridge.mut.Unlock() // TODO: move this directly after the Lock()
	return nil
}

func (bridge *Bridge) mint(ctx context.Context, memoAddress solana.Address, depositedAmount *big.Int, txID string) (err error) {
	if !bridge.synced {
		return errors.New("bridge is not synced, retry later")
	}

	// Check if this tx is a known mint TX
	log.Info().Str("receiver", memoAddress.String()).Str("txID", txID).Msg("Minting")
	// check if we already know this ID
	known, err := bridge.solanaWallet.IsMintTxID(ctx, txID)
	if err != nil {
		return
	}
	if known {
		log.Info().Str("txID", txID).Msg("Skipping known minting transaction")
		// we already know this withdrawal address, so ignore the transaction
		return
	}

	// Check if we've already refunded this TX
	known, err = bridge.wallet.TransactionStorage.TransactionWithMemoExists(ctx, txID)
	if err != nil {
		return
	}
	if known {
		log.Info().Str("txID", txID).Msg("Skipping minting transaction we've already refunded")
		// we already refunded this, so ignore the transaction
		return
	}

	// Convert receiver address to derived ATA
	receiver, err := bridge.solanaWallet.ATAFromMasterAddress(memoAddress)
	if err != nil {
		return errors.Wrap(err, "could not convert memo master address to derived ATA")
	}

	valid, err := bridge.solanaWallet.IsValidReceiver(ctx, receiver)
	if err != nil {
		return errors.Wrap(err, "Failed to check if receiver is proper")
	}

	if !valid {
		return faults.ErrInvalidReceiver
	}

	depositFeeBigInt := big.NewInt(stellar.IntToStroops(bridge.config.DepositFee))

	if depositedAmount.Cmp(depositFeeBigInt) <= 0 {
		log.Error().Str("amount", depositedAmount.String()).Str("txID", txID).Msg("Deposited amount is <= Fee, should be returned")
		return faults.ErrInsufficientDepositAmount
	}
	amount := &big.Int{}
	amount = amount.Sub(depositedAmount, depositFeeBigInt)

	requiredSignatureCount, err := bridge.solanaWallet.GetRequiresSignatureCount(ctx)
	if err != nil {
		return err
	}
	log.Debug().Int64("count", requiredSignatureCount).Msg("required signature count")

	// We don't need to resolve our own peer address
	onlineSigners, err := bridge.signersClient.SolID(ctx, int(requiredSignatureCount-1))
	if err != nil {
		return errors.Wrap(err, "could not resolve online solana signers")
	}

	os := make([]solana.Address, 0, len(onlineSigners)+1)
	for _, v := range onlineSigners {
		os = append(os, v)
	}

	tx, err := bridge.solanaWallet.PrepareMintTx(ctx, solana.MintInfo{
		// We are always online and ready to sign
		OnlineSigners: append(os, bridge.solanaWallet.Address()),
		Amount:        uint64(amount.Int64()),
		TxID:          txID,
		To:            receiver,
	})
	if err != nil {
		return errors.Wrap(err, "could not prepare solana transaction")
	}
	txB64, err := tx.ToBase64()
	if err != nil {
		return errors.Wrap(err, "could not encode solana transaction to base64")
	}

	onlinePeers := make([]peer.ID, 0, len(onlineSigners))
	for p := range onlineSigners {
		onlinePeers = append(onlinePeers, p)
	}

	res, err := bridge.signersClient.SignMint(ctx, onlinePeers, SolanaRequest{
		Receiver: receiver,
		Amount:   amount.Int64(),
		TxID:     txID,
		// subtract 1 from the required signature count, because the master signature is already included
		RequiredSignatures: requiredSignatureCount - 1,
		Tx:                 txB64,
	})
	if err != nil {
		return err
	}

	// First create the master signature
	signature, idx, err := bridge.solanaWallet.CreateTokenSignature(*tx)
	if err != nil {
		return err
	}

	// Append to the signatures array
	res = append(res, SolanaResponse{Who: bridge.solanaWallet.Address(), Signature: signature, SigIdx: idx})

	signers, err := bridge.solanaWallet.GetSigners(ctx)
	if err != nil {
		return err
	}

	orderderedSignatures := make([]solana.Signature, len(res))
	for i := 0; i < len(signers); i++ {
		for _, sign := range res {
			if sign.SigIdx == i {
				orderderedSignatures[i] = sign.Signature
			}
		}
	}

	log.Debug().Int("count", len(orderderedSignatures)).Msg("Total signatures count")

	tx.Signatures = orderderedSignatures

	if err = tx.VerifySignatures(); err != nil {
		log.Error().Err(err).Msg("Signature verification error")
		return err
	}

	return bridge.solanaWallet.Mint(ctx, tx)
}

// Start the main processing loop of the bridge
func (bridge *Bridge) Start(ctx context.Context) error {
	solanaBurns, err := bridge.solanaWallet.SubscribeTokenBurns(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to solana burns")
	}

	// Only the bridge running as the master bridge should do the following things:
	// - Monitor the Bridge Stellar account and initiate Minting transactions accordingly
	// - Monitor the Contract for Withdrawal events and initiate a Withdrawal transaction accordingly
	if !bridge.config.Follower {
		// Scan bridge account for outgoing transactions to avoid double withdraws or refunds
		if err := bridge.wallet.ScanBridgeAccount(ctx); err != nil {
			panic(err)
		}

		// Monitor the bridge wallet for incoming transactions
		// mint transactions on solana if possible
		go func() {
			if err := bridge.wallet.MonitorBridgeAccountAndMint(ctx, bridge.mint, bridge.blockPersistency); err != nil {
				panic(err)
			}
		}()

	}

	go func() {
		// txMap := make(map[string]solana.Burn)
		bridge.synced = true
		for {
			select {
			// Remember new withdraws
			// Never happens for cosigners, only for the master since the cosugners are not subscribed to withdraw events
			case burn, ok := <-solanaBurns:
				// Check for closed channel
				if !ok {
					log.Warn().Msg("Solana burn channel is closed")
					return
				}

				// log.Info().Str("txHash", burn.TxID().String()).Str("shortTxHash", burn.ShortTxID().String()).Msg("Remembering withdraw event")
				log.Info().Str("txHash", burn.TxID().String()).Str("shortTxHash", burn.ShortTxID().String()).Msg("Starting withdrawal")
				// txMap[burn.ShortTxID().String()] = burn
				// log.Info().Str("txHash", we.TxID().String()).Msg("Starting withdrawal")
				err := bridge.withdraw(ctx, burn)
				if err != nil {
					log.Error().Err(err).Str("address", burn.Memo()).Msg("failed to create payment for withdrawal")
					continue
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (bridge *Bridge) withdraw(ctx context.Context, burn solana.Burn) (err error) {
	// if a withdraw was made to the bridge fee wallet or the bridge address, soak the funds and return
	// TODO: Should these adresses be fetched through the wallet?
	if burn.Memo() == bridge.wallet.Config.StellarFeeWallet || burn.Memo() == bridge.wallet.GetAddress() {
		log.Warn().Msg("Received a withdrawal with destination which is either the fee wallet or the bridge wallet, skipping...")
		return nil
	}

	hash := burn.TxID()
	shortTxID := burn.ShortTxID()
	amount := burn.RawAmount()

	if amount == 0 {
		log.Warn().Str("solanaTx", hash.String()).Str("shortSolanaTxID", shortTxID.String()).Msg("Can not withdraw an amount of 0")
		return
	}

	if amount <= uint64(WithdrawFee) {
		log.Warn().Str("amount", stellar.StroopsToDecimal(int64(amount)).String()).Str("solanaTx", hash.String()).Str("shortSolanaTxID", shortTxID.String()).Msg("Withdrawn amount is less than the withdraw fee, skip it")
		return
	}

	log.Info().Str("solanaTx", hash.String()).Str("shortSolanaTxID", shortTxID.String()).Str("destination", burn.Memo()).Str("amount", stellar.StroopsToDecimal(int64(amount)).String()).Msg("Creating a withdraw tx")

	amount -= uint64(WithdrawFee)
	// TODO: Should this adress be fetched through the wallet?
	includeWithdrawFee := bridge.wallet.Config.StellarFeeWallet != ""
	err = bridge.wallet.CreateAndSubmitPayment(ctx, burn.Memo(), amount, burn.Caller(), shortTxID, "", includeWithdrawFee)
	return
}
