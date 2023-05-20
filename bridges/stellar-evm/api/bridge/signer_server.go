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
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/contracts/tokenv1"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/eth"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/multisig"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/stellar"
)

const (
	Protocol = protocol.ID("/p2p/rpc/signer")
)

var (
	ErrInvalidTransaction       = errors.New("Invalid transaction")
	ErrTransactionAlreadyExists = errors.Wrap(ErrInvalidTransaction, "transaction already exists")
	ErrAlreadyRefunded          = errors.Wrap(ErrInvalidTransaction, "The deposit was already refunded")
	ErrInvalidFeePayment        = errors.Wrap(ErrInvalidTransaction, "Invalid fee payment")
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
	depositedAmount, _, err := s.stellarWallet.GetDepositAmountAndSender(request.TxId, s.bridgeMasterAddress)
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
		log.Info("Validating withdrawal signing request")
		err := s.validateWithdrawal(request, txn)
		if err != nil {
			log.Error("An error occurred while validating a withdrawal signing request", "err", err)
			return err
		}
	} else if request.Message != "" {
		// If the signrequest has a message attached we know it's a refund transaction
		log.Info("Validating refund signing request", "deposit", request.Message)
		err := s.validateRefundTransaction(request, txn)
		if err != nil {
			if errors.Is(err, ErrInvalidTransaction) {
				log.Warn("Refund validation error", "err", err)
				return err
			}
			log.Error("An error occurred while validating a refund signing request", "err", err)
			return errors.New("Error") //Internal errors should not be exposed externally
		}
	} else {
		// If the signrequest is not a withdrawal request and a refund request
		// then it's most likely a transfer to fee wallet transaction
		log.Info("Validating fee transfer signing request")
		err := s.validateFeeTransfer(request, txn)
		if err != nil {
			if errors.Is(err, ErrInvalidTransaction) {
				log.Info("Fee transfer validation error", "err", err)
				return err
			}
			log.Error("An error occurred while validating a fee transfer signing request", "err", err)
			return errors.New("Error") //Internal errors should not be exposed externally
		}
	}

	log.Info("Signing valid signing request")
	txn, err = s.stellarWallet.Sign(txn)
	if err != nil {
		return err
	}

	signatures := txn.Signatures()
	if len(signatures) != 1 {
		log.Info("invalid number of signatures on the transaction")
		return fmt.Errorf("invalid number of signatures on the transaction")
	}

	response.Address = s.stellarWallet.GetAddress()
	response.Signature = base64.StdEncoding.EncodeToString(signatures[0].Signature)
	return nil
}

func (s *SignerService) validateWithdrawal(request multisig.StellarSignRequest, txn *txnbuild.Transaction) error {
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

	// check if a refund already happened
	memo, err := stellar.ExtractMemoFromTx(txn)
	if err != nil {
		log.Warn("Failed to extract memo", "err", err)
		return ErrInvalidTransaction
	}
	if memo != request.Message {
		return errors.Wrap(ErrInvalidTransaction, "The transaction memo and the signrequest message do not match")
	}
	alreadyRefunded, err := s.stellarWallet.TransactionStorage.TransactionWithMemoExists(memo)
	if err != nil {
		return err
	}
	if alreadyRefunded {
		return ErrAlreadyRefunded
	}

	var destinationAccount string
	var refundAmountWithoutPenalty int64
	var penaltyPayment bool
	// In case the deposited amount==1, there is only a payment to the feewallet
	// If it is bigger, there are 2 payment operations, 1 to the feewallet and 1 to the account that made the deposit
	if len(txn.Operations()) > 2 {
		return errors.Wrap(ErrInvalidTransaction, "The refund transaction has too many operations")
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
			return fmt.Errorf("failed to get payment operation")
		}

		operationDestinationAccount := paymentOperation.Destination.Address()

		if operationDestinationAccount == s.stellarWallet.Config.StellarFeeWallet {
			if penaltyPayment {
				return errors.Wrap(ErrInvalidTransaction, "Multiple payments to the feewallet")
			}
			penaltyPayment = true
			if paymentOperation.Amount != xdr.Int64(WithdrawFee) {
				return errors.Wrapf(ErrInvalidFeePayment, "fee amount should be %d, but got %d", WithdrawFee, paymentOperation.Amount)
			}
			continue
		}
		destinationAccount = operationDestinationAccount
		refundAmountWithoutPenalty = int64(paymentOperation.Amount)
	}

	//TODO: this does an horizon call, is this necessary? The transactions are also present in the Transactionstorage
	txnEffectsFromMessage, err := s.stellarWallet.GetTransactionEffects(memo)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve transaction effects of the transaction a refund is requested for")
	}

	// Check if the deposit was sent from the account that we are trying to credit
	//  and if the refund amount is correct
	var depositedAmount decimal.Decimal
	for _, effect := range txnEffectsFromMessage.Embedded.Records {
		if effect.GetType() == effects.EffectTypeNames[effects.EffectAccountDebited] {
			if effect.GetAccount() != destinationAccount {
				return errors.Wrapf(ErrInvalidTransaction, "destination is not correct, got %s, original account debited is %s", destinationAccount, effect.GetAccount())
			}
		}
		if effect.GetType() == effects.EffectTypeNames[effects.EffectAccountCredited] {
			if effect.GetAccount() == s.bridgeMasterAddress {
				bridgeDepositEffect, ok := effect.(effects.AccountCredited)
				if !ok {
					return errors.New("Unable to convert an AccountCredited effect to its real type")
				}
				depositedAmount, err = decimal.NewFromString(bridgeDepositEffect.Amount)
				if err != nil {
					return err
				}
			}
		}

		if stellar.DecimalToStroops(depositedAmount) != (refundAmountWithoutPenalty + WithdrawFee) {
			return errors.Wrap(ErrInvalidTransaction, "The refunded amount does not match the deposit")
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
		memo, err := stellar.ExtractMemoFromTx(txn)
		if err != nil {
			log.Warn("Failed to extract memo", "err", err)
			return ErrInvalidTransaction
		}
		alreadyExists, err := s.stellarWallet.TransactionStorage.TransactionWithMemoExists(memo)
		if err != nil {
			return err
		}
		if alreadyExists {
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
