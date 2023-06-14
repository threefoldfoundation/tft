package stellar

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/strkey"

	"github.com/ethereum/go-ethereum/log"
)

func AccountAdressFromSecret(secret string) (address string, err error) {
	kp, err := keypair.ParseFull(secret)
	if err != nil {
		return
	}
	address = kp.Address()
	return
}

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

func fetchTransactions(ctx context.Context, client *horizonclient.Client, address string, cursor string, handler func(op hProtocol.Transaction) error) error {
	timeouts := 0
	pageLimit := uint(100)
	opRequest := horizonclient.TransactionRequest{
		ForAccount:    address,
		IncludeFailed: false,
		Cursor:        cursor,
		Limit:         pageLimit,
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
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
				return ctx.Err()
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
			opRequest.Limit = pageLimit
			timeouts = 0
		}

		if len(response.Embedded.Records) == 0 {
			return nil
		}

	}

}

func IsValidStellarAddress(address string) bool {
	return strkey.IsValidEd25519PublicKey(address)
}

func IsValidStellarSecret(secret string) bool {
	return strkey.IsValidEd25519SecretSeed(secret)
}
