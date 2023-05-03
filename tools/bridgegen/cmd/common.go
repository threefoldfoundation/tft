package cmd

import (
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/protocols/horizon/base"
	"github.com/stellar/go/txnbuild"
)

const (
	TFT            = "TFT"
	TESTNET_ISSUER = "GA47YZA3PKFUZMPLQ3B5F2E3CJIB57TGGU7SPCQT2WAEYKN766PWIMB3"
)

var HorizonClient = horizonclient.DefaultTestNetClient

var TestnetTft = txnbuild.CreditAsset{Code: TFT, Issuer: TESTNET_ISSUER}
var TestnetTftAsset = base.Asset{Type: "credit_alphanum4", Code: TFT, Issuer: TESTNET_ISSUER}

func hasTftTrustline(hAccount horizon.Account) bool {
	hasTftTrustline := false
	for _, b := range hAccount.Balances {
		if b.Asset == TestnetTftAsset {
			hasTftTrustline = true
			break
		}
	}

	return hasTftTrustline
}
