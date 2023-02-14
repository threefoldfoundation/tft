package cmd

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
)

func GetTestnetTFT(secret string) error {
	kp, err := keypair.ParseFull(secret)
	if err != nil {
		return errors.Wrap(err, "invalid stellar secret")
	}

	accountRequest := horizonclient.AccountRequest{AccountID: kp.Address()}
	hAccount, err := HorizonClient.AccountDetail(accountRequest)
	if err != nil {
		return errors.Wrap(err, "account does not exist")
	}

	totalBalance, err := hAccount.GetNativeBalance()
	if err != nil {
		return err
	}

	hasTftTrustline := false
	for _, b := range hAccount.Balances {
		if b.Asset == TestnetTftAsset {
			hasTftTrustline = true
			break
		}
	}

	if !hasTftTrustline {
		return errors.New("account has no valid TFT trustline")
	}

	receiveTx := txnbuild.PathPaymentStrictReceive{
		SendAsset:     txnbuild.NativeAsset{},
		SendMax:       totalBalance,
		Destination:   kp.Address(),
		DestAsset:     TestnetTft,
		SourceAccount: kp.Address(),
		DestAmount:    "10",
	}

	params := txnbuild.TransactionParams{
		SourceAccount:        &hAccount,
		IncrementSequenceNum: true,
		Operations:           []txnbuild.Operation{&receiveTx},
		BaseFee:              txnbuild.MinBaseFee,
		Memo:                 nil,
		Preconditions: txnbuild.Preconditions{
			TimeBounds: txnbuild.NewInfiniteTimeout(),
		},
	}
	tx, err := txnbuild.NewTransaction(params)
	if err != nil {
		return err
	}

	// Sign the transaction, and base 64 encode its XDR representation
	signedTx, _ := tx.Sign(network.TestNetworkPassphrase, kp)
	txeBase64, _ := signedTx.Base64()

	// Submit the transaction
	_, err = HorizonClient.SubmitTransactionXDR(txeBase64)
	if err != nil {
		hError := err.(*horizonclient.Error)
		fmt.Println(hError.Problem.Extras)
		return hError
	}

	return nil
}
