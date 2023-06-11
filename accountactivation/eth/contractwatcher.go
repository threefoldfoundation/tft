package eth

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/threefoldfoundation/tft/accountactivation/eth/contract"
	"github.com/threefoldfoundation/tft/accountactivation/state"
)

type ContractWatcher struct {
	EthClient        *ethclient.Client
	ContractFilterer *contract.AccountActivationFilterer
	blockPersistency *state.ChainPersistency
	Sink             chan AccounActivationRequest
}

type AccounActivationRequest struct {
	Network             string
	Account             string
	EthereumTransaction common.Hash
	BlockNumber         uint64
}

// NewContractWatcher creates a new ContractWatcher.
func NewContractWatcher(ethNodeUrl string, contractAddress string, blockPersistency *state.ChainPersistency, sink chan AccounActivationRequest) (cw *ContractWatcher, err error) {

	cl, err := ethclient.Dial(ethNodeUrl)
	if err != nil {
		return
	}

	contractFilterer, err := contract.NewAccountActivationFilterer(common.HexToAddress(contractAddress), cl)
	cw = &ContractWatcher{
		EthClient:        cl,
		ContractFilterer: contractFilterer,
		blockPersistency: blockPersistency,
		Sink:             sink,
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
	waitABit := func(ctx context.Context) {
		if ctx.Err() != nil {
			return
		}
		time.Sleep(time.Second * 5)
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		currentBlock, err := cw.EthClient.BlockNumber(ctx)

		if err != nil {
			log.Warn("Unable to get current block", "err", err)
			waitABit(ctx)
			continue
		}
		if watchFromHeight >= currentBlock {
			waitABit(ctx)
			continue
		}

		it, err := cw.ContractFilterer.FilterActivateAccount(&bind.FilterOpts{
			Context: ctx,
			Start:   watchFromHeight,
			End:     &currentBlock,
		})
		if err != nil {
			log.Warn("Unable to filter logs for ActivateAccount events", "err", err)
			waitABit(ctx)
			continue
		}
		for it.Next() {
			log.Info("Account Activation request", "account", it.Event.Account, "tx", it.Event.Raw.TxHash.Hex())
			cw.Sink <- AccounActivationRequest{
				//sink <- AccounActivationRequest{
				Network:             it.Event.Network,
				Account:             it.Event.Account,
				EthereumTransaction: it.Event.Raw.TxHash,
				BlockNumber:         it.Event.Raw.BlockNumber,
			}
		}
		watchFromHeight = currentBlock
		waitABit(ctx)
	}

}
