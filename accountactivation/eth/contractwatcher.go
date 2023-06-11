package eth

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/threefoldfoundation/tft/accountactivation/eth/contract"
	"github.com/threefoldfoundation/tft/accountactivation/state"
)

type ContractWatcher struct {
	EthClient        *ethclient.Client
	ContractFilterer *contract.AccountActivationFilterer
	blockPersistency *state.ChainPersistency
}

// NewContractWatcher creates a new ContractWatcher.
func NewContractWatcher(ethNodeUrl string, contractAddress string, stateFile string) (cw *ContractWatcher, err error) {
	blockPersistency := state.NewChainPersistency(stateFile)

	cl, err := ethclient.Dial(ethNodeUrl)
	if err != nil {
		return
	}

	contractFilterer, err := contract.NewAccountActivationFilterer(common.HexToAddress(contractAddress), cl)
	cw = &ContractWatcher{
		EthClient:        cl,
		ContractFilterer: contractFilterer,
		blockPersistency: blockPersistency,
	}
	return
}

func (cw *ContractWatcher) Close() {
	cw.EthClient.Close()
}

func (cw *ContractWatcher) Start(ctx context.Context, watchFromHeight uint64) (err error) {

	// If the user provides a height to rescan from, use that
	// Otherwise use the saved height in the persistency file
	if watchFromHeight == 0 {
		watchFromHeight, err = cw.blockPersistency.GetHeight()
		if err != nil {
			return
		}
	}
	// If there is no explicit starting block, just use the current block
	if watchFromHeight == 0 {
		watchFromHeight, err = cw.EthClient.BlockNumber(ctx)
		if err != nil {
			return
		}
	}
	log.Info("Watching for ActivateAccount events", "start", watchFromHeight)
	watchOpts := bind.WatchOpts{Context: ctx, Start: &watchFromHeight}
	eventSink := make(chan *contract.AccountActivationActivateAccount)
	subscription := event.Resubscribe(time.Second*20, func(ctx context.Context) (event.Subscription, error) {
		s, err := cw.ContractFilterer.WatchActivateAccount(&watchOpts, eventSink)
		if err != nil {
			log.Debug("event subscription failed", "err", err)
		}
		return s, err
	})
	defer subscription.Unsubscribe()

	for {
		select {
		case err = <-subscription.Err():
			return
		case activationEvent := <-eventSink:
			if activationEvent.Raw.Removed {
				// ignore removed events
				continue
			}
			log.Info("Account Activation request", "account", activationEvent.Account, "tx", activationEvent.Raw.TxHash)

		}
	}

}
