package cmd

import (
	"fmt"
	"net/http"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
)

const (
	TFT            = "TFT"
	TESTNET_ISSUER = "GA47YZA3PKFUZMPLQ3B5F2E3CJIB57TGGU7SPCQT2WAEYKN766PWIMB3"
)

var client = horizonclient.DefaultTestNetClient

// Generates and activates account on the stellar testnet
func GenerateAndActivateAccounts(count int) ([]keypair.Full, error) {
	accounts := make([]keypair.Full, 0)
	for i := 0; i < count; i++ {
		kp, err := keypair.Random()
		if err != nil {
			return nil, err
		}

		err = activateAccount(kp.Address())
		if err != nil {
			return nil, err
		}

		createTftTrustlineOperation := txnbuild.ChangeTrust{
			Line: txnbuild.ChangeTrustAssetWrapper{
				Asset: txnbuild.CreditAsset{Code: TFT, Issuer: TESTNET_ISSUER},
			},
			Limit:         "",
			SourceAccount: kp.Address(),
		}

		// Get information about the account we just created
		accountRequest := horizonclient.AccountRequest{AccountID: kp.Address()}
		hAccount, err := client.AccountDetail(accountRequest)
		if err != nil {
			return nil, err
		}

		params := txnbuild.TransactionParams{
			SourceAccount:        &hAccount,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&createTftTrustlineOperation},
			BaseFee:              txnbuild.MinBaseFee,
			Memo:                 nil,
			Preconditions: txnbuild.Preconditions{
				TimeBounds: txnbuild.NewInfiniteTimeout(),
			},
		}
		tx, err := txnbuild.NewTransaction(params)
		if err != nil {
			return nil, err
		}

		// Sign the transaction, and base 64 encode its XDR representation
		signedTx, _ := tx.Sign(network.TestNetworkPassphrase, kp)
		txeBase64, _ := signedTx.Base64()

		// Submit the transaction
		_, err = client.SubmitTransactionXDR(txeBase64)
		if err != nil {
			hError := err.(*horizonclient.Error)
			fmt.Println(hError.Problem.Extras)
			return nil, hError
		}

		accounts = append(accounts, *kp)
	}

	return accounts, nil
}

func SetAccountOptions(keypairs []keypair.Full) error {
	masterKey := keypairs[0]
	majority := (len(keypairs) / 2) + 1

	setOptionsOperations := make([]txnbuild.Operation, 0)
	for i := 1; i < len(keypairs); i++ {
		activeSigner := keypairs[i]
		setOptions := txnbuild.SetOptions{
			InflationDestination: nil,
			ClearFlags:           nil,
			SetFlags:             nil,
			MasterWeight:         txnbuild.NewThreshold(*txnbuild.NewThreshold(1)),
			LowThreshold:         txnbuild.NewThreshold(txnbuild.Threshold(0)),
			MediumThreshold:      txnbuild.NewThreshold(txnbuild.Threshold(majority)),
			HighThreshold:        txnbuild.NewThreshold(txnbuild.Threshold(majority)),
			HomeDomain:           nil,
			Signer:               &txnbuild.Signer{Address: activeSigner.Address(), Weight: 1},
			SourceAccount:        masterKey.Address(),
		}

		setOptionsOperations = append(setOptionsOperations, &setOptions)
	}

	// Get information about the account we just created
	accountRequest := horizonclient.AccountRequest{AccountID: masterKey.Address()}
	hAccount, err := client.AccountDetail(accountRequest)
	if err != nil {
		return err
	}

	params := txnbuild.TransactionParams{
		SourceAccount:        &hAccount,
		IncrementSequenceNum: true,
		Operations:           setOptionsOperations,
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
	signedTx, _ := tx.Sign(network.TestNetworkPassphrase, &masterKey)
	txeBase64, _ := signedTx.Base64()

	// Submit the transaction
	_, err = client.SubmitTransactionXDR(txeBase64)
	if err != nil {
		hError := err.(*horizonclient.Error)
		fmt.Println(hError.Problem.Extras)
		return hError
	}

	return nil
}

func activateAccount(addr string) error {
	_, err := http.Get("https://friendbot.stellar.org/?addr=" + addr)
	if err != nil {
		return err
	}

	return nil
}
