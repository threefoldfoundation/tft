package bridge

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stellar/go/clients/horizonclient"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type StellarTransactionStorage struct {
	network                   string
	addressToScan             string
	knownTransactionWithMemos map[string]struct{}
	stellarCursor             string
}

func NewStellarTransactionStorage(network, addressToScan string) *StellarTransactionStorage {
	return &StellarTransactionStorage{
		network:                   network,
		addressToScan:             addressToScan,
		knownTransactionWithMemos: make(map[string]struct{}),
	}
}

func (s *StellarTransactionStorage) TransactionWithMemoExists(txn *txnbuild.Transaction) (exists bool, err error) {
	return s.transactionWithMemoExists(txn)
}

func (s *StellarTransactionStorage) TransactionWithMemoExistsAndScan(txn *txnbuild.Transaction) (exists bool, err error) {
	exists, err = s.transactionWithMemoExists(txn)
	if err != nil {
		return
	}

	// trigger a rescan
	// will not rescan from start since we saved the cursor
	err = s.ScanBridgeAccount()
	if err != nil {
		return
	}

	// check again if memo hash exists now after a rescan
	exists, err = s.transactionWithMemoExists(txn)
	if err != nil {
		return
	}

	return
}

func (s *StellarTransactionStorage) transactionWithMemoExists(txn *txnbuild.Transaction) (exists bool, err error) {
	memo, err := s.memoToString(txn)
	if err != nil {
		return
	}
	log.Info("checking tx with", "memo", memo)

	_, ok := s.knownTransactionWithMemos[memo]
	if ok {
		return true, nil
	}

	return
}

func (s *StellarTransactionStorage) ScanBridgeAccount() error {
	if s.addressToScan == "" {
		return errors.New("no master bridge account set, aborting now")
	}
	log.Info("scanning account ", "account", s.addressToScan)

	transactionHandler := func(tx hProtocol.Transaction) {
		if tx.MemoType != "hash" && tx.MemoType != "return" {
			return
		}

		bytes, err := base64.StdEncoding.DecodeString(tx.Memo)
		if err != nil {
			return
		}
		memoAsHex := hex.EncodeToString(bytes)

		_, ok := s.knownTransactionWithMemos[memoAsHex]
		if !ok {
			log.Info("storing memo hash in known transaction storage", "hash", memoAsHex)
			// add the transaction memo to the list of known transaction memos
			s.knownTransactionWithMemos[memoAsHex] = struct{}{}
		}
	}

	err := s.FetchTransactions(context.Background(), s.stellarCursor, transactionHandler)
	if err != nil {
		return err
	}

	return nil
}

func (s *StellarTransactionStorage) FetchTransactions(ctx context.Context, cursor string, handler func(op hProtocol.Transaction)) error {
	client, err := s.getHorizonClient()
	if err != nil {
		return err
	}

	opRequest := horizonclient.TransactionRequest{
		ForAccount:    s.addressToScan,
		IncludeFailed: false,
		Cursor:        s.stellarCursor,
		Limit:         stellarPageLimit,
	}
	log.Info("Start fetching stellar transactions", "horizon", client.HorizonURL, "account", opRequest.ForAccount, "cursor", opRequest.Cursor)

	for {
		if ctx.Err() != nil {
			return nil
		}

		response, err := client.Transactions(opRequest)
		if err != nil {
			log.Info("Error getting transactions for stellar account", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
				continue
			}

		}
		for _, tx := range response.Embedded.Records {
			handler(tx)
			s.stellarCursor = tx.PagingToken()
			opRequest.Cursor = s.stellarCursor
		}
		if len(response.Embedded.Records) == 0 {
			return nil
		}

	}

}

func (s *StellarTransactionStorage) StoreTransactionWithMemo(txn *txnbuild.Transaction) error {
	memo, err := s.memoToString(txn)
	if err != nil {
		return err
	}

	_, ok := s.knownTransactionWithMemos[memo]
	if !ok {
		log.Info("storing memo hash in known transaction storage", "hash", memo)
		// add the transaction memo to the list of known transaction memos
		s.knownTransactionWithMemos[memo] = struct{}{}
		return nil
	}

	return fmt.Errorf("transaction with memo already exists")
}

func (s *StellarTransactionStorage) memoToString(txn *txnbuild.Transaction) (txMemoString string, err error) {
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
	switch s.network {
	case "testnet":
		return horizonclient.DefaultTestNetClient, nil
	case "production":
		return horizonclient.DefaultPublicNetClient, nil
	default:
		return nil, errors.New("network is not supported")
	}
}
