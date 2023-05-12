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
	network           string
	addressToScan     string
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

// TransactionWithMemoExists checks if a transaction with the given memo exists on the stellar network
func (s *StellarTransactionStorage) TransactionWithMemoExists(txn *txnbuild.Transaction) (exists bool, err error) {
	return s.transactionWthMemoExists(txn)
}

// TransactionWithMemoExistsAndScan checks if a transaction with the given memo exists on the stellar network and also scans the bridge account for new transactions
func (s *StellarTransactionStorage) TransactionWithMemoExistsAndScan(txn *txnbuild.Transaction) (exists bool, err error) {
	exists, err = s.transactionWthMemoExists(txn)
	if err != nil {
		return
	}

	// trigger a rescan
	// will not rescan from start since we saved the cursor
	err = s.ScanBridgeAccount()
	if err != nil {
		return
	}

	return s.transactionWthMemoExists(txn)
}

func (s *StellarTransactionStorage) transactionWthMemoExists(txn *txnbuild.Transaction) (exists bool, err error) {
	memo, err := s.memoToString(txn)
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

func (s *StellarTransactionStorage) ScanBridgeAccount() error {
	if s.addressToScan == "" {
		return errors.New("no master bridge account set, aborting now")
	}
	log.Info("scanning account ", "account", s.addressToScan)

	transactionHandler := func(tx hProtocol.Transaction) {
		_, ok := s.knownTransactions[tx.Hash]
		if !ok {
			log.Info("storing memo hash in known transaction storage", "hash", tx.Hash)
			s.knownTransactions[tx.Hash] = tx
		}
		s.stellarCursor = tx.PagingToken()
	}

	err := s.FetchTransactionsForStorage(context.Background(), s.stellarCursor, transactionHandler)
	if err != nil {
		return err
	}

	return nil
}

func (s *StellarTransactionStorage) FetchTransactionsForStorage(ctx context.Context, cursor string, handler func(op hProtocol.Transaction)) error {
	client, err := s.getHorizonClient()
	if err != nil {
		return err
	}

	log.Info("Start fetching stellar transactions", "horizon", client.HorizonURL, "account", s.addressToScan, "cursor", s.stellarCursor)
	return fetchTransactions(ctx, client, s.addressToScan, s.stellarCursor, handler)

}

func (s *StellarTransactionStorage) StoreTransaction(txn hProtocol.Transaction) {
	_, ok := s.knownTransactions[txn.Hash]
	if !ok {
		log.Info("storing memo hash in known transaction storage", "hash", txn.Hash)
		// add the transaction memo to the list of known transaction memos
		s.knownTransactions[txn.Hash] = txn
	}
}

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

func (s *StellarTransactionStorage) memoToString(txn *txnbuild.Transaction) (txMemoString string, err error) {
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
