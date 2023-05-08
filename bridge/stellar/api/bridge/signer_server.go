package bridge

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/threefoldfoundation/tft/bridge/stellar/contracts/tokenv1"

	"github.com/multiformats/go-multiaddr"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

const (
	Protocol = protocol.ID("/p2p/rpc/signer")
)

type StellarSignRequest struct {
	TxnXDR             string
	RequiredSignatures int
	Receiver           common.Address
	Block              uint64
	Message            string
}

type EthSignRequest struct {
	Receiver           common.Address
	Amount             int64
	TxId               string
	RequiredSignatures int64
}

type StellarSignResponse struct {
	// Signature is a base64 of the signautre
	Signature string
	// The account address
	Address string
}

type EthSignResponse struct {
	Who       common.Address
	Signature tokenv1.Signature
}

type SignerService struct {
	bridgeContract            *BridgeContract
	StellarTransactionStorage *StellarTransactionStorage
	stellarWallet             *stellarWallet
	bridgeMasterAddress       string
}

func NewSignerServer(host host.Host, bridgeMasterAddress string, bridgeContract *BridgeContract, stellarWallet *stellarWallet) (*SignerService, error) {
	log.Info("server started", "identity", host.ID().Pretty())
	ipfs, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ipfs/%s", host.ID().Pretty()))
	if err != nil {
		return nil, err
	}

	for _, addr := range host.Addrs() {
		full := addr.Encapsulate(ipfs)
		log.Info("p2p node address", "address", full.String())
	}

	server := gorpc.NewServer(host, Protocol)

	stellarTransactionStorage := NewStellarTransactionStorage(stellarWallet.config.StellarNetwork, bridgeMasterAddress)

	signerService := SignerService{
		bridgeContract:            bridgeContract,
		StellarTransactionStorage: stellarTransactionStorage,
		stellarWallet:             stellarWallet,
		bridgeMasterAddress:       bridgeMasterAddress,
	}

	err = server.Register(&signerService)

	return &signerService, err
}

func (s *SignerService) SignMint(ctx context.Context, request EthSignRequest, response *EthSignResponse) error {
	log.Debug("sign mint request", "request txid", request.TxId)

	// Check in transaction storage if the deposit transaction exists
	tx, err := s.StellarTransactionStorage.GetTransactionWithId(request.TxId)
	if err != nil {
		return err
	}

	// Validate amount
	depositedAmount, err := s.stellarWallet.getAmountFromTx(request.TxId, s.bridgeMasterAddress)
	if err != nil {
		return err
	}
	log.Debug("validating amount for sign tx", "amount", depositedAmount, "request amount", request.Amount)

	depositFeeBigInt := big.NewInt(s.stellarWallet.config.DepositFeeInStroops())

	amount := &big.Int{}
	amount = amount.Sub(big.NewInt(depositedAmount), depositFeeBigInt)

	if amount.Int64() != request.Amount {
		return fmt.Errorf("amounts do not match")
	}

	log.Debug("tx memo", "memoType", tx.MemoType, "memo", tx.Memo)
	// Validate address
	addr, err := GetErc20AddressFromB64(tx.Memo)
	if err != nil {
		return err
	}

	if addr != ERC20Address(request.Receiver.Bytes()) {
		return fmt.Errorf("deposit addresses do not match")
	}

	signature, err := s.bridgeContract.CreateTokenSignature(request.Receiver, request.Amount, request.TxId)
	if err != nil {
		return err
	}

	response.Signature = signature
	response.Who = s.bridgeContract.ethc.address

	return nil
}

// Sign signs a stellar sign request
// This is calable on the libp2p network with RPC
func (s *SignerService) Sign(ctx context.Context, request StellarSignRequest, response *StellarSignResponse) error {
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
	} else {
		// If the signrequest is not a withdrawal request and a refund request
		// then it's most likely a transfer to fee wallet transaction
		err := s.validateFeeTransfer(request, txn)
		if err != nil {
			return err
		}
	}

	txn, err = txn.Sign(s.stellarWallet.GetNetworkPassPhrase(), s.stellarWallet.keypair)
	if err != nil {
		return err
	}

	signatures := txn.Signatures()
	if len(signatures) != 1 {
		return fmt.Errorf("invalid number of signatures on the transaction")
	}

	response.Address = s.stellarWallet.keypair.Address()
	response.Signature = base64.StdEncoding.EncodeToString(signatures[0].Signature)
	return nil
}

func (s *SignerService) validateWithdrawal(request StellarSignRequest, txn *txnbuild.Transaction) error {
	log.Info("Validating withdrawal request...")

	withdraw, err := s.bridgeContract.tftContract.filter.FilterWithdraw(&bind.FilterOpts{Start: request.Block}, []common.Address{request.Receiver})
	if err != nil {
		return err
	}

	if !withdraw.Next() {
		return fmt.Errorf("no withdraw event found")
	}

	amount := withdraw.Event.Tokens.Uint64()
	log.Info("validating withdrawal", "amount", amount, "receiver", withdraw.Event.BlockchainAddress, "network", withdraw.Event.Network)
	amount -= uint64(WithdrawFee)
	if len(txn.Operations()) != 2 {
		return fmt.Errorf("a withdraw tx needs to contain 2 payment operations")
	}
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
			return fmt.Errorf("transaction contains non payment operations")
		}

		acc := paymentOperation.Destination.ToAccountId()

		if acc.Address() == s.stellarWallet.config.StellarFeeWallet {
			if int64(paymentOperation.Amount) != WithdrawFee {
				return errors.New("the withdraw fee is incorrect")
			}
			continue
		}

		if acc.Address() != withdraw.Event.BlockchainAddress {
			return fmt.Errorf("destination is not correct, got %s, need %s", acc.Address(), withdraw.Event.BlockchainAddress)
		}

		if int64(paymentOperation.Amount) != int64(amount) {
			return fmt.Errorf("amount is not correct, received %d, need %d", paymentOperation.Amount, xdr.Int64(withdraw.Event.Tokens.Int64()))
		}

		// check if a similar transaction was made before
		exists, err := s.StellarTransactionStorage.TransactionExistsAndScan(txn)
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

func (s *SignerService) validateRefundTransaction(request StellarSignRequest, txn *txnbuild.Transaction) error {
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

		// Skip the fee wallet transaction
		if destinationAccount.Address() == s.stellarWallet.config.StellarFeeWallet {
			if paymentOperation.Amount != xdr.Int64(WithdrawFee) {
				return fmt.Errorf("fee payment operation should be %d, but got %d", WithdrawFee, paymentOperation.Amount)
			}
			continue
		}

		txnEffectsFromMessage, err := s.stellarWallet.getTransactionEffects(request.Message)
		if err != nil {
			return fmt.Errorf("failed to retrieve transaction from message")
		}

		// Check if the source transaction actually was sent from the account that we are trying to credit
		// This way we can infer that it is indeed a refund transaction
		for _, effect := range txnEffectsFromMessage.Embedded.Records {
			if effect.GetType() == "account_debited" {
				if effect.GetAccount() == s.stellarWallet.config.StellarFeeWallet {
					continue
				}

				if effect.GetAccount() != destinationAccount.Address() {
					return fmt.Errorf("destination is not correct, got %s, need user wallet %s", destinationAccount.Address(), effect.GetAccount())
				}
			}
		}

		// check if a similar transaction was made before
		exists, err := s.StellarTransactionStorage.TransactionExistsAndScan(txn)
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

func (s *SignerService) validateFeeTransfer(request StellarSignRequest, txn *txnbuild.Transaction) error {
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
			return fmt.Errorf("transaction contains non payment operations")
		}

		acc := paymentOperation.Destination.ToAccountId()
		if acc.Address() != s.stellarWallet.config.StellarFeeWallet {
			return fmt.Errorf("destination is not correct, got %s, need fee wallet %s", acc.Address(), s.stellarWallet.config.StellarFeeWallet)
		}

		// check if a similar transaction was made before
		exists, err := s.StellarTransactionStorage.TransactionExistsAndScan(txn)
		if err != nil {
			return errors.Wrap(err, "failed to check transaction storage for existing transaction hash")
		}
		// if the transaction exists, return with error
		if exists {
			log.Info("Transaction with this hash already executed, skipping now..")
			return fmt.Errorf("transaction with hash already exists")
		}

		switch int64(paymentOperation.Amount) {
		case s.stellarWallet.config.DepositFeeInStroops():
			return nil
		case WithdrawFee:
			return nil
		default:
			return fmt.Errorf("amount is not correct, received %d, need %d or %d", paymentOperation.Amount, s.stellarWallet.config.DepositFeeInStroops(), WithdrawFee)
		}

	}
	return nil
}
