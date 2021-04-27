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
	"github.com/pkg/errors"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

const (
	Protocol = protocol.ID("/p2p/rpc/signer")
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
	kp             *keypair.Full
	bridgeContract *BridgeContract
	config         *StellarConfig
}

func NewSignerServer(host host.Host, config *StellarConfig, bridgeContract *BridgeContract) error {
	log.Info("server started", "identity", host.ID().Pretty())
	ipfs, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ipfs/%s", host.ID().Pretty()))
	if err != nil {
		return err
	}

	for _, addr := range host.Addrs() {
		full := addr.Encapsulate(ipfs)
		log.Info("p2p node address", "address", full.String())
	}

	_, err = newSignerServer(host, config, bridgeContract)
	return err
}

func newSignerServer(host host.Host, config *StellarConfig, bridgeContract *BridgeContract) (*gorpc.Server, error) {
	full, err := keypair.ParseFull(config.StellarSeed)
	if err != nil {
		return nil, err
	}
	log.Debug("wallet address", "address", full.Address())
	server := gorpc.NewServer(host, Protocol)

	signer := SignerService{
		kp:             full,
		bridgeContract: bridgeContract,
		config:         config,
	}

	err = server.Register(&signer)
	return server, err
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

	// If the signrequest is withdrawal request at specified smart chain blockheight
	// Validate if it is valid
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
	} else {
		// If the signrequest is not a withdrawal request and a refund request
		// then it's most likely a transfer to fee wallet transaction
		err := s.validateFeeTransfer(request, txn)
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
	}
	return nil
}

func (s *SignerService) validateFeeTransfer(request SignRequest, txn *txnbuild.Transaction) error {
	log.Info("Validating fee transfer request...")

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
		if acc.Address() != s.config.StellarFeeWallet {
			return fmt.Errorf("destination is not correct, got %s, need fee wallet %s", acc.Address(), s.config.StellarFeeWallet)
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
			return fmt.Errorf("blabla")
		}

		destinationAccount := paymentOperation.Destination.ToAccountId()

		log.Info("destination address", "address", destinationAccount)
		log.Info("stellar fee wallet", "feewallet", s.config.StellarFeeWallet)
		// ignore fee wallet transactions here
		if destinationAccount.Address() == s.config.StellarFeeWallet {
			log.Info("fee wallet transfer, breaking ...")
			continue
		}

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

				if paymentOperation.Amount > feeAmount {
					return fmt.Errorf("amount is not correct, we expected a refund transaction with an amount less than %f but we received %d", feeAmount, paymentOperation.Amount)
				}
			}
		}

	}
	return nil
}

// getNetworkPassPhrase gets the Stellar network passphrase based on the wallet's network
func (s *SignerService) getNetworkPassPhrase() string {
	switch s.config.StellarNetwork {
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
	switch s.config.StellarNetwork {
	case "testnet":
		return horizonclient.DefaultTestNetClient, nil
	case "production":
		return horizonclient.DefaultPublicNetClient, nil
	default:
		return nil, errors.New("network is not supported")
	}
}

func (s *SignerService) getTransactionEffects(txHash string) (effects effects.EffectsPage, err error) {
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
