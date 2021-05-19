package bridge

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/multiformats/go-multiaddr"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	horizoneffects "github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/support/errors"
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
	network                   string
	kp                        *keypair.Full
	bridgeContract            *BridgeContract
	stellarTransactionStorage *StellarTransactionStorage
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

func (s *SignerService) validateWithdrawal(request SignRequest, txn *txnbuild.Transaction) error {
	log.Info("Validating withdrawal request...")

	withdraw, err := s.bridgeContract.tftContract.filter.FilterWithdraw(&bind.FilterOpts{Start: request.Block}, []common.Address{request.Receiver})
	if err != nil {
		return err
	}

	if !withdraw.Next() {
		return fmt.Errorf("no withdraw event found")
	}

	log.Info("validating withdrawal", "amount", withdraw.Event.Tokens.Uint64(), "receiver", withdraw.Event.BlockchainAddress, "network", withdraw.Event.Network)

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

		// check if a similar transaction was made before
		exists, err := s.stellarTransactionStorage.TransactionWithMemoExistsAndScan(txn)
		if err != nil {
			return errors.Wrap(err, "failed to check transaction storage for existing transaction hash")
		}
		// if the transaction exists, return with error
		if exists {
			log.Info("Transaction with this hash already executed, skipping now..")
			return fmt.Errorf("transaction with hash already exists")
		}
	}

	return nil
}

func (s *SignerService) validateRefundTransaction(request SignRequest, txn *txnbuild.Transaction) error {
	log.Info("Validating refund request...")

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

		// check if a similar transaction was made before
		exists, err := s.stellarTransactionStorage.TransactionWithMemoExistsAndScan(txn)
		if err != nil {
			return errors.Wrap(err, "failed to check transaction storage for existing transaction hash")
		}
		// if the transaction exists, return with error
		if exists {
			log.Info("Transaction with this hash already executed, skipping now..")
			return fmt.Errorf("transaction with hash already exists")
		}
	}
	return nil
}

func (s *SignerService) ScanBridgeAccount() error {
	return s.stellarTransactionStorage.ScanBridgeAccount()
}

func newSignerServer(host host.Host, network, secret, bridgeMasterAddress string, bridgeContract *BridgeContract) (*gorpc.Server, *SignerService, error) {
	full, err := keypair.ParseFull(secret)
	if err != nil {
		return nil, nil, err
	}
	log.Debug("wallet address", "address", full.Address())
	server := gorpc.NewServer(host, Protocol)

	stellarTransactionStorage := NewStellarTransactionStorage(network, bridgeMasterAddress)

	signer := SignerService{
		network:                   network,
		kp:                        full,
		bridgeContract:            bridgeContract,
		stellarTransactionStorage: stellarTransactionStorage,
	}

	err = server.Register(&signer)
	return server, &signer, err
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

// GetHorizonClient gets the horizon client based on the transaction storage's network
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
