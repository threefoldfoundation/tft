package bridge

import (
	"context"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"

	"github.com/stellar/go/protocols/horizon"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/threefoldfoundation/tft/polygon/bridges/stellar/bridge/stellar"
)

const (
	TFTMainnet = "TFT:GBOVQKJYHXRR3DX6NOX2RRYFRCUMSADGDESTDNBDS6CDVLGVESRTAC47"
	TFTTest    = "TFT:GA47YZA3PKFUZMPLQ3B5F2E3CJIB57TGGU7SPCQT2WAEYKN766PWIMB3"

	StellarPrecision       = 1e7
	StellarPrecisionDigits = 7
)

func GetTFTAsset(network string) (asset horizon.Asset) {
	var canonicalTFTAsset string
	switch network {
	case "production":
		canonicalTFTAsset = TFTMainnet
	default:
		canonicalTFTAsset = TFTTest
	}

	splitTFTAsset := strings.Split(canonicalTFTAsset, ":")
	asset.Code = splitTFTAsset[0]
	asset.Issuer = splitTFTAsset[1]
	return
}

func MonitorBridgeStellarTransactions(ctx context.Context, network, vaultAddress, cursor string, handler func(tx hProtocol.Transaction) error) (err error) {
	client, err := stellar.GetHorizonClient(network)
	if err != nil {
		return
	}

	log.Info("Start watching stellar account transactions", "account", vaultAddress, "cursor", cursor)

	for {
		if ctx.Err() != nil {
			return
		}

		internalHandler := func(tx hProtocol.Transaction) {
			err := handler(tx)
			for err != nil {
				err = handler(tx)
			}
			cursor = tx.PagingToken()
		}
		err = stellar.FetchTransactions(ctx, client, vaultAddress, cursor, internalHandler)
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}

	}

}
