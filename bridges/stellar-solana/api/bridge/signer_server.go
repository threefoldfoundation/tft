package bridge

import (
	"context"
	"encoding/base64"
	"fmt"

	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	"github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/multisig"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/solana"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/stellar"
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

type SolanaRequest struct {
	Receiver           solana.Address
	Amount             int64
	TxID               string
	RequiredSignatures int64
	// Tx is the base64 encoded solana transaction
	Tx string
}

type SolanaResponse struct {
	Who       solana.Address
	Signature solana.Signature
	SigIdx    int
}

type SignerService struct {
	solWallet           *solana.Solana
	stellarWallet       *stellar.Wallet
	bridgeMasterAddress string
	depositFee          int64 // deposit fee in TFT units TODO: maybe just pass part of the config
}

func NewSignerServer(host host.Host, bridgeMasterAddress string, solanaWallet *solana.Solana, stellarWallet *stellar.Wallet, depositFee int64) error {
	log.Info().Str("identity", host.ID().String()).Msg("server started")
	partialMA, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", host.ID()))
	if err != nil {
		return err
	}

	for _, addr := range host.Addrs() {
		full := addr.Encapsulate(partialMA)
		log.Info().Str("address", full.String()).Msg("p2p node address")
	}

	server := gorpc.NewServer(host, Protocol)

	signerService := SignerService{
		solWallet:           solanaWallet,
		stellarWallet:       stellarWallet,
		bridgeMasterAddress: bridgeMasterAddress,
		depositFee:          depositFee,
	}

	return server.Register(&signerService)
}

func (s *SignerService) SignMint(ctx context.Context, request SolanaRequest, response *SolanaResponse) error {
	log.Info().Str("request txid", request.TxID).Msg("sign mint request")

	solTx := new(solana.Transaction)
	err := solTx.UnmarshalBase64(request.Tx)
	if err != nil {
		log.Warn().Str("txid", request.TxID).Msg("could not unmarshal transaction")
		return err
	}

	amount, memo, receiver, err := solana.ExtractMintvalues(*solTx)
	if memo != request.TxID {
		log.Warn().Str("requested txid", request.TxID).Str("embedded txid", memo).Msg("could not unmarshal transaction")
		return errors.New("mismatched embedded transaction ID")
	}
	if err != nil {
		log.Warn().Str("txid", request.TxID).Msg("could not unmarshal transaction")
		return err
	}

	if receiver != request.Receiver {
		log.Warn().Str("txid", request.TxID).Str("tx receiver", receiver.String()).Str("requested receiver", request.Receiver.String()).Msg("Receiver does not match")
		return errors.New("Tx receiver does not match requested receiver")
	}

	// Check in transaction storage if the deposit transaction exists
	tx, err := s.stellarWallet.TransactionStorage.GetTransactionWithID(ctx, request.TxID)
	if err != nil {
		log.Info().Str("txid", request.TxID).Msg("transaction not found")
		return err
	}

	// Validate amount
	depositedAmount, _, err := s.stellarWallet.GetDepositAmountAndSender(request.TxID, s.bridgeMasterAddress)
	if err != nil {
		return err
	}
	depositFeeBigInt := stellar.IntToStroops(s.depositFee)
	// Subtract fee from deposit amount
	depositedAmount -= depositFeeBigInt

	log.Debug().Int64("embedded amount", amount).Int64("amount", depositedAmount).Int64("request amount", request.Amount).Msg("validating amount for sign tx")

	if amount != request.Amount || amount != depositedAmount {
		log.Warn().Int64("request amount", request.Amount).Int64("embedded amount", amount).Int64("deposit amount", depositedAmount).Msg("tx amounts don't match")
		return fmt.Errorf("amounts do not match")
	}

	log.Debug().Str("memoType", tx.MemoType).Str("memo", tx.Memo).Msg("tx memo")
	// Validate address
	if tx.MemoType != "hash" {
		return errors.New("memo is not of type memo hash")
	}
	addr, err := solana.AddressFromB64(tx.Memo)
	if err != nil {
		return err
	}

	if addr != request.Receiver {
		return fmt.Errorf("deposit addresses do not match")
	}

	known, err := s.solWallet.IsMintTxID(ctx, tx.ID)
	if err != nil {
		return errors.Wrap(err, "Could not verify if we already know this mint")
	}

	if known {
		return errors.New("Refusing to sign mint request for transaction we already minted")
	}

	signature, idx, err := s.solWallet.CreateTokenSignature(*solTx)
	if err != nil {
		return err
	}

	response.Signature = signature
	response.SigIdx = idx
	response.Who = s.solWallet.Address()

	return nil
}

// Sign signs a stellar sign request
// This is calable on the libp2p network with RPC
func (s *SignerService) Sign(ctx context.Context, request multisig.StellarSignRequest, response *multisig.StellarSignResponse) error {
	log.Info().Msg("got signing request")

	loaded, err := txnbuild.TransactionFromXDR(request.TxnXDR)
	if err != nil {
		return err
	}

	txn, ok := loaded.Transaction()
	if !ok {
		return fmt.Errorf("provided transaction is of wrong type")
	}

	var emptyAddr solana.Address
	if request.Receiver != emptyAddr {
		log.Info().Msg("Validating withdrawal signing request")
		err = s.validateWithdrawal(ctx, request, txn)
		if err != nil {
			if errors.Is(err, ErrInvalidTransaction) {
				log.Warn().Err(err).Msg("Withdrawal validation error")
				return err
			}
			log.Error().Err(err).Msg("An error occurred while validating a withdrawal signing request")
			return errors.New("Error") // Internal errors should not be exposed externally
		}
	} else if request.Message != "" {
		// If the signrequest has a message attached we know it's a refund transaction
		log.Info().Str("deposit", request.Message).Msg("Validating refund signing request")
		err = s.validateRefundTransaction(ctx, request, txn)
		if err != nil {
			if errors.Is(err, ErrInvalidTransaction) {
				log.Warn().Err(err).Msg("Refund validation error")
				return err
			}
			log.Error().Err(err).Msg("An error occurred while validating a refund signing request")
			return errors.New("Error") // Internal errors should not be exposed externally
		}
	} else {
		// If the signrequest is not a withdrawal request and a refund request
		// then it's most likely a transfer to fee wallet transaction from a deposit
		log.Info().Msg("Validating fee transfer signing request")
		err = s.validateDepositFeeTransfer(ctx, request, txn)
		if err != nil {
			if errors.Is(err, ErrInvalidTransaction) {
				log.Info().Err(err).Msg("Fee transfer validation error")
				return err
			}
			log.Error().Err(err).Msg("An error occurred while validating a deposit fee transfer signing request")
			return errors.New("Error") // Internal errors should not be exposed externally
		}
	}

	log.Info().Msg("Signing valid signing request")
	txn, err = s.stellarWallet.Sign(txn)
	if err != nil {
		return err
	}

	signatures := txn.Signatures()
	if len(signatures) != 1 {
		log.Info().Msg("invalid number of signatures on the transaction")
		return fmt.Errorf("invalid number of signatures on the transaction")
	}

	response.Address = s.stellarWallet.GetAddress()
	response.Signature = base64.StdEncoding.EncodeToString(signatures[0].Signature)
	return nil
}

// validates a withdrawal (burn on solana)
func (s *SignerService) validateWithdrawal(ctx context.Context, request multisig.StellarSignRequest, txn *txnbuild.Transaction) error {
	shortTxIDHash, err := stellar.ExtractTxHashMemoFromTx(txn)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to extract the memo from the supplied transaction")
		return errors.Wrap(ErrInvalidTransaction, "Unable to extract the memo from the supplied transaction")
	}
	shortTxID := solana.NewShortTxID(shortTxIDHash)
	withdraw, err := s.solWallet.GetBurnTransaction(ctx, shortTxID)
	if err != nil {
		return err
	}

	amount := int64(withdraw.RawAmount())
	receiver := withdraw.Memo()
	log.Info().Str("amount", stellar.StroopsToDecimal(amount).String()).Str("receiver", receiver).Str("tx", withdraw.TxID().String()).Msg("validating withdrawal")
	withdrawalAlreadyExecuted, err := s.stellarWallet.TransactionStorage.TransactionWithShortTxIDExists(ctx, shortTxID)
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

		if acc.Address() != receiver {
			return errors.Wrapf(ErrInvalidTransaction, "destination is not correct, got %s, need %s", acc.Address(), receiver)
		}

		if int64(paymentOperation.Amount) != amount {
			return fmt.Errorf("amount is not correct, received %d, need %d", paymentOperation.Amount, xdr.Int64(amount))
		}
	}
	if !feePaymentPresent {
		return errors.Wrap(ErrInvalidTransaction, "No withdraw fee payment")
	}

	return nil
}

func (s *SignerService) validateRefundTransaction(ctx context.Context, request multisig.StellarSignRequest, txn *txnbuild.Transaction) error {
	// check if a refund already happened
	memo, err := stellar.ExtractMemoFromTx(txn)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to extract memo")
		return ErrInvalidTransaction
	}
	if memo != request.Message {
		return errors.Wrap(ErrInvalidTransaction, "The transaction memo and the signrequest message do not match")
	}
	alreadyRefunded, err := s.stellarWallet.TransactionStorage.TransactionWithMemoExists(ctx, memo)
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

	// TODO: this does an horizon call, is this necessary? The transactions are also present in the Transactionstorage
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
	}

	if stellar.DecimalToStroops(depositedAmount) != (refundAmountWithoutPenalty + WithdrawFee) {
		return errors.Wrapf(ErrInvalidTransaction, "The refunded amount %s does not match the deposit %s minus the penalty", stellar.StroopsToDecimal(refundAmountWithoutPenalty), depositedAmount)
	}

	return nil
}

func (s *SignerService) validateDepositFeeTransfer(ctx context.Context, request multisig.StellarSignRequest, txn *txnbuild.Transaction) (err error) {
	// Check if a fee transfer for this already happened
	memo, err := stellar.ExtractMemoFromTx(txn)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to extract memo")
		return ErrInvalidTransaction
	}
	alreadyExists, err := s.stellarWallet.TransactionStorage.TransactionWithMemoExists(ctx, memo)
	if err != nil {
		return
	}
	if alreadyExists {
		return ErrTransactionAlreadyExists
	}

	// A Fee transfer only has 1 operation
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
	// TODO: should this be fetched through the wallet?
	if acc.Address() != s.stellarWallet.Config.StellarFeeWallet {
		return errors.Wrapf(ErrInvalidTransaction, "destination is not correct, got %s, need fee wallet %s", acc.Address(), s.stellarWallet.Config.StellarFeeWallet)
	}

	if int64(paymentOperation.Amount) != stellar.IntToStroops(s.depositFee) {
		return errors.Wrapf(ErrInvalidTransaction, "amount is not correct, received %d, need %d", stellar.StroopsToDecimal(int64(paymentOperation.Amount)), s.depositFee)
	}
	// Validate the deposit transaction that triggered this deposit fee transfer
	depositedAmount, _, err := s.stellarWallet.GetDepositAmountAndSender(memo, s.bridgeMasterAddress)
	if err != nil {
		return
	}
	if depositedAmount <= s.depositFee {
		return errors.Wrap(ErrInvalidFeePayment, "The amount of the deposit is smaller than the deposit fee")
	}
	return
}
