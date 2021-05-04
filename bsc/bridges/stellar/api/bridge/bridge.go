package bridge

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
)

var errInsufficientDepositAmount = errors.New("deposited amount is <= Fee")

const (
	// EthBlockDelay is the amount of blocks to wait before
	// pushing eth transaction to the tfchain network
	EthBlockDelay = 3
	DepositFee    = 500000000
)

// Bridge is a high lvl structure which listens on contract events and bridge-related
// tfchain transactions, and handles them
type Bridge struct {
	bridgeContract   *BridgeContract
	wallet           *stellarWallet
	blockPersistency *ChainPersistency
	mut              sync.Mutex
	config           *BridgeConfig
	depositFee       *big.Int
}

type BridgeConfig struct {
	EthNetworkName          string
	Bootnodes               []string
	ContractAddress         string
	MultisigContractAddress string
	EthPort                 uint16
	AccountJSON             string
	AccountPass             string
	Datadir                 string
	RescanBridgeAccount     bool
	PersistencyFile         string
	Follower                bool
	StellarConfig
}

type StellarConfig struct {
	// network for the stellar config
	StellarNetwork string
	// seed for the stellar bridge wallet
	StellarSeed string
	// stellar fee wallet address
	StellarFeeWallet string
}

// NewBridge creates a new Bridge.
func NewBridge(ctx context.Context, config *BridgeConfig, host host.Host, router routing.PeerRouting) (*Bridge, error) {

	contract, err := NewBridgeContract(config)
	if err != nil {
		return nil, err
	}

	blockPersistency, err := initPersist(config.PersistencyFile)
	if err != nil {
		return nil, err
	}

	var wallet *stellarWallet
	// Only create the stellar wallet if the bridge is a master bridge
	if !config.Follower {
		wallet, err = newStellarWallet(ctx, &config.StellarConfig, host, router)
		if err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Stellar bridge account %s loaded on Stellar network %s", wallet.keypair.Address(), config.StellarNetwork))
	}

	if config.RescanBridgeAccount {
		// saving the cursor to 1 will trigger the bridge stellar account
		// to scan for every transaction ever made on the bridge account
		// and mint accordingly
		err = blockPersistency.saveStellarCursor("0")
		if err != nil {
			return nil, err
		}
	}
	var depositFee big.Int
	depositFee.SetInt64(DepositFee) // 50 TFT with a precision of 7decimals
	bridge := &Bridge{
		bridgeContract:   contract,
		blockPersistency: blockPersistency,
		wallet:           wallet,
		config:           config,
		depositFee:       &depositFee,
	}

	return bridge, nil
}

// Close bridge
func (bridge *Bridge) Close() error {
	bridge.mut.Lock()
	defer bridge.mut.Unlock()
	err := bridge.bridgeContract.Close()
	return err
}

func (bridge *Bridge) mint(receiver ERC20Address, depositedAmount *big.Int, txID string) (err error) {
	log.Info("Minting", "receiver", hex.EncodeToString(receiver[:]), "txID", txID)
	// check if we already know this ID
	if depositedAmount.Cmp(bridge.depositFee) <= 0 {
		return errInsufficientDepositAmount
	}
	known, err := bridge.bridgeContract.IsMintTxID(txID)
	if err != nil {
		return
	}
	if known {
		log.Info("Skipping known minting transaction", "txID", txID)
		// we already know this withdrawal address, so ignore the transaction
		return
	}
	amount := &big.Int{}
	amount.Sub(depositedAmount, bridge.depositFee)
	return bridge.bridgeContract.Mint(receiver, amount, txID)
}

// GetClient returns bridgecontract lightclient
func (bridge *Bridge) GetClient() *LightClient {
	return bridge.bridgeContract.LightClient()
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
	go bridge.bridgeContract.SubscribeTransfers()
	go bridge.bridgeContract.SubscribeMint()

	// Channel where withdrawal events are stored
	// Should only be read from by the master bridge
	withdrawChan := make(chan WithdrawEvent)

	// Channel where sumbission events from the multisig contract are stored
	// Should only be read from by the follower bridges, this event channel
	// will indicate when they need to confirm the withdrawal transaction that is submitted
	submissionChan := make(chan SubmissionEvent)

	// Only the bridge running as the master bridge should do the following things:
	// - Monitor the Bridge Stellar account and initiate Minting transactions accordingly
	// - Monitor the Contract for Withdrawal events and initiate a Withdrawal transaction accordingly
	if !bridge.config.Follower {
		// Monitor the bridge wallet for incoming transactions
		// mint transactions on ERC20 if possible
		go bridge.wallet.MonitorBridgeAndMint(bridge.mint, bridge.blockPersistency)

		height, err := bridge.blockPersistency.GetHeight()
		if err != nil {
			return err
		}
		var lastHeight uint64
		if height.LastHeight > EthBlockDelay {
			lastHeight = height.LastHeight - EthBlockDelay
		}
		go bridge.bridgeContract.SubscribeWithdraw(withdrawChan, lastHeight)
	} else {
		go bridge.bridgeContract.SubscribeSubmission(submissionChan)
	}

	go func() {
		txMap := make(map[string]WithdrawEvent)
		for {
			select {
			// Remember new withdraws
			case we := <-withdrawChan:
				log.Info("Remembering withdraw event", "txHash", we.TxHash(), "height", we.BlockHeight())
				txMap[we.txHash.String()] = we
			// If we get a new head, check every withdraw we have to see if it has matured
			case submission := <-submissionChan:
				log.Info("Submission Event seen", "txid", submission.TransactionId())
				err := bridge.bridgeContract.ConfirmTransaction(submission.TransactionId())
				if err != nil {
					log.Error("error occured during confirming transaction")
				}
			case head := <-heads:
				bridge.mut.Lock()
				ids := make([]string, 0, len(txMap))
				for id := range txMap {
					ids = append(ids, id)
				}

				for _, id := range ids {
					we := txMap[id]
					if head.Number.Uint64() >= we.blockHeight+EthBlockDelay {
						hash := we.TxHash()
						log.Info("Create a withdraw tx", "ethTx", hash)

						err := bridge.wallet.CreateAndSubmitPayment(ctx, we.blockchain_address, we.network, we.amount.Uint64(), we.receiver, we.blockHeight, hash, "")
						if err != nil {
							log.Error(fmt.Sprintf("failed to create payment for withdrawal to %s, %s", we.blockchain_address, err.Error()))
						}
						// forget about our tx
						delete(txMap, id)
					}
				}

				err := bridge.blockPersistency.saveHeight(head.Number.Uint64())
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
