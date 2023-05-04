package bridge

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/threefoldfoundation/tft/bridge/stellar/contracts/tokenv1"
)

var errInsufficientDepositAmount = errors.New("deposited amount is <= Fee")

const (
	// EthBlockDelay is the amount of blocks to wait before
	// pushing eth transaction to the stellar network
	EthBlockDelay = 3
	// Withdrawing from smartchain to Stellar fee
	WithdrawFee      = int64(1 * stellarPrecision)
	BridgeNetwork    = "stellar"
	EthMessagePrefix = "\x19Ethereum Signed Message:\n32"
)

// Bridge is a high lvl structure which listens on contract events and bridge-related
// stellar transactions, and handles them
type Bridge struct {
	bridgeContract   *BridgeContract
	wallet           *stellarWallet
	blockPersistency *ChainPersistency
	mut              sync.Mutex
	config           *BridgeConfig
	synced           bool
}

type BridgeConfig struct {
	EthNetworkName          string
	EthUrl                  string
	EthPrivateKey           string
	ContractAddress         string
	MultisigContractAddress string
	RescanBridgeAccount     bool
	RescanFromHeight        int64
	PersistencyFile         string
	Follower                bool
	BridgeMasterAddress     string
	Relay                   string
	Psk                     string
	StellarConfig
}

type StellarConfig struct {
	// network for the stellar config
	StellarNetwork string
	// seed for the stellar bridge wallet
	StellarSeed string
	// stellar fee wallet address
	StellarFeeWallet string
	// deposit fee in TFT units
	DepositFee int64
}

// DepositFeeInStroops returns the DepositFee in the Stellar base unit
func (c *StellarConfig) DepositFeeInStroops() int64 {
	return c.DepositFee * stellarPrecision
}

// NewBridge creates a new Bridge.
func NewBridge(ctx context.Context, config *BridgeConfig, host host.Host, router routing.PeerRouting) (bridge *Bridge, err error) {

	contract, err := NewBridgeContract(config)
	if err != nil {
		return
	}

	blockPersistency := newChainPersistency(config.PersistencyFile)

	wallet, err := newStellarWallet(ctx, &config.StellarConfig)
	if err != nil {
		return
	}

	// Only create the stellar signer wallet if the bridge is a master bridge
	if !config.Follower {
		relayAddrInfo, addrErr := peer.AddrInfoFromString(config.Relay)
		if err != nil {
			return nil, addrErr
		}

		err = wallet.newSignerClient(ctx, host, router, relayAddrInfo)
		if err != nil {
			return
		}
		log.Info(fmt.Sprintf("Stellar bridge account %s loaded on Stellar network %s", wallet.keypair.Address(), config.StellarNetwork))
	}

	if config.RescanBridgeAccount {
		// setting the cursor to 0 will trigger the bridge
		// to scan for every transaction ever made on the bridge account
		// and mint accordingly
		err = blockPersistency.saveStellarCursor("0")
		if err != nil {
			return
		}
	}

	bridge = &Bridge{
		bridgeContract:   contract,
		blockPersistency: blockPersistency,
		wallet:           wallet,
		config:           config,
	}

	return
}

// Close bridge
func (bridge *Bridge) Close() error {
	bridge.mut.Lock()
	bridge.bridgeContract.ethc.Close()
	defer bridge.mut.Unlock()
	return nil
}

func (bridge *Bridge) mint(receiver ERC20Address, depositedAmount *big.Int, txID string) (err error) {
	if !bridge.synced {
		return errors.New("bridge is not synced, retry later")
	}
	log.Info("Minting", "receiver", hex.EncodeToString(receiver[:]), "txID", txID)
	// check if we already know this ID
	known, err := bridge.bridgeContract.IsMintTxID(txID)
	if err != nil {
		return
	}
	if known {
		log.Info("Skipping known minting transaction", "txID", txID)
		// we already know this withdrawal address, so ignore the transaction
		return
	}

	depositFeeBigInt := big.NewInt(bridge.config.DepositFeeInStroops())

	if depositedAmount.Cmp(depositFeeBigInt) <= 0 {
		log.Error("Deposited amount is <= Fee, should be returned", "amount", depositedAmount, "txID", txID)
		return errInsufficientDepositAmount
	}
	amount := &big.Int{}
	amount = amount.Sub(depositedAmount, depositFeeBigInt)

	requiredSignatureCount, err := bridge.bridgeContract.GetRequiresSignatureCount()
	if err != nil {
		return err
	}

	// Create signatures array with the required length
	signs := make([]tokenv1.Signature, requiredSignatureCount.Int64())

	res, err := bridge.wallet.client.SignMint(context.Background(), EthSignRequest{
		Receiver: common.BytesToAddress(receiver[:]),
		Amount:   amount.Int64(),
		TxId:     txID,
		// subtract 1 from the required signature count, because the master signature is already included
		RequiredSignatures: requiredSignatureCount.Sub(requiredSignatureCount, big.NewInt(1)).Int64(),
	})
	if err != nil {
		return err
	}

	// First append the master signature
	signature, err := bridge.bridgeContract.CreateTokenSignature(common.Address(receiver), amount.Int64(), txID)
	if err != nil {
		return err
	}
	signs[0] = signature

	// Append signatures in order
	// TODO: check what order
	for i := 0; i < len(res); i++ {
		// todo: verify signatures
		signs[i+1] = res[i].Signature
	}

	signers, err := bridge.bridgeContract.GetSigners()
	if err != nil {
		return err
	}

	log.Debug("signers list", "l", signers)

	log.Debug("total signatures count", "count", len(signs))

	return bridge.bridgeContract.Mint(receiver, amount, txID, signs)
}

// GetClient returns bridgecontract lightclient
func (bridge *Bridge) GetClient() *EthClient {
	return bridge.bridgeContract.EthClient()
}

// GetBridgeContract returns this bridge's contract.
func (bridge *Bridge) GetBridgeContract() *BridgeContract {
	return bridge.bridgeContract
}

// Start the main processing loop of the bridge
func (bridge *Bridge) Start(ctx context.Context) error {
	heads := make(chan *ethtypes.Header)

	go bridge.bridgeContract.Loop(heads)

	// subscribing to these events is not needed for operational purposes, but might be nice to get some info
	go func() {
		err := bridge.bridgeContract.SubscribeTransfers()
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		err := bridge.bridgeContract.SubscribeMint()
		if err != nil {
			panic(err)
		}
	}()

	// Channel where withdrawal events are stored
	// Should only be read from by the master bridge
	withdrawChan := make(chan WithdrawEvent)

	// Only the bridge running as the master bridge should do the following things:
	// - Monitor the Bridge Stellar account and initiate Minting transactions accordingly
	// - Monitor the Contract for Withdrawal events and initiate a Withdrawal transaction accordingly
	if !bridge.config.Follower {
		// Scan bridge account for outgoing transactions to avoid double withdraws or refunds
		if err := bridge.wallet.ScanBridgeAccount(); err != nil {
			panic(err)
		}

		// Monitor the bridge wallet for incoming transactions
		// mint transactions on ERC20 if possible
		go func() {
			if err := bridge.wallet.MonitorBridgeAccountAndMint(ctx, bridge.mint, bridge.blockPersistency); err != nil {
				panic(err)
			}
		}()

		// Sync up any withdrawals made if the blockheight is manually set
		// to a previous value
		currentBlock, err := bridge.bridgeContract.ethc.BlockNumber(ctx)
		if err != nil {
			return err
		}

		var lastHeight uint64
		// If the user provides a height to rescan from, use that
		// Otherwise use the saved height in the persistency file
		if bridge.config.RescanFromHeight > 0 {
			lastHeight = uint64(bridge.config.RescanFromHeight) - EthBlockDelay
		} else {
			height, err := bridge.blockPersistency.GetHeight()
			if err != nil {
				return err
			}
			// if the saved height is 0, just use current block
			if height.LastHeight == 0 {
				lastHeight = currentBlock
			}
			if height.LastHeight > EthBlockDelay {
				lastHeight = height.LastHeight - EthBlockDelay
			}
		}

		if lastHeight < currentBlock {
			// todo filter logs
			go func() {
				if err := bridge.bridgeContract.FilterWithdraw(withdrawChan, lastHeight, currentBlock); err != nil {
					panic(err)
				}
			}()
		}

		go func() {
			err := bridge.bridgeContract.SubscribeWithdraw(withdrawChan, currentBlock)
			if err != nil {
				panic(err)
			}
		}()

	}

	go func() {
		txMap := make(map[string]WithdrawEvent)
		for {
			select {
			// Remember new withdraws
			// Never happens for cosigners, only for the master since the cosugners are not subscribed to withdraw events
			case we := <-withdrawChan:
				if we.network == BridgeNetwork {
					log.Info("Remembering withdraw event", "txHash", we.TxHash(), "height", we.BlockHeight(), "network", we.network)
					txMap[we.txHash.String()] = we
				} else {
					log.Warn("Ignoring withdrawal, invalid target network", "hash", we.TxHash(), "height", we.BlockHeight(), "network", we.network)
				}
			case head := <-heads:
				bridge.mut.Lock()

				progress, err := bridge.bridgeContract.ethc.SyncProgress(ctx)
				if err != nil {
					log.Error(fmt.Sprintf("failed to get sync progress %s", err.Error()))
				}
				if progress == nil {
					bridge.synced = true
				}

				log.Info("found new head", "head", head.Number, "synced", bridge.synced)

				if bridge.synced {
					ids := make([]string, 0, len(txMap))
					for id := range txMap {
						ids = append(ids, id)
					}

					for _, id := range ids {
						we := txMap[id]
						if head.Number.Uint64() >= we.blockHeight+EthBlockDelay {
							log.Info("Starting withdrawal", "txHash", we.TxHash())
							err := bridge.withdraw(ctx, we)
							if err != nil {
								log.Error(fmt.Sprintf("failed to create payment for withdrawal to %s, %s", we.blockchain_address, err.Error()))
								continue
							}

							// forget about our tx
							delete(txMap, id)
						}
					}

				}

				err = bridge.blockPersistency.saveHeight(head.Number.Uint64())
				if err != nil {
					log.Error("error occured saving blockheight", "error", err)
				}
				bridge.mut.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (bridge *Bridge) withdraw(ctx context.Context, we WithdrawEvent) (err error) {
	// if a withdraw was made to the bridge fee wallet or the bridge address, soak the funds and return
	if we.blockchain_address == bridge.config.StellarFeeWallet || we.blockchain_address == bridge.wallet.keypair.Address() {
		log.Warn("Received a withdrawal with destination which is either the fee wallet or the bridge wallet, skipping...")
		return nil
	}

	hash := we.TxHash()
	log.Info("Creating a withdraw tx", "ethTx", hash)
	amount := we.amount.Uint64()

	if amount == 0 {
		log.Error("Can not withdraw an amount of 0", "ethTx", hash)
		return
	}

	if amount <= uint64(WithdrawFee) {
		log.Warn("Withdrawn amount is less than the withdraw fee, sending the amount to the fee wallet", "amount", amount)
		err = bridge.wallet.CreateAndSubmitFeepayment(ctx, amount, hash)
		if err != nil {
			log.Error(fmt.Sprintf("failed to create fee payment for withdrawal to %s, %s", we.blockchain_address, err.Error()))
			return err
		}
		return nil
	}

	amount -= uint64(WithdrawFee)
	includeWithdrawFee := bridge.wallet.config.StellarFeeWallet != ""
	err = bridge.wallet.CreateAndSubmitPayment(ctx, we.blockchain_address, amount, we.receiver, we.blockHeight, hash, "", includeWithdrawFee)
	if err != nil {
		log.Error(fmt.Sprintf("failed to create payment for withdrawal to %s, %s", we.blockchain_address, err.Error()))
	}
	return
}
