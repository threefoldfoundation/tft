package stellar

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/ethereum/go-ethereum/log"
	hProtocol "github.com/stellar/go/protocols/horizon"
)

type TransactionStorage struct {
	network       string
	addressToScan string
	// sentTransactionMemos keeps the memo's of outgoing transactions of the addressToScan account
	// this is used to check if an accountactivation has already been executed
	TransactionMemos map[string]bool
	stellarCursor    string
}

func NewTransactionStorage(network, addressToScan string) *TransactionStorage {
	return &TransactionStorage{
		network:          network,
		addressToScan:    addressToScan,
		TransactionMemos: make(map[string]bool),
	}
}

// TransactionWithMemoExists checks if a transaction with the given memo exists
// Will return a context.Canceled error if the context is canceled
func (s *TransactionStorage) TransactionWithMemoExists(ctx context.Context, memo string) (exists bool, err error) {
	err = s.ScanAccount(ctx)
	for err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Warn("Failed to Scan the activation account", "err", err)
		err = s.ScanAccount(ctx)
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
			}
			memoAsHex := hex.EncodeToString(bytes)
			log.Debug("Remembering memo of transaction", "tx", tx.Hash, "memo", memoAsHex)
			s.TransactionMemos[memoAsHex] = true

		}

	}
}

func (s *TransactionStorage) ScanAccount(ctx context.Context) error {
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

	return fetchTransactions(ctx, client, s.addressToScan, s.stellarCursor, transactionHandler)
}
