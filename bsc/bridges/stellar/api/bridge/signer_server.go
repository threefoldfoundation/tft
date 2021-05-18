package bridge

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
	horizoneffects "github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

const (
	Protocol         = protocol.ID("/p2p/rpc/signer")
	stellarPageLimit = 100
)

type SignRequest struct {
	TxnXDR             string
	RequiredSignatures int
	Receiver           common.Address
	Block              uint64
	Message            string
}

type SignResponse struct {
	// Signature is a base64 of the signautre
	Signature string
	// The account address
	Address string
}

type SignerService struct {
	network               string
	kp                    *keypair.Full
	bridgeContract        *BridgeContract
	knownTransactionMemos map[string]struct{}
	bridgeMasterAddress   string
	cursor                string
}

func NewSignerServer(host host.Host, network, secret, bridgeMasterAddress string, bridgeContract *BridgeContract) (*SignerService, error) {
	log.Info("server started", "identity", host.ID().Pretty())
	ipfs, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ipfs/%s", host.ID().Pretty()))
	if err != nil {
		return nil, err
	}

	for _, addr := range host.Addrs() {
		full := addr.Encapsulate(ipfs)
		log.Info("p2p node address", "address", full.String())
	}

	_, signer, err := newSignerServer(host, network, secret, bridgeMasterAddress, bridgeContract)
	return signer, err
}

func (s *SignerService) Sign(ctx context.Context, request SignRequest, response *SignResponse) error {
	loaded, err := txnbuild.TransactionFromXDR(request.TxnXDR)
	if err != nil {
		return err
	}

	txn, ok := loaded.Transaction()
	if !ok {
		return fmt.Errorf("provided transaction is of wrong type")
	}

	if request.Block != 0 {
		err := s.validateWithdrawal(request, txn)
		if err != nil {
			return err
		}
	} else if request.Message != "" {
		// If the signrequest has a message attached we know it's a refund transaction
		err := s.validateRefundTransaction(request, txn)
		if err != nil {
			return err
		}
	}

	txn, err = txn.Sign(s.getNetworkPassPhrase(), s.kp)
	if err != nil {
		return err
	}

	signatures := txn.Signatures()
	if len(signatures) != 1 {
		return fmt.Errorf("invalid number of signatures on the transaction")
	}

	response.Address = s.kp.Address()
	response.Signature = base64.StdEncoding.EncodeToString(signatures[0].Signature)
	return nil
}

func newSignerServer(host host.Host, network, secret, bridgeMasterAddress string, bridgeContract *BridgeContract) (*gorpc.Server, *SignerService, error) {
	full, err := keypair.ParseFull(secret)
	if err != nil {
		return nil, nil, err
	}
	log.Debug("wallet address", "address", full.Address())
	server := gorpc.NewServer(host, Protocol)

	signer := SignerService{
		network:               network,
		kp:                    full,
		bridgeContract:        bridgeContract,
		bridgeMasterAddress:   bridgeMasterAddress,
		knownTransactionMemos: make(map[string]struct{}),
	}

	err = server.Register(&signer)
	return server, &signer, err
}

func (s *SignerService) validateWithdrawal(request SignRequest, txn *txnbuild.Transaction) error {
	log.Info("address to check", "address", request.Receiver)
	withdraw, err := s.bridgeContract.tftContract.filter.FilterWithdraw(&bind.FilterOpts{Start: request.Block}, []common.Address{request.Receiver})
	if err != nil {
		return err
	}

	if !withdraw.Next() {
		return fmt.Errorf("no withdraw event found")
	}

	log.Info("Withdraw event found", "event", withdraw)

	log.Info("got amount", "amount", withdraw.Event.Tokens.Uint64())
	log.Info("got receiver", "receiver", withdraw.Event.BlockchainAddress)
	log.Info("got network", "network", withdraw.Event.Network)

	for _, op := range txn.Operations() {
		opXDR, err := op.BuildXDR()
		if err != nil {
			return fmt.Errorf("failed to build operation xdr")
		}

		if opXDR.Body.Type != xdr.OperationTypePayment {
			continue
		}

		paymentOperation, ok := opXDR.Body.GetPaymentOp()
		if !ok {
			return fmt.Errorf("blabla")
		}

		acc := paymentOperation.Destination.ToAccountId()
		if acc.Address() != withdraw.Event.BlockchainAddress {
			return fmt.Errorf("destination is not correct, got %s, need %s", acc.Address(), withdraw.Event.BlockchainAddress)
		}

		if paymentOperation.Amount != xdr.Int64(withdraw.Event.Tokens.Int64()) {
			return fmt.Errorf("amount is not correct, received %d, need %d", paymentOperation.Amount, xdr.Int64(withdraw.Event.Tokens.Int64()))
		}

		err = s.checkExistingTransactionHash(txn)
		if err != nil {
			log.Error("error while checking transaction hash", "err", err.Error())
			return err
		}
	}

	return nil
}

func (s *SignerService) validateRefundTransaction(request SignRequest, txn *txnbuild.Transaction) error {
	log.Info("Validating refund request...")

	log.Info("number of operations to check", "length", len(txn.Operations()))
	for _, op := range txn.Operations() {
		opXDR, err := op.BuildXDR()
		if err != nil {
			return fmt.Errorf("failed to build operation xdr")
		}

		if opXDR.Body.Type != xdr.OperationTypePayment {
			continue
		}

		paymentOperation, ok := opXDR.Body.GetPaymentOp()
		if !ok {
			return fmt.Errorf("failed to get payment operation")
		}

		destinationAccount := paymentOperation.Destination.ToAccountId()

		txnEffectsFromMessage, err := s.getTransactionEffects(request.Message)
		if err != nil {
			return fmt.Errorf("failed to retrieve transaction from message")
		}

		// Check if the source transaction actually was sent from the account that we are trying to credit
		// This way we can infer that it is indeed a refund transaction
		for _, effect := range txnEffectsFromMessage.Embedded.Records {
			if effect.GetType() == "account_debited" {
				if destinationAccount.Address() != effect.GetAccount() {
					return fmt.Errorf("destination is not correct, got %s, need user wallet %s", destinationAccount.Address(), effect.GetAccount())
				}

				if paymentOperation.Amount > DepositFee {
					return fmt.Errorf("amount is not correct, we expected a refund transaction with an amount less than %d but we received %d", DepositFee, paymentOperation.Amount)
				}
			}
		}

	}
	return nil
}
func (s *SignerService) checkExistingTransactionHash(txn *txnbuild.Transaction) error {
	txMemo, err := txn.Memo().ToXDR()
	if err != nil {
		return err
	}

	// only check transaction with hash memos
	if txMemo.Type != xdr.MemoTypeMemoHash {
		return nil
	}

	hashMemo := txn.Memo().(txnbuild.MemoHash)
	txMemoString := hex.EncodeToString(hashMemo[:])

	_, ok := s.knownTransactionMemos[txMemoString]
	if ok {
		return fmt.Errorf("transaction with memo %s already exists on bridge account %s", txMemoString, txn.SourceAccount().AccountID)
	}

	// trigger a rescan
	// will not rescan from start since we saved the cursor
	err = s.ScanBridgeAccount()
	if err != nil {
		return err
	}

	_, ok = s.knownTransactionMemos[txMemoString]
	if ok {
		return fmt.Errorf("transaction with memo %s already exists on bridge account %s", txMemoString, txn.SourceAccount().AccountID)
	}
	log.Info("transaction not found")

	return nil
}

func (s *SignerService) ScanBridgeAccount() error {
	if s.bridgeMasterAddress == "" {
		return errors.New("no master bridge account set, aborting now")
	}

	transactionHandler := func(tx hProtocol.Transaction) {
		if tx.MemoType != "hash" {
			return
		}

		bytes, err := base64.StdEncoding.DecodeString(tx.Memo)
		if err != nil {
			return
		}
		memoAsHex := hex.EncodeToString(bytes)

		_, ok := s.knownTransactionMemos[memoAsHex]
		if !ok {
			log.Info("found", "x", memoAsHex)
			// add the transaction memo to the list of known transaction memos
			s.knownTransactionMemos[memoAsHex] = struct{}{}
		}
	}

	err := s.FetchTransactions(context.Background(), s.cursor, transactionHandler)
	if err != nil {
		return err
	}

	return nil
}

// getNetworkPassPhrase gets the Stellar network passphrase based on the wallet's network
func (s *SignerService) getNetworkPassPhrase() string {
	switch s.network {
	case "testnet":
		return network.TestNetworkPassphrase
	case "production":
		return network.PublicNetworkPassphrase
	default:
		return network.TestNetworkPassphrase
	}
}

// GetHorizonClient gets the horizon client based on the wallet's network
func (s *SignerService) getHorizonClient() (*horizonclient.Client, error) {
	switch s.network {
	case "testnet":
		return horizonclient.DefaultTestNetClient, nil
	case "production":
		return horizonclient.DefaultPublicNetClient, nil
	default:
		return nil, errors.New("network is not supported")
	}
}

func (s *SignerService) getTransactionEffects(txHash string) (effects horizoneffects.EffectsPage, err error) {
	client, err := s.getHorizonClient()
	if err != nil {
		return effects, err
	}

	effectsReq := horizonclient.EffectRequest{
		ForTransaction: txHash,
	}
	effects, err = client.Effects(effectsReq)
	if err != nil {
		return effects, err
	}

	return effects, nil
}

func (s *SignerService) FetchTransactions(ctx context.Context, cursor string, handler func(op hProtocol.Transaction)) error {
	client, err := s.getHorizonClient()
	if err != nil {
		return err
	}

	opRequest := horizonclient.TransactionRequest{
		ForAccount:    s.bridgeMasterAddress,
		IncludeFailed: false,
		Cursor:        s.cursor,
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
			s.cursor = tx.PagingToken()
			opRequest.Cursor = s.cursor
		}
		if len(response.Embedded.Records) == 0 {
			return nil
		}

	}

}
