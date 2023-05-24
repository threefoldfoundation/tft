package stellar

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"

	"github.com/ethereum/go-ethereum/log"
)

const (
	TFTMainnet = "TFT:GBOVQKJYHXRR3DX6NOX2RRYFRCUMSADGDESTDNBDS6CDVLGVESRTAC47"
	TFTTest    = "TFT:GA47YZA3PKFUZMPLQ3B5F2E3CJIB57TGGU7SPCQT2WAEYKN766PWIMB3"

	Precision       = int64(1e7)
	PrecisionDigits = 7
	PageLimit       = 100 // TODO: should this be public?
)

// GetHorizonClient gets an horizon client for a specific network
func GetHorizonClient(network string) (*horizonclient.Client, error) {
	switch network {
	case "testnet":
		return horizonclient.DefaultTestNetClient, nil
	case "production":
		return horizonclient.DefaultPublicNetClient, nil
	default:
		return nil, errors.New("network is not supported")
	}
}

// GetNetworkPassPhrase gets the Stellar network passphrase based on a network input
func GetNetworkPassPhrase(ntwrk string) string {
	switch ntwrk {
	case "testnet":
		return network.TestNetworkPassphrase
	case "production":
		return network.PublicNetworkPassphrase
	default:
		return network.TestNetworkPassphrase
	}
}

// IntToStroops converts units to stroops (1 TFT = 1000000 stroops)
func IntToStroops(x int64) int64 {
	return x * Precision
}

// IntToStroops converts units to stroops (1 TFT = 1000000 stroops)
func DecimalToStroops(x decimal.Decimal) int64 {
	stroops := x.Mul(decimal.NewFromInt(Precision))
	return stroops.IntPart()
}

// IntToStroops converts stroops to units to (1 TFT = 1000000 stroops)
func StroopsToDecimal(stroops int64) decimal.Decimal {
	decimalStroops := decimal.NewFromInt(stroops)
	return decimalStroops.Div(decimal.NewFromInt(Precision))
}

func fetchTransactions(ctx context.Context, client *horizonclient.Client, address string, cursor string, handler func(op hProtocol.Transaction)) error {
	timeouts := 0
	opRequest := horizonclient.TransactionRequest{
		ForAccount:    address,
		IncludeFailed: false,
		Cursor:        cursor,
		Limit:         PageLimit,
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		response, err := client.Transactions(opRequest)
		if err != nil {
			log.Info("Error getting transactions for stellar account", "address", opRequest.ForAccount, "cursor", opRequest.Cursor, "pagelimit", opRequest.Limit, "error", err)
			horizonError, ok := err.(*horizonclient.Error)
			if ok && (horizonError.Response.StatusCode == http.StatusGatewayTimeout || horizonError.Response.StatusCode == http.StatusServiceUnavailable) {
				timeouts++
				if timeouts == 1 {
					opRequest.Limit = 5
				} else if timeouts > 1 {
					opRequest.Limit = 1
				}

				log.Info("Request timed out, lowering pagelimit", "pagelimit", opRequest.Limit)
			}

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
				continue
			}

		}
		for _, tx := range response.Embedded.Records {
			handler(tx)
			opRequest.Cursor = tx.PagingToken()
		}

		if timeouts > 0 {
			log.Info("Fetching transaction succeeded, resetting page limit and timeouts")
			opRequest.Limit = PageLimit
			timeouts = 0
		}

		if len(response.Embedded.Records) == 0 {
			return nil
		}

	}

}

func ExtractMemoFromTx(txn *txnbuild.Transaction) (memoAsHex string, err error) {
	memo := txn.Memo()

	if memo == nil {
		return
	}

	txMemo, err := txn.Memo().ToXDR()
	if err != nil {
		return
	}

	switch txMemo.Type {
	case xdr.MemoTypeMemoHash:
		hashMemo := txn.Memo().(txnbuild.MemoHash)
		memoAsHex = hex.EncodeToString(hashMemo[:])
	case xdr.MemoTypeMemoReturn:
		hashMemo := txn.Memo().(txnbuild.MemoReturn)
		memoAsHex = hex.EncodeToString(hashMemo[:])
	default:
		err = fmt.Errorf("transaction memo type not supported")
	}

	return
}

func IsValidStellarAddress(address string) bool {
	return strkey.IsValidEd25519PublicKey(address)
}
