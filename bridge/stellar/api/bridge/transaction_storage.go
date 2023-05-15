package bridge

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stellar/go/clients/horizonclient"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type StellarTransactionStorage struct {
	network       string
	addressToScan string
	// knownTransactions is an in-memory cache of the transactions of the addressToScan account
	knownTransactions map[string]hProtocol.Transaction
	stellarCursor     string
}

func NewStellarTransactionStorage(network, addressToScan string) *StellarTransactionStorage {
	return &StellarTransactionStorage{
		network:           network,
		addressToScan:     addressToScan,
		knownTransactions: make(map[string]hProtocol.Transaction),
	}
}

// GetTransactionWithId returns a transaction with the given id (hash)
// returns error if the transaction is not found
func (s *StellarTransactionStorage) GetTransactionWithId(txid string) (*hProtocol.Transaction, error) {
	// trigger a rescan
	// will not rescan from start since we saved the cursor
	err := s.ScanBridgeAccount()
	if err != nil {
		return nil, nil
	}

	tx, ok := s.knownTransactions[txid]
	if !ok {
		return nil, errors.New("transaction not found")
	}
	return &tx, nil
}

// TransactionExists checks if a transaction exists on the stellar network
// it hashes the transaction and checks if the hash is in the list of known transactions
// this can be used to check if a transaction was already submitted to the stellar network
func (s *StellarTransactionStorage) TransactionExists(txn *txnbuild.Transaction) (bool, error) {
	// trigger a rescan
	// will not rescan from start since we saved the cursor
	err := s.ScanBridgeAccount()
	if err != nil {
		return false, nil
	}

	// check if the actual transaction already happened or not
	hash, err := txn.HashHex(GetNetworkPassPhrase(s.network))
	if err != nil {
		return false, errors.Wrap(err, "failed to get transaction hash")
	}

	_, ok := s.knownTransactions[hash]
	return ok, nil
}

// TransactionWithMemoExists checks if a transaction with the given memo exists on the stellar network and also scans the bridge account for new transactions
func (s *StellarTransactionStorage) TransactionWithMemoExists(txn *txnbuild.Transaction) (exists bool, err error) {
	memo, err := extractMemoFromTx(txn)
	if err != nil || memo == "" {
		return
	}

	log.Info("checking if transaction with memo exists in strorage..", "memo", memo)
	// txhash here is equal to memo
	for h := range s.knownTransactions {
		if h == memo {
			exists = true
		}
	}

	if !exists {
		return false, nil
	}

	return
}

// StoreTransaction stores a transaction in the transaction storage
func (s *StellarTransactionStorage) StoreTransaction(txn hProtocol.Transaction) {
	_, ok := s.knownTransactions[txn.Hash]
	if !ok {
		log.Info("storing transaction in the cache", "hash", txn.Hash)
		s.knownTransactions[txn.Hash] = txn
	}
}

func (s *StellarTransactionStorage) ScanBridgeAccount() error {
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

	log.Info("start fetching stellar transactions", "horizon", client.HorizonURL, "account", s.addressToScan, "cursor", s.stellarCursor)
	return fetchTransactions(context.Background(), client, s.addressToScan, s.stellarCursor, transactionHandler)
}

func extractMemoFromTx(txn *txnbuild.Transaction) (txMemoString string, err error) {
	memo := txn.Memo()

	if memo == nil {
		return "", nil
	}

	txMemo, err := txn.Memo().ToXDR()
	if err != nil {
		return "", err
	}

	switch txMemo.Type {
	case xdr.MemoTypeMemoHash:
		hashMemo := txn.Memo().(txnbuild.MemoHash)
		txMemoString = hex.EncodeToString(hashMemo[:])
	case xdr.MemoTypeMemoReturn:
		hashMemo := txn.Memo().(txnbuild.MemoReturn)
		txMemoString = hex.EncodeToString(hashMemo[:])
	default:
		return "", fmt.Errorf("transaction hash type not supported")
	}

	return
}

// GetHorizonClient gets the horizon client based on the transaction storage's network
func (s *StellarTransactionStorage) getHorizonClient() (*horizonclient.Client, error) {
	return GetHorizonClient(s.network)
}
