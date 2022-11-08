package bridge

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stellar/go/amount"
	hProtocol "github.com/stellar/go/protocols/horizon"

	horizoneffects "github.com/stellar/go/protocols/horizon/effects"

	"github.com/threefoldfoundation/tft/polygon/bridges/stellar/bridge/stellar"
)

// Bridge implements the actual briding
type Bridge struct {
	Persistency *ChainPersistency
	config      *BridgeConfig
	synced      bool
}

// NewBridge creates a new Bridge.
func NewBridge(config *BridgeConfig) (br *Bridge, err error) {

	blockPersistency := newChainPersistency(config.PersistencyFile)

	br = &Bridge{
		Persistency: blockPersistency,
		config:      config,
	}

	return
}

func (br *Bridge) Start(ctx context.Context) (err error) {

	// get saved cursor
	blockHeight, err := br.Persistency.GetHeight()
	for err != nil {
		log.Warn("Error getting the bridge persistency", "error", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
			blockHeight, err = br.Persistency.GetHeight()
		}
	}
	//Temporary transaction handler
	transactionHandler := func(tx hProtocol.Transaction) (err error) {
		if !tx.Successful {
			return
		}
		log.Info("Received transaction on bridge stellar account", "hash", tx.Hash)

		effects, err := stellar.GetTransactionEffects(br.config.StellarNetwork, tx.Hash)
		if err != nil {
			log.Error("error while fetching transaction effects:", err.Error())
			return
		}
		tftAsset := GetTFTAsset(br.config.StellarNetwork)
		var receivedTFTAmount int64
		for _, effect := range effects.Embedded.Records {
			if effect.GetAccount() != br.config.VaultAddress {
				continue
			}
			if effect.GetType() == "account_credited" {
				creditedEffect := effect.(horizoneffects.AccountCredited)
				if creditedEffect.Asset.Code != tftAsset.Code && creditedEffect.Asset.Issuer != tftAsset.Issuer {
					continue
				}
				parsedAmount, err := amount.ParseInt64(creditedEffect.Amount)
				if err != nil {
					continue
				}

				receivedTFTAmount += parsedAmount
			}
		}
		if receivedTFTAmount != 0 {
			log.Info("Received TFT", "amount", amount.StringFromInt64(receivedTFTAmount))
		}

		return
	}
	go func() {
		if err := MonitorBridgeStellarTransactions(ctx, br.config.StellarNetwork, br.config.VaultAddress, blockHeight.StellarCursor, transactionHandler); err != nil {
			panic(err)
		}
	}()

	return
}
