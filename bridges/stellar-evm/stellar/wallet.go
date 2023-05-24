package stellar

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/eth"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/multisig"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/state"

	"github.com/threefoldfoundation/tft/bridges/stellar-evm/faults"

	"github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
)

// Wallet is the bridge wallet
// Payments will be funded and fees will be taken with this wallet
type Wallet struct {
	keypair            *keypair.Full
	Config             *StellarConfig //TODO: should this be public?
	TransactionStorage *TransactionStorage
	depositFee         int64
	withdrawFee        int64
	signerWallet
}
type signersClient interface {
	Sign(ctx context.Context, signRequest multisig.StellarSignRequest) ([]multisig.StellarSignResponse, error)
}
type signerWallet struct {
	client         signersClient
	signatureCount int
}

func NewWallet(config *StellarConfig, depositFee int64, withdrawFee int64, stellarTransactionStorage *TransactionStorage) (*Wallet, error) {
	kp, err := keypair.ParseFull(config.StellarSeed)

	if err != nil {
		return nil, err
	}

	w := &Wallet{
		keypair:            kp,
		Config:             config,
		TransactionStorage: stellarTransactionStorage,
		depositFee:         depositFee,
		withdrawFee:        withdrawFee,
	}

	return w, nil
}

func (w *Wallet) GetAddress() string {
	return w.keypair.Address()
}

func (w *Wallet) GetSigningRequirements() (cosigners []string, requiredSignatures int, err error) {
	account, err := w.getAccountDetails()
	if err != nil {
		return
	}
	cosigners = make([]string, 0, len(account.Signers))
	for _, signer := range account.Signers {
		if signer.Key == w.GetAddress() {
			continue
		}
		cosigners = append(cosigners, signer.Key)
	}
	requiredSignatures = int(account.Thresholds.MedThreshold)

	return
}
func (w *Wallet) SetRequiredSignatures(requiredSignatures int) {
	w.signatureCount = requiredSignatures - 1
}
func (w *Wallet) SetSignerClient(client signersClient) {

	w.client = client
}

// Sign returns a new Transaction instance which extends the current instance
// with a signature from this wallet.
func (w *Wallet) Sign(tx *txnbuild.Transaction) (*txnbuild.Transaction, error) {
	return tx.Sign(w.GetNetworkPassPhrase(), w.keypair)
}

func (w *Wallet) CreateAndSubmitPayment(ctx context.Context, target string, amount uint64, receiver common.Address, blockheight uint64, txHash common.Hash, message string, includeWithdrawFee bool) (err error) {
	if !IsValidStellarAddress(target) {
		log.Warn("Invalid address, skipping payment", "address", target)
		return
	}
	txnBuild, err := w.generatePaymentOperation(amount, target, includeWithdrawFee)
	if err != nil {
		return
	}

	txnBuild.Memo = txnbuild.MemoHash(txHash)

	signReq := multisig.StellarSignRequest{
		RequiredSignatures: w.signatureCount,
		Receiver:           receiver,
		Block:              blockheight,
		Message:            message,
	}

	return w.signAndSubmitTransaction(ctx, txnBuild, signReq)
}

// CreateAndSubmitRefund refunds a deposit for the transaction txToRefund ( hexadecimal representation of the transaction hash)
func (w *Wallet) CreateAndSubmitRefund(ctx context.Context, target string, amount uint64, txToRefund string, includeWithdrawFee bool) (err error) {
	txnBuild, err := w.generatePaymentOperation(amount, target, includeWithdrawFee)
	if err != nil {
		return
	}

	txToRefundAsBytes, err := hex.DecodeString(txToRefund)
	if err != nil {
		return
	}
	if len(txToRefundAsBytes) != 32 {
		return errors.New("A stellar transaction hash should be 32 bytes")
	}

	txnBuild.Memo = txnbuild.MemoReturn([32]byte(txToRefundAsBytes))

	signReq := multisig.StellarSignRequest{
		RequiredSignatures: w.signatureCount,
		Message:            txToRefund,
	}

	return w.signAndSubmitTransaction(ctx, txnBuild, signReq)
}

// CreateAndSubmitFeepayment creates and submites a payment to the fee wallet
// only an amount and hash needs to be specified
func (w *Wallet) CreateAndSubmitFeepayment(ctx context.Context, amount uint64, txHash [32]byte) error {

	txnBuild, err := w.generatePaymentOperation(amount, w.Config.StellarFeeWallet, false)
	if err != nil {
		return errors.Wrap(err, "failed to generate payment operation")
	}

	txnBuild.Memo = txnbuild.MemoHash(txHash)

	signReq := multisig.StellarSignRequest{
		RequiredSignatures: w.signatureCount,
	}

	return w.signAndSubmitTransaction(ctx, txnBuild, signReq)
}

func (w *Wallet) generatePaymentOperation(amount uint64, destination string, includeWithdrawFee bool) (txnbuild.TransactionParams, error) {
	// if amount is zero, do nothing
	if amount == 0 {
		return txnbuild.TransactionParams{}, errors.New("invalid amount")
	}

	sourceAccount, err := w.getAccountDetails()
	if err != nil {
		return txnbuild.TransactionParams{}, errors.Wrap(err, "failed to get source account")
	}

	assetCode, issuer := w.GetAssetCodeAndIssuer()

	var paymentOperations []txnbuild.Operation
	paymentOP := txnbuild.Payment{
		Destination: destination,
		Amount:      big.NewRat(int64(amount), Precision).FloatString(PrecisionDigits),
		Asset: txnbuild.CreditAsset{
			Code:   assetCode,
			Issuer: issuer,
		},
		SourceAccount: sourceAccount.AccountID,
	}
	paymentOperations = append(paymentOperations, &paymentOP)

	if includeWithdrawFee {
		feePaymentOP := txnbuild.Payment{
			Destination: w.Config.StellarFeeWallet,
			Amount:      big.NewRat(w.withdrawFee, Precision).FloatString(PrecisionDigits),
			Asset: txnbuild.CreditAsset{
				Code:   assetCode,
				Issuer: issuer,
			},
			SourceAccount: sourceAccount.AccountID,
		}
		paymentOperations = append(paymentOperations, &feePaymentOP)
	}

	txnBuild := txnbuild.TransactionParams{
		Operations:           paymentOperations,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		SourceAccount:        &sourceAccount,
		BaseFee:              Precision,
		IncrementSequenceNum: true,
	}

	return txnBuild, nil
}

// signAndSubmitTransaction gathers signatures from cosigners if required and submits the transaction to the Stellar network
// If there already is a transaction with the same memo hash, no new transaction is created and submitted.
func (w *Wallet) signAndSubmitTransaction(ctx context.Context, txn txnbuild.TransactionParams, signReq multisig.StellarSignRequest) (err error) {
	tx, err := txnbuild.NewTransaction(txn)
	if err != nil {
		return errors.Wrap(err, "failed to build transaction")
	}

	// check if the actual transaction to be submitted already happened on the stellar network
	memo, err := ExtractMemoFromTx(tx)
	if err != nil {
		log.Error("Failed to extract memo", "err", err)
		return err
	}
	exists, err := w.TransactionStorage.TransactionWithMemoExists(memo)
	if err != nil {
		return errors.Wrapf(err, "failed to check if transaction exists with memo %s", memo)
	}

	if exists {
		log.Info("Transaction with this memo already executed, skipping")
		return
	}

	// Only try to request signatures if there are signatures required
	if w.signatureCount > 0 {
		xdr, err := tx.Base64()
		if err != nil {
			return errors.Wrap(err, "failed to serialize transaction")
		}
		signReq.TxnXDR = xdr

		signatures, err := w.client.Sign(ctx, signReq)
		if err != nil {
			return err
		}

		if len(signatures) < w.signatureCount {
			return fmt.Errorf("received %d signatures, need %d", len(signatures), w.signatureCount)
		}

		for _, signature := range signatures {
			tx, err = tx.AddSignatureBase64(w.GetNetworkPassPhrase(), signature.Address, signature.Signature)
			if err != nil {
				log.Error("Failed to add signature", "err", err.Error())
				return err
			}
		}
	}

	tx, err = tx.Sign(w.GetNetworkPassPhrase(), w.keypair)
	if err != nil {
		log.Error("Failed to sign transaction", "error", err)
		return errors.Wrap(err, "failed to sign transaction with keypair")
	}

	// Submit the transaction

	client, err := w.GetHorizonClient()
	if err != nil {
		return errors.Wrap(err, "failed to get horizon client")
	}
	txResult, err := client.SubmitTransaction(tx)
	if err != nil {
		if hError, ok := err.(*horizonclient.Error); ok {
			resultcodes, err := hError.ResultCodes()
			if err != nil {
				log.Error("Unable to extract result codes from horizon error")
			} else {

				for _, resultcode := range resultcodes.OperationCodes {
					if resultcode == "op_no_destination" {
						log.Warn("Invalid address, skipping")
						return nil
					}
					if resultcode == "op_no_trust" {
						log.Warn("Destination address has no TFT trustline, skipping")
						return nil
					}
				}
			}
			log.Error("Error submitting tx", "extras", hError.Problem.Extras)
		}
		return errors.Wrap(err, "error submitting transaction")
	}
	log.Info(fmt.Sprintf("transaction: %s submitted to the stellar network..", txResult.Hash))

	// Store the transaction in the database
	w.TransactionStorage.StoreTransaction(txResult)

	return
}

// sender is the account that made the deposit
func (w *Wallet) refundDeposit(ctx context.Context, totalAmount uint64, sender string, tx hProtocol.Transaction) {
	if totalAmount <= uint64(w.withdrawFee) {
		log.Warn("Deposited amount is less than the withdraw fee, not refunding", "tx", tx.Hash)
		return
	}
	amount := totalAmount - uint64(w.withdrawFee)
	log.Info("Calling refund")

	err := w.CreateAndSubmitRefund(ctx, sender, amount, tx.Hash, true)
	for err != nil {
		log.Error("error while refunding", "err", err.Error(), "amount", StroopsToDecimal(int64(totalAmount)))
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
			err = w.CreateAndSubmitRefund(ctx, tx.Account, amount, tx.Hash, true)
		}
	}

}

// mint handler
type mint func(eth.ERC20Address, *big.Int, string) error

// MonitorBridgeAccountAndMint is a blocking function that keeps monitoring
// the bridge account on the Stellar network for new transactions and calls the
// mint function when a deposit is made
func (w *Wallet) MonitorBridgeAccountAndMint(ctx context.Context, mintFn mint, persistency *state.ChainPersistency) error {
	transactionHandler := func(tx hProtocol.Transaction) {
		if !tx.Successful {
			return
		}
		log.Info("Received transaction on bridge stellar account", "hash", tx.Hash)

		//TODO: this does an horizon call while we have the transaction here
		totalAmount, sender, err := w.GetDepositAmountAndSender(tx.Hash, w.GetAddress())
		if err != nil || totalAmount == 0 {
			return
		}

		if totalAmount <= IntToStroops(w.depositFee) {
			log.Warn("Deposited amount is less than the depositfee, refunding")
			w.refundDeposit(ctx, uint64(totalAmount), sender, tx)
			return
		}

		log.Info("deposited amount", "a", StroopsToDecimal(totalAmount))
		depositedAmount := big.NewInt(totalAmount)
		log.Info("memo", "m", tx.Memo)

		ethAddress, err := eth.GetErc20AddressFromB64(tx.Memo)
		if err != nil {
			log.Warn("error converting transaction memo to an Ethereum address, refunding", "error", err.Error())
			w.refundDeposit(ctx, uint64(totalAmount), sender, tx)
			return
		}

		err = mintFn(ethAddress, depositedAmount, tx.Hash)
		for err != nil {
			log.Error(fmt.Sprintf("Error occured while minting: %s", err.Error()))
			//TODO: we already checked this above
			if err == faults.ErrInsufficientDepositAmount {
				log.Warn("User is trying to swap less than the fee amount, refunding", "amount", totalAmount)
				w.refundDeposit(ctx, uint64(totalAmount), sender, tx)
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				err = mintFn(ethAddress, depositedAmount, tx.Hash)
			}
		}

		log.Info("Transferring the fee to the fee wallet", "address", w.Config.StellarFeeWallet)

		// convert tx hash string to bytes
		parsedMessage, err := hex.DecodeString(tx.Hash)
		if err != nil {
			log.Error("Error hex decoding transaction hash", "err", err)
			return
		}
		var memo [32]byte
		copy(memo[:], parsedMessage)

		//TODO: a context is there for a reason
		err = w.CreateAndSubmitFeepayment(context.Background(), uint64(IntToStroops(w.depositFee)), memo)
		for err != nil {
			log.Error("error sending fee to the fee wallet", "err", err.Error())
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				err = w.CreateAndSubmitFeepayment(context.Background(), uint64(IntToStroops(w.depositFee)), memo)
			}
		}

		log.Info("Mint succesfull, saving cursor now")

		// save cursor
		cursor := tx.PagingToken()
		err = persistency.SaveStellarCursor(cursor)
		if err != nil {
			log.Error("error while saving cursor:", err.Error())
			return
		}

	}

	// get saved cursor
	blockHeight, err := persistency.GetHeight()
	for err != nil {
		log.Warn("Error getting the bridge persistency", "error", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
			blockHeight, err = persistency.GetHeight()
		}
	}

	return w.StreamBridgeStellarTransactions(ctx, blockHeight.StellarCursor, transactionHandler)
}

// TODO: is this called from a place where we really only have the transaction hash
// instead of the entire transaction
// If the entire transaction is available, there is no need to call horizon
func (w *Wallet) GetDepositAmountAndSender(txHash string, bridgeAccount string) (depositedAmount int64, sender string, err error) {
	transactionEffects, err := w.GetTransactionEffects(txHash)
	if err != nil {
		log.Error("error while fetching transaction effects:", err.Error())
		return
	}
	assetCode, issuer := w.GetAssetCodeAndIssuer()

	for _, effect := range transactionEffects.Embedded.Records {

		if effect.GetType() == effects.EffectTypeNames[effects.EffectAccountDebited] {
			// Only TFT payments matter, Assume normal payments

			debitedEffect := effect.(effects.AccountDebited)
			if debitedEffect.Asset.Code != assetCode && debitedEffect.Asset.Issuer != issuer {
				continue
			}
			//Normally a payment to the feebump service and the deposit payment are done by the same account
			sender = effect.GetAccount()
		}
		if effect.GetType() == effects.EffectTypeNames[effects.EffectAccountCredited] {

			if effect.GetAccount() != bridgeAccount {
				// only payments to the bridgeaccount matter
				continue
			}
			creditedEffect := effect.(effects.AccountCredited)
			if creditedEffect.Asset.Code != assetCode && creditedEffect.Asset.Issuer != issuer {
				continue
			}
			parsedAmount, err := amount.ParseInt64(creditedEffect.Amount)
			if err != nil {
				continue
			}

			depositedAmount += parsedAmount
		}
	}

	return
}

// getAccountDetails gets theaccount details of the account being scanned
func (w *Wallet) getAccountDetails() (account hProtocol.Account, err error) {
	client, err := w.GetHorizonClient()
	if err != nil {
		return hProtocol.Account{}, err
	}
	ar := horizonclient.AccountRequest{AccountID: w.GetAddress()}
	account, err = client.AccountDetail(ar)
	if err != nil {
		return hProtocol.Account{}, errors.Wrapf(err, "failed to get account details for account: %s", w.GetAddress())
	}
	return account, nil
}

func (w *Wallet) StreamBridgeStellarTransactions(ctx context.Context, cursor string, handler func(op hProtocol.Transaction)) (err error) {
	client, err := w.GetHorizonClient()
	if err != nil {
		return
	}

	log.Info("Start watching stellar account transactions", "horizon", client.HorizonURL, "account", w.keypair.Address(), "cursor", cursor)

	for {
		if ctx.Err() != nil {
			return
		}

		internalHandler := func(tx hProtocol.Transaction) {
			handler(tx)
			cursor = tx.PagingToken()
		}
		err = fetchTransactions(ctx, client, w.GetAddress(), cursor, internalHandler)
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}

	}

}

func (w *Wallet) ScanBridgeAccount() error {
	return w.TransactionStorage.ScanBridgeAccount()
}

// TODO: is this function really needed?
// It does an horizon call while the place where this is called from might have the entire transaction present
func (w *Wallet) GetTransactionEffects(txHash string) (transactionEffects effects.EffectsPage, err error) {
	client, err := w.GetHorizonClient()
	if err != nil {
		return
	}

	effectsReq := horizonclient.EffectRequest{
		ForTransaction: txHash,
	}
	transactionEffects, err = client.Effects(effectsReq)
	return
}

// GetHorizonClient gets the horizon client based on the wallet's network
func (w *Wallet) GetHorizonClient() (*horizonclient.Client, error) {
	return GetHorizonClient(w.Config.StellarNetwork)
}

// GetNetworkPassPhrase gets the Stellar network passphrase based on the wallet's network
func (w *Wallet) GetNetworkPassPhrase() string {
	return GetNetworkPassPhrase(w.Config.StellarNetwork)
}

func (w *Wallet) GetAssetCodeAndIssuer() (assetCode, issuer string) {
	var assetCodeAndIssuerAsSlice []string
	if w.Config.StellarNetwork == "production" {
		assetCodeAndIssuerAsSlice = strings.Split(TFTMainnet, ":")
	} else {
		assetCodeAndIssuerAsSlice = strings.Split(TFTTest, ":")
	}
	return assetCodeAndIssuerAsSlice[0], assetCodeAndIssuerAsSlice[1]
}
