package stellar

import (
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/support/log"
	"github.com/stellar/go/txnbuild"
)

type Wallet struct {
	keypair *keypair.Full
	network string
}

func NewWallet(secret string, network string) Wallet {
	kp := keypair.MustParseFull(secret)
	return Wallet{
		keypair: kp,
		network: network,
	}

}
func (w *Wallet) ActivateAccount(account string, memoHash [32]byte) (err error) {
	if !IsValidStellarAddress(account) {
		return ErrInvalidAddress
	}

	op := txnbuild.CreateAccount{
		Destination: account,
		Amount:      "2",
	}
	client, err := GetHorizonClient(w.network)
	if err != nil {
		return
	}
	ar := horizonclient.AccountRequest{AccountID: w.keypair.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	if err != nil {
		return
	}
	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &sourceAccount,
		IncrementSequenceNum: true,
		BaseFee:              1000000,
		Operations:           []txnbuild.Operation{&op},
		Preconditions: txnbuild.Preconditions{
			TimeBounds: txnbuild.NewTimeout(300),
		},
		Memo: txnbuild.MemoHash(memoHash),
	})
	if err != nil {
		return
	}

	// Sign the transaction
	tx, err = tx.Sign(GetNetworkPassPhrase(w.network), w.keypair)
	if err != nil {
		return
	}

	txe, err := tx.Base64()
	if err != nil {
		return
	}

	// Send the transaction to the network
	resp, err := client.SubmitTransactionXDR(txe)
	if err != nil {
		if horizonError, ok := err.(*horizonclient.Error); ok {
			if resultcodes, resultcodesErr := horizonError.ResultCodes(); resultcodesErr == nil {
				if resultcodes != nil && resultcodes.OperationCodes != nil {
					for _, opResult := range resultcodes.OperationCodes {
						if opResult == "op_already_exists" {
							err = ErrAccountAlreadyExists
						}
					}
				}
			}
		}
		return
	}
	log.Info("Activated account", "StellarTx", resp.ID)
	return
}
