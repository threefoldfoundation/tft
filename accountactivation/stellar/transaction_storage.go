package stellar

import (
	"context"
	"encoding/base64"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/log"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/errors"
)

type TransactionStorage struct {
	network       string
	addressToScan string
	// sentTransactionMemos keeps the memo's of outgoing transactions of the addressToScan account
	// this is used to check if an accountactivation has already been executed
	TransactionMemos map[string]bool
	stellarCursor    string
}

var ErrTransactionNotFound = errors.New("transaction not found")

func NewTransactionStorage(network, addressToScan string) *TransactionStorage {
	return &TransactionStorage{
		network:          network,
		addressToScan:    addressToScan,
		TransactionMemos: make(map[string]bool),
	}
}

// TransactionWithMemoExists checks if a transaction with the given memo exists
func (s *TransactionStorage) TransactionWithMemoExists(memo string) (exists bool, err error) {
	err = s.ScanAccount()
	if err != nil {
		return
	}
	log.Debug("checking if transaction with memo exists in the cache", "memo", memo)
	_, exists = s.TransactionMemos[memo]
	return
}

// StoreTransaction stores a transaction in the cache
// If there is a memo of type hash
// and the transaction is created by the account being watched,
// the memo is kept as well to know that an activation already happened.
func (s *TransactionStorage) StoreTransaction(tx hProtocol.Transaction) {

	if tx.Account == s.addressToScan {
		if tx.MemoType == "hash" {

			bytes, err := base64.StdEncoding.DecodeString(tx.Memo)
			if err != nil {
				log.Error("Unable to base64 decode a transaction memo", "tx", tx.Hash)
				// Something is really wrong, bail out
				panic(err)
			} else {
				memoAsHex := hex.EncodeToString(bytes)
				log.Debug("Remembering memo of transaction", "tx", tx.Hash, "memo", memoAsHex)
				s.TransactionMemos[memoAsHex] = true
			}
		}

	}
}

func (s *TransactionStorage) ScanAccount() error {
	if s.addressToScan == "" {
		return errors.New("no account set, aborting now")
	}

	transactionHandler := func(tx hProtocol.Transaction) (err error) {
		s.StoreTransaction(tx)
		s.stellarCursor = tx.PagingToken()
		return
	}

	client, err := GetHorizonClient(s.network)
	if err != nil {
		return err
	}

	log.Debug("fetching stellar transactions", "account", s.addressToScan, "cursor", s.stellarCursor)

	return fetchTransactions(context.TODO(), client, s.addressToScan, s.stellarCursor, transactionHandler)
}
