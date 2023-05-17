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
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/contracts/tokenv1"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/eth"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/multisig"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/stellar"

	"github.com/multiformats/go-multiaddr"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

const (
	Protocol = protocol.ID("/p2p/rpc/signer")
)

var (
	ErrTransactionAlreadyExists = errors.New("transaction already exists")
)

type EthSignRequest struct {
	Receiver           common.Address
	Amount             int64
	TxId               string
	RequiredSignatures int64
}

type EthSignResponse struct {
	Who       common.Address
	Signature tokenv1.Signature
}

type SignerService struct {
	bridgeContract      *BridgeContract
	stellarWallet       *stellar.Wallet
	bridgeMasterAddress string
	depositFee          int64 // deposit fee in TFT units TODO: maybe just pass part of the comfig
}

func NewSignerServer(host host.Host, bridgeMasterAddress string, bridgeContract *BridgeContract, stellarWallet *stellar.Wallet, depositFee int64) error {
	log.Info("server started", "identity", host.ID().Pretty())
	partialMA, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", host.ID()))
	if err != nil {
		return err
	}

	for _, addr := range host.Addrs() {
		full := addr.Encapsulate(partialMA)
		log.Info("p2p node address", "address", full.String())
	}

	server := gorpc.NewServer(host, Protocol)

	signerService := SignerService{
		bridgeContract:      bridgeContract,
		stellarWallet:       stellarWallet,
		bridgeMasterAddress: bridgeMasterAddress,
		depositFee:          depositFee,
	}

	return server.Register(&signerService)
}

func (s *SignerService) SignMint(ctx context.Context, request EthSignRequest, response *EthSignResponse) error {
	log.Info("sign mint request", "request txid", request.TxId)

	// Check in transaction storage if the deposit transaction exists
	tx, err := s.stellarWallet.TransactionStorage.GetTransactionWithId(request.TxId)
	if err != nil {
		log.Info("transaction not found", "txid", request.TxId)
		return err
	}

	// Validate amount
	depositedAmount, err := s.stellarWallet.GetAmountFromTx(request.TxId, s.bridgeMasterAddress)
	if err != nil {
		return err
	}
	log.Debug("validating amount for sign tx", "amount", depositedAmount, "request amount", request.Amount)

	depositFeeBigInt := big.NewInt(stellar.IntToStroops(s.depositFee))

	amount := &big.Int{}
	amount = amount.Sub(big.NewInt(depositedAmount), depositFeeBigInt)

	if amount.Int64() != request.Amount {
		return fmt.Errorf("amounts do not match")
	}

	log.Debug("tx memo", "memoType", tx.MemoType, "memo", tx.Memo)
	// Validate address
	addr, err := eth.GetErc20AddressFromB64(tx.Memo)
	if err != nil {
		return err
	}

	if addr != eth.ERC20Address(request.Receiver.Bytes()) {
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
func (s *SignerService) Sign(ctx context.Context, request multisig.StellarSignRequest, response *multisig.StellarSignResponse) error {
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
		log.Info("Validating fee transfer signing request")
		err := s.validateFeeTransfer(request, txn)
		if err != nil {
			if err == ErrTransactionAlreadyExists {
				log.Info("A Fee transfer with this memo already exists")
				return err
			}
			log.Error("An error occurred while validating a fee transfer request", "err", err)
			return errors.New("Error")
		}
	}

	txn, err = s.stellarWallet.Sign(txn)
	if err != nil {
		return err
	}

	signatures := txn.Signatures()
	if len(signatures) != 1 {
		return fmt.Errorf("invalid number of signatures on the transaction")
	}

	response.Address = s.stellarWallet.GetAddress()
	response.Signature = base64.StdEncoding.EncodeToString(signatures[0].Signature)
	return nil
}

func (s *SignerService) validateWithdrawal(request multisig.StellarSignRequest, txn *txnbuild.Transaction) error {
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

		if acc.Address() == s.stellarWallet.Config.StellarFeeWallet {
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

		exists, err := s.stellarWallet.TransactionStorage.TransactionExists(txn)
		if err != nil {
			return errors.Wrap(err, "failed to check if transaction exists")
		}

		if exists {
			log.Info("Transaction with this hash already executed, skipping validating withdraw now..")
			return ErrTransactionAlreadyExists
		}
	}

	return nil
}

func (s *SignerService) validateRefundTransaction(request multisig.StellarSignRequest, txn *txnbuild.Transaction) error {
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
		if destinationAccount.Address() == s.stellarWallet.Config.StellarFeeWallet {
			if paymentOperation.Amount != xdr.Int64(WithdrawFee) {
				return fmt.Errorf("fee payment operation should be %d, but got %d", WithdrawFee, paymentOperation.Amount)
			}
			continue
		}

		//TODO: this does an horizon call, is this necessary?
		txnEffectsFromMessage, err := s.stellarWallet.GetTransactionEffects(request.Message)
		if err != nil {
			return fmt.Errorf("failed to retrieve transaction effects from message")
		}

		// Check if the source transaction actually was sent from the account that we are trying to credit
		// This way we can infer that it is indeed a refund transaction
		for _, effect := range txnEffectsFromMessage.Embedded.Records {
			if effect.GetType() == "account_debited" {
				if effect.GetAccount() == s.stellarWallet.Config.StellarFeeWallet {
					continue
				}

				if effect.GetAccount() != destinationAccount.Address() {
					return fmt.Errorf("destination is not correct, got %s, need user wallet %s", destinationAccount.Address(), effect.GetAccount())
				}
			}
		}

		// check if the actual transaction already happened or not
		exists, err := s.stellarWallet.TransactionStorage.TransactionExists(txn)
		if err != nil {
			return errors.Wrap(err, "failed to check if transaction exists")
		}

		if exists {
			log.Info("Transaction with this hash already executed, skipping validating refund now..")
			return ErrTransactionAlreadyExists
		}

		// check if the deposit for this fee transaction actually happened
		exists, err = s.stellarWallet.TransactionStorage.TransactionWithMemoExists(txn)
		if err != nil {
			return errors.Wrap(err, "failed to check transaction storage for existing transaction hash")
		}
		// if the transaction not exists, return with error
		if !exists {
			return stellar.ErrTransactionNotFound
		}
	}
	return nil
}

func (s *SignerService) validateFeeTransfer(request multisig.StellarSignRequest, txn *txnbuild.Transaction) error {

	for _, op := range txn.Operations() {
		opXDR, err := op.BuildXDR()
		if err != nil {
			return fmt.Errorf("failed to build operation xdr")
		}

		if opXDR.Body.Type != xdr.OperationTypePayment {
			return fmt.Errorf("transaction contains non payment operations")
		}

		paymentOperation, ok := opXDR.Body.GetPaymentOp()
		if !ok {
			return fmt.Errorf("transaction contains non payment operations")
		}

		acc := paymentOperation.Destination.ToAccountId()
		//TODO: should this be fetched through the wallet?
		if acc.Address() != s.stellarWallet.Config.StellarFeeWallet {
			return fmt.Errorf("destination is not correct, got %s, need fee wallet %s", acc.Address(), s.stellarWallet.Config.StellarFeeWallet)
		}

		// get the transaction hash
		// TODO: this does not make sense, ok, we do not need to sign but if the transaction already exists,
		//  it can never be submitted to the network anyway (sequence and such)
		exists, err := s.stellarWallet.TransactionStorage.TransactionExists(txn)
		if err != nil {
			return errors.Wrap(err, "failed to check if transaction exists")
		}

		if exists {
			log.Info("Transaction with this hash already executed, skipping validating fee transfer now..")
			return ErrTransactionAlreadyExists
		}

		// Check if a fee transfer for this already happened
		exists, err = s.stellarWallet.TransactionStorage.TransactionWithMemoExists(txn)
		if err != nil {
			return errors.Wrap(err, "failed to check transaction storage for transaction with memo")
		}
		if exists {
			return ErrTransactionAlreadyExists
		}

		// TODO: check if the deposit/withdraw for this fee transaction actually happened

		switch int64(paymentOperation.Amount) {
		case stellar.IntToStroops(s.depositFee):
			return nil
		case WithdrawFee:
			return nil
		default:
			return fmt.Errorf("amount is not correct, received %d, need %d or %d", paymentOperation.Amount, stellar.IntToStroops(s.depositFee), WithdrawFee)
		}

	}
	return nil
}
