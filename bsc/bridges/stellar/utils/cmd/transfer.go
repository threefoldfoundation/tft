package cmd

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
)

func Transfer(secret, destination, memo string, amount string) error {
	kp, err := keypair.ParseFull(secret)
	if err != nil {
		return errors.Wrap(err, "invalid stellar secret")
	}

	accountRequest := horizonclient.AccountRequest{AccountID: kp.Address()}
	hAccount, err := HorizonClient.AccountDetail(accountRequest)
	if err != nil {
		return errors.Wrap(err, "account does not exist")
	}

	if !hasTftTrustline(hAccount) {
		return errors.New("source account does not have trustline")
	}

	destAccountRequest := horizonclient.AccountRequest{AccountID: destination}
	destHAccount, err := HorizonClient.AccountDetail(destAccountRequest)
	if err != nil {
		return errors.Wrap(err, "account does not exist")
	}

	if !hasTftTrustline(destHAccount) {
		return errors.New("destination account does not have trustline")
	}

	transferTx := txnbuild.Payment{
		Destination:   destination,
		Amount:        amount,
		Asset:         TestnetTft,
		SourceAccount: kp.Address(),
	}

	params := txnbuild.TransactionParams{
		SourceAccount:        &hAccount,
		IncrementSequenceNum: true,
		Operations:           []txnbuild.Operation{&transferTx},
		BaseFee:              txnbuild.MinBaseFee,
		Memo:                 txnbuild.MemoText(memo),
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
