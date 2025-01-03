package stellar

import (
	"context"
	"encoding/base64"
	"encoding/hex"

	"github.com/rs/zerolog/log"
	"github.com/stellar/go/clients/horizonclient"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/solana"
)

type TransactionStorage struct {
	network       string
	addressToScan string
	// transactions is an in-memory cache of the transactions of the addressToScan account
	transactions map[string]hProtocol.Transaction
	// sentTransactionMemos keeps the memo's of outgoing transactions of the addressToScan account
	// this is used to check if a withdraw, refund or feetransfer for a deposit has already occurred
	sentTransactionMemos map[string]bool
	stellarCursor        string
}

var ErrTransactionNotFound = errors.New("transaction not found")

func NewTransactionStorage(network, addressToScan string) *TransactionStorage {
	return &TransactionStorage{
		network:              network,
		addressToScan:        addressToScan,
		transactions:         make(map[string]hProtocol.Transaction),
		sentTransactionMemos: make(map[string]bool),
	}
}

// GetTransactionWithId returns a transaction with the given id (hash)
// returns error if the transaction is not found
func (s *TransactionStorage) GetTransactionWithId(txid string) (tx *hProtocol.Transaction, err error) {
	// trigger a rescan
	// will not rescan from start since we saved the cursor
	err = s.ScanBridgeAccount()
	if err != nil {
		return
	}

	foundTx, ok := s.transactions[txid]
	if !ok {
		err = ErrTransactionNotFound
		return
	}
	tx = &foundTx
	return
}

// TransactionExists checks if a transaction exists on the stellar network
// it hashes the transaction and checks if the hash is in the list of known transactions
// this can be used to check if a transaction was already submitted to the stellar network
func (s *TransactionStorage) TransactionExists(txn *txnbuild.Transaction) (exists bool, err error) {
	// trigger a rescan
	// will not rescan from start since we saved the cursor
	err = s.ScanBridgeAccount()
	if err != nil {
		return
	}

	// check if the actual transaction already happened or not
	hash, err := txn.HashHex(GetNetworkPassPhrase(s.network))
	if err != nil {
		return false, errors.Wrap(err, "failed to get transaction hash")
	}

	_, ok := s.transactions[hash]
	return ok, nil
}

// TransactionWithShortTxIDExists checks if a transaction is already executed with the given short tx id as memo.
func (s *TransactionStorage) TransactionWithShortTxIDExists(shortID solana.ShortTxID) (bool, error) {
	return s.TransactionWithMemoExists(hex.EncodeToString(shortID[:]))
}

// TransactionWithMemoExists checks if a transaction with the given memo exists
func (s *TransactionStorage) TransactionWithMemoExists(memo string) (exists bool, err error) {
	err = s.ScanBridgeAccount()
	if err != nil {
		return
	}
	log.Debug().Str("memo", memo).Msg("checking if transaction with memo exists in the cache")
	_, exists = s.sentTransactionMemos[memo]
	return
}

// StoreTransaction stores a transaction in the cache
// If there is a memo of type hash or return
// and the transaction is created by the account being watched ( the bridge vault account),
// the memo is kept as well to know that a withdraw, refund or fee transfer already happened.
func (s *TransactionStorage) StoreTransaction(tx hProtocol.Transaction) {
	_, ok := s.transactions[tx.Hash]
	if !ok {
		log.Debug().Str("hash", tx.Hash).Msg("storing transaction in the cache")
		s.transactions[tx.Hash] = tx
		if tx.Account == s.addressToScan {
			if tx.MemoType == "hash" || tx.MemoType == "return" {

				bytes, err := base64.StdEncoding.DecodeString(tx.Memo)
				if err != nil {
					log.Error().Str("tx", tx.Hash).Msg("Unable to base64 decode a transaction memo")
				} else {
					memoAsHex := hex.EncodeToString(bytes)
					log.Debug().Str("tx", tx.Hash).Str("memo", memoAsHex).Msg("Remembering memo of transaction")
					s.sentTransactionMemos[memoAsHex] = true
				}

			}
		}

	}
}

func (s *TransactionStorage) ScanBridgeAccount() error {
	if s.addressToScan == "" {
		return errors.New("no account set, aborting now")
	}

	transactionHandler := func(tx hProtocol.Transaction) {
		s.StoreTransaction(tx)
		s.stellarCursor = tx.PagingToken()
	}

	client, err := s.getHorizonClient()
	if err != nil {
		return err
	}

	log.Debug().Str("account", s.addressToScan).Str("cursor", s.stellarCursor).Msg("start fetching stellar transactions")
	// TODO: we should not use the background context here
	return fetchTransactions(context.Background(), client, s.addressToScan, s.stellarCursor, transactionHandler)
}

// GetHorizonClient gets the horizon client based on the transaction storage's network
func (s *TransactionStorage) getHorizonClient() (*horizonclient.Client, error) {
	return GetHorizonClient(s.network)
}
