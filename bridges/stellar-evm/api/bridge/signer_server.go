package bridge

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"

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
			if errors.Is(err, ErrInvalidTransaction) {
				log.Warn("Withdrawal validation error", "err", err)
				return err
			}
			log.Error("An error occurred while validating a withdrawal signing request", "err", err)
			return errors.New("Error") //Internal errors should not be exposed externally
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
		// then it's most likely a transfer to fee wallet transaction from a deposit
		log.Info("Validating fee transfer signing request")
		err := s.validateDepositFeeTransfer(request, txn)
		if err != nil {
			if errors.Is(err, ErrInvalidTransaction) {
				log.Info("Fee transfer validation error", "err", err)
				return err
			}
			log.Error("An error occurred while validating a deposit fee transfer signing request", "err", err)
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
	ethereumTransactionHash, _ := strings.CutPrefix(withdraw.Event.Raw.TxHash.Hex(), "0x")

	amount := withdraw.Event.Tokens.Int64()
	log.Info("validating withdrawal", "amount", stellar.StroopsToDecimal(amount), "receiver", withdraw.Event.BlockchainAddress, "tx", ethereumTransactionHash)
	memo, err := stellar.ExtractMemoFromTx(txn)
	if err != nil {
		log.Warn("Unable to extract the memo from the supplied transaction", "err", err)
		return errors.Wrap(ErrInvalidTransaction, "Unable to extract the memo from the supplied transaction")
	}
	if memo != ethereumTransactionHash {
		log.Warn("The supplied memo and the ethereum transaction do not match", "memo", memo, "tx", ethereumTransactionHash)
		return errors.Wrap(ErrInvalidTransaction, "The supplied memo and the ethereum transaction do not match")
	}
	withdrawalAlreadyExecuted, err := s.stellarWallet.TransactionStorage.TransactionWithMemoExists(memo)
	if err != nil {
		return err
	}
	if withdrawalAlreadyExecuted {
		return errors.Wrap(ErrInvalidTransaction, "Withdrawal already executed")
	}

	amount -= WithdrawFee
	if len(txn.Operations()) != 2 {
		return errors.Wrap(ErrInvalidTransaction, "a withdraw tx needs to contain 2 payment operations")
	}
	feePaymentPresent := false
	for _, op := range txn.Operations() {
		opXDR, err := op.BuildXDR()
		if err != nil {
			return errors.Wrap(ErrInvalidTransaction, "failed to build operation xdr")
		}

		if opXDR.Body.Type != xdr.OperationTypePayment {
			continue
		}

		paymentOperation, ok := opXDR.Body.GetPaymentOp()
		if !ok {
			return errors.Wrap(ErrInvalidTransaction, "transaction contains non payment operations")
		}

		acc := paymentOperation.Destination.ToAccountId()

		if acc.Address() == s.stellarWallet.Config.StellarFeeWallet {
			if int64(paymentOperation.Amount) != WithdrawFee {
				return errors.Wrap(ErrInvalidTransaction, "the withdraw fee is incorrect")
			}
			feePaymentPresent = true
			continue
		}

		if acc.Address() != withdraw.Event.BlockchainAddress {
			return errors.Wrapf(ErrInvalidTransaction, "destination is not correct, got %s, need %s", acc.Address(), withdraw.Event.BlockchainAddress)
		}

		if int64(paymentOperation.Amount) != amount {
			return fmt.Errorf("amount is not correct, received %d, need %d", paymentOperation.Amount, xdr.Int64(withdraw.Event.Tokens.Int64()))
		}
	}
	if !feePaymentPresent {
		return errors.Wrap(ErrInvalidTransaction, "No withdraw fee payment")
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
	// There are 2 payment operations, 1 to the feewallet and 1 to the account that made the deposit
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

func (s *SignerService) validateDepositFeeTransfer(request multisig.StellarSignRequest, txn *txnbuild.Transaction) (err error) {

	// Check if a fee transfer for this already happened
	memo, err := stellar.ExtractMemoFromTx(txn)
	if err != nil {
		log.Warn("Failed to extract memo", "err", err)
		return ErrInvalidTransaction
	}
	alreadyExists, err := s.stellarWallet.TransactionStorage.TransactionWithMemoExists(memo)
	if err != nil {
		return
	}
	if alreadyExists {
		return ErrTransactionAlreadyExists
	}

	//A Fee transfer only has 1 operation
	if len(txn.Operations()) != 1 {
		return errors.Wrap(ErrInvalidTransaction, "The transaction should have exactly 1 operation")
	}

	op := txn.Operations()[0]
	opXDR, err := op.BuildXDR()
	if err != nil {
		return errors.Wrap(ErrInvalidTransaction, "failed to build operation xdr")
	}

	if opXDR.Body.Type != xdr.OperationTypePayment {
		return errors.Wrap(ErrInvalidTransaction, "transaction contains non payment operations")
	}

	paymentOperation, ok := opXDR.Body.GetPaymentOp()
	if !ok {
		return errors.Wrap(ErrInvalidTransaction, "transaction contains non payment operations")
	}

	acc := paymentOperation.Destination.ToAccountId()
	//TODO: should this be fetched through the wallet?
	if acc.Address() != s.stellarWallet.Config.StellarFeeWallet {
		return errors.Wrapf(ErrInvalidTransaction, "destination is not correct, got %s, need fee wallet %s", acc.Address(), s.stellarWallet.Config.StellarFeeWallet)
	}

	if int64(paymentOperation.Amount) != stellar.IntToStroops(s.depositFee) {
		return errors.Wrapf(ErrInvalidTransaction, "amount is not correct, received %d, need %d", stellar.StroopsToDecimal(int64(paymentOperation.Amount)), s.depositFee)
	}
	//Validate the deposit transaction that triggered this deposit fee transfer
	depositedAmount, _, err := s.stellarWallet.GetDepositAmountAndSender(memo, s.bridgeMasterAddress)
	if err != nil {
		return
	}
	if depositedAmount <= s.depositFee {
		return errors.Wrap(ErrInvalidFeePayment, "The amount of the deposit is smaller than the deposit fee")
	}
	return
}
