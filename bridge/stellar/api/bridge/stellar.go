package bridge

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
	horizoneffects "github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
)

const (
	TFTMainnet = "TFT:GBOVQKJYHXRR3DX6NOX2RRYFRCUMSADGDESTDNBDS6CDVLGVESRTAC47"
	TFTTest    = "TFT:GA47YZA3PKFUZMPLQ3B5F2E3CJIB57TGGU7SPCQT2WAEYKN766PWIMB3"
	// TFTTest = "TFTXXX:GCRO7FLIU4LKELZBLGWOTQ7T64OKSU4O4OWATLHV3BFSVQZMJWWKRE5A"

	stellarPrecision       = 1e7
	stellarPrecisionDigits = 7
	stellarPageLimit       = 100
)

// stellarWallet is the bridge wallet
// Payments will be funded and fees will be taken with this wallet
type stellarWallet struct {
	keypair                   *keypair.Full
	config                    *StellarConfig
	stellarTransactionStorage *StellarTransactionStorage
	depositFee                int64
	signerWallet
}

type StellarConfig struct {
	// network for the stellar config
	StellarNetwork string
	// seed for the stellar bridge wallet
	StellarSeed string
	// stellar fee wallet address
	StellarFeeWallet string
}

type signerWallet struct {
	client         *SignersClient
	signatureCount int
}

func NewStellarWallet(ctx context.Context, config *StellarConfig, depositFee int64) (*stellarWallet, error) {
	kp, err := keypair.ParseFull(config.StellarSeed)

	if err != nil {
		return nil, err
	}

	stellarTransactionStorage := NewStellarTransactionStorage(config.StellarNetwork, kp.Address())
	w := &stellarWallet{
		keypair:                   kp,
		config:                    config,
		stellarTransactionStorage: stellarTransactionStorage,
		depositFee:                depositFee,
	}

	return w, nil
}

func (w *stellarWallet) GetAddress() string {
	return w.keypair.Address()
}

func (w *stellarWallet) newSignerClient(ctx context.Context, host host.Host, router routing.PeerRouting, relay *peer.AddrInfo) error {
	account, err := w.GetAccountDetails(w.keypair.Address())
	if err != nil {
		return err
	}
	var keys []string
	for _, signer := range account.Signers {
		if signer.Key == w.keypair.Address() {
			continue
		}
		keys = append(keys, signer.Key)
	}

	log.Info("required signature count", "signatures", int(account.Thresholds.MedThreshold))
	w.signatureCount = int(account.Thresholds.MedThreshold) - 1

	w.client, err = NewSignersClient(ctx, host, router, keys, relay)
	if err != nil {
		return err
	}
	return nil
}

func (w *stellarWallet) CreateAndSubmitPayment(ctx context.Context, target string, amount uint64, receiver common.Address, blockheight uint64, txHash common.Hash, message string) error {
	txnBuild, err := w.generatePaymentOperation(amount, target)
	if err != nil {
		return err
	}

	txnBuild.Memo = txnbuild.MemoHash(txHash)

	signReq := StellarSignRequest{
		RequiredSignatures: w.signatureCount,
		Receiver:           receiver,
		Block:              blockheight,
		Message:            message,
	}

	return w.submitTransaction(ctx, txnBuild, signReq)
}

func (w *stellarWallet) CreateAndSubmitRefund(ctx context.Context, target string, amount uint64, message string) error {
	txnBuild, err := w.generatePaymentOperation(amount, target)
	if err != nil {
		return err
	}

	parsedMessage, err := hex.DecodeString(message)
	if err != nil {
		return err
	}

	var memo [32]byte
	copy(memo[:], parsedMessage)

	txnBuild.Memo = txnbuild.MemoReturn(memo)

	signReq := StellarSignRequest{
		RequiredSignatures: w.signatureCount,
		Message:            message,
	}

	return w.submitTransaction(ctx, txnBuild, signReq)
}

// CreateAndSubmitFeepayment creates and submites a payment to the fee wallet
// only an amount and hash needs to be specified
func (w *stellarWallet) CreateAndSubmitFeepayment(ctx context.Context, amount uint64, txHash common.Hash) error {
	feeWalletAddress := w.keypair.Address()
	if w.config.StellarFeeWallet != "" {
		feeWalletAddress = w.config.StellarFeeWallet
	}

	txnBuild, err := w.generatePaymentOperation(amount, feeWalletAddress)
	if err != nil {
		return errors.Wrap(err, "failed to generate payment operation")
	}

	txnBuild.Memo = txnbuild.MemoHash(txHash)

	signReq := StellarSignRequest{
		RequiredSignatures: w.signatureCount,
	}

	return w.submitTransaction(ctx, txnBuild, signReq)
}

func (w *stellarWallet) generatePaymentOperation(amount uint64, destination string) (txnbuild.TransactionParams, error) {
	// if amount is zero, do nothing
	if amount == 0 {
		return txnbuild.TransactionParams{}, errors.New("invalid amount")
	}

	sourceAccount, err := w.GetAccountDetails(w.keypair.Address())
	if err != nil {
		return txnbuild.TransactionParams{}, errors.Wrap(err, "failed to get source account")
	}

	asset := w.GetAssetCodeAndIssuer()

	var paymentOperations []txnbuild.Operation
	paymentOP := txnbuild.Payment{
		Destination: destination,
		Amount:      big.NewRat(int64(amount), stellarPrecision).FloatString(stellarPrecisionDigits),
		Asset: txnbuild.CreditAsset{
			Code:   asset[0],
			Issuer: asset[1],
		},
		SourceAccount: sourceAccount.AccountID,
	}
	paymentOperations = append(paymentOperations, &paymentOP)

	if w.config.StellarFeeWallet != "" {
		feePaymentOP := txnbuild.Payment{
			Destination: w.config.StellarFeeWallet,
			Amount:      big.NewRat(WithdrawFee, stellarPrecision).FloatString(stellarPrecisionDigits),
			Asset: txnbuild.CreditAsset{
				Code:   asset[0],
				Issuer: asset[1],
			},
			SourceAccount: sourceAccount.AccountID,
		}
		paymentOperations = append(paymentOperations, &feePaymentOP)
	}

	txnBuild := txnbuild.TransactionParams{
		Operations:           paymentOperations,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		SourceAccount:        &sourceAccount,
		BaseFee:              stellarPrecision,
		IncrementSequenceNum: true,
	}

	return txnBuild, nil
}

// submitTransaction gathers signatures from cosigners if required and submits the transaction to the Stellar network
// If there already is a transaction with the same memo hash, no new transaction is created and submitted.
func (w *stellarWallet) submitTransaction(ctx context.Context, txn txnbuild.TransactionParams, signReq StellarSignRequest) error {
	tx, err := txnbuild.NewTransaction(txn)
	if err != nil {
		return errors.Wrap(err, "failed to build transaction")
	}

	// check if a similar transaction with a memo was made before
	exists, err := w.stellarTransactionStorage.TransactionExists(tx)
	if err != nil {
		return errors.Wrap(err, "failed to check transaction storage for existing transaction hash")
	}
	// if the transaction exists, return with nil error
	if exists {
		log.Info("Transaction with this hash already executed, skipping now..")
		return nil
	}

	client, err := w.GetHorizonClient()
	if err != nil {
		return errors.Wrap(err, "failed to get horizon client")
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
			return fmt.Errorf("received %d signatures, needed %d", len(signatures), w.signatureCount)
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
		if hError, ok := err.(*horizonclient.Error); ok {
			log.Error("Error submitting tx", "extras", hError.Problem.Extras)
		}
		return errors.Wrap(err, "failed to sign transaction with keypair")
	}

	// Submit the transaction
	txResult, err := client.SubmitTransaction(tx)
	if err != nil {
		if hError, ok := err.(*horizonclient.Error); ok {
			log.Error("Error submitting tx", "extras", hError.Problem.Extras)
		}
		return errors.Wrap(err, "error submitting transaction")
	}
	log.Info(fmt.Sprintf("transaction: %s submitted to the stellar network..", txResult.Hash))

	// Store TX
	w.stellarTransactionStorage.StoreTransaction(txResult)

	return nil
}

func (w *stellarWallet) refundDeposit(ctx context.Context, totalAmount uint64, tx hProtocol.Transaction) {
	if totalAmount <= uint64(WithdrawFee) {
		log.Warn("Deposited amount is smaller than the withdraw fee, not refunding", "tx", tx.Hash)
		return
	}
	amount := totalAmount - uint64(WithdrawFee)
	log.Warn("Calling refund", "tx", tx.Hash, "amount", amount, "account", tx.Account)

	err := w.CreateAndSubmitRefund(ctx, tx.Account, amount, tx.Hash)
	for err != nil {
		log.Error("error while trying to refund user", "err", err.Error(), "amount", totalAmount)
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
			err = w.CreateAndSubmitRefund(ctx, tx.Account, amount, tx.Hash)
		}
	}

}

// mint handler
type mint func(ERC20Address, *big.Int, string) error

// MonitorBridgeAccountAndMint is a blocking function that keeps monitoring
// the bridge account on the Stellar network for new transactions and calls the
// mint function when a deposit is made
func (w *stellarWallet) MonitorBridgeAccountAndMint(ctx context.Context, mintFn mint, persistency *ChainPersistency) error {
	transactionHandler := func(tx hProtocol.Transaction) {
		if !tx.Successful {
			return
		}
		log.Info("Received transaction on bridge stellar account", "hash", tx.Hash)

		totalAmount, err := w.getAmountFromTx(tx.Hash, w.keypair.Address())
		if err != nil || totalAmount == 0 {
			return
		}

		if totalAmount <= IntToStroops(w.depositFee) {
			log.Warn("Deposited amount is less than the depositfee, refunding")
			w.refundDeposit(ctx, uint64(totalAmount), tx)
			return
		}

		log.Info("deposited amount", "a", totalAmount)
		depositedAmount := big.NewInt(totalAmount)
		log.Info("memo", "m", tx.Memo)

		ethAddress, err := GetErc20AddressFromB64(tx.Memo)
		if err != nil {
			log.Warn("error decoding transaction memo, refunding", "error", err.Error())
			w.refundDeposit(ctx, uint64(totalAmount), tx)
			return
		}

		err = mintFn(ethAddress, depositedAmount, tx.Hash)
		for err != nil {
			log.Error(fmt.Sprintf("Error occured while minting: %s", err.Error()))
			if err == errInsufficientDepositAmount {
				log.Warn("User is trying to swap less than the fee amount, refunding now", "amount", totalAmount)
				w.refundDeposit(ctx, uint64(totalAmount), tx)
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				err = mintFn(ethAddress, depositedAmount, tx.Hash)
			}
		}

		if w.config.StellarFeeWallet != "" {
			log.Info("Trying to transfer the fees generated to the fee wallet", "address", w.config.StellarFeeWallet)

			// convert tx hash string to bytes
			parsedMessage, err := hex.DecodeString(tx.Hash)
			if err != nil {
				return
			}
			var memo [32]byte
			copy(memo[:], parsedMessage)

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
		}

		log.Info("Mint succesfull, saving cursor now")

		// save cursor
		cursor := tx.PagingToken()
		err = persistency.saveStellarCursor(cursor)
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

func (w *stellarWallet) getAmountFromTx(txHash string, forAccount string) (int64, error) {
	effects, err := w.getTransactionEffects(txHash)
	if err != nil {
		log.Error("error while fetching transaction effects:", err.Error())
		return 0, err
	}

	asset := w.GetAssetCodeAndIssuer()

	var totalAmount int64
	for _, effect := range effects.Embedded.Records {
		if effect.GetAccount() != forAccount {
			continue
		}
		if effect.GetType() == "account_credited" {
			creditedEffect := effect.(horizoneffects.AccountCredited)
			if creditedEffect.Asset.Code != asset[0] && creditedEffect.Asset.Issuer != asset[1] {
				continue
			}
			parsedAmount, err := amount.ParseInt64(creditedEffect.Amount)
			if err != nil {
				continue
			}

			totalAmount += parsedAmount
		}
	}

	return totalAmount, nil
}

// GetAccountDetails gets account details based an a Stellar address
func (w *stellarWallet) GetAccountDetails(address string) (account hProtocol.Account, err error) {
	client, err := w.GetHorizonClient()
	if err != nil {
		return hProtocol.Account{}, err
	}
	ar := horizonclient.AccountRequest{AccountID: address}
	account, err = client.AccountDetail(ar)
	if err != nil {
		return hProtocol.Account{}, errors.Wrapf(err, "failed to get account details for account: %s", address)
	}
	return account, nil
}

func (w *stellarWallet) StreamBridgeStellarTransactions(ctx context.Context, cursor string, handler func(op hProtocol.Transaction)) (err error) {
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
		err = fetchTransactions(ctx, client, w.keypair.Address(), cursor, internalHandler)
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

func fetchTransactions(ctx context.Context, client *horizonclient.Client, address string, cursor string, handler func(op hProtocol.Transaction)) error {
	timeouts := 0
	opRequest := horizonclient.TransactionRequest{
		ForAccount:    address,
		IncludeFailed: false,
		Cursor:        cursor,
		Limit:         stellarPageLimit,
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		response, err := client.Transactions(opRequest)
		if err != nil {
			log.Info("Error getting transactions for stellar account", "address", opRequest.ForAccount, "cursor", opRequest.Cursor, "pagelimit", opRequest.Limit, "error", err)
			horizonError, ok := err.(*horizonclient.Error)
			if ok && (horizonError.Response.StatusCode == http.StatusGatewayTimeout || horizonError.Response.StatusCode == http.StatusServiceUnavailable) {
				timeouts++
				if timeouts == 1 {
					opRequest.Limit = 5
				} else if timeouts > 1 {
					opRequest.Limit = 1
				}

				log.Info("Request timed out, lowering pagelimit", "pagelimit", opRequest.Limit)
			}

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
				continue
			}

		}
		for _, tx := range response.Embedded.Records {
			handler(tx)
			opRequest.Cursor = tx.PagingToken()
		}

		if timeouts > 0 {
			log.Info("Fetching transaction succeeded, resetting page limit and timeouts")
			opRequest.Limit = stellarPageLimit
			timeouts = 0
		}

		if len(response.Embedded.Records) == 0 {
			return nil
		}

	}

}

func (w *stellarWallet) ScanBridgeAccount() error {
	return w.stellarTransactionStorage.ScanBridgeAccount()
}

func (w *stellarWallet) getTransactionEffects(txHash string) (effects horizoneffects.EffectsPage, err error) {
	client, err := w.GetHorizonClient()
	if err != nil {
		return
	}

	effectsReq := horizonclient.EffectRequest{
		ForTransaction: txHash,
	}
	effects, err = client.Effects(effectsReq)
	return
}

// GetHorizonClient gets the horizon client based on the wallet's network
func (w *stellarWallet) GetHorizonClient() (*horizonclient.Client, error) {
	return GetHorizonClient(w.config.StellarNetwork)
}

// GetNetworkPassPhrase gets the Stellar network passphrase based on the wallet's network
func (w *stellarWallet) GetNetworkPassPhrase() string {
	switch w.config.StellarNetwork {
	case "testnet":
		return network.TestNetworkPassphrase
	case "production":
		return network.PublicNetworkPassphrase
	default:
		return network.TestNetworkPassphrase
	}
}

func (w *stellarWallet) GetAssetCodeAndIssuer() []string {
	switch w.config.StellarNetwork {
	case "testnet":
		return strings.Split(TFTTest, ":")
	case "production":
		return strings.Split(TFTMainnet, ":")
	default:
		return strings.Split(TFTTest, ":")
	}
}

// GetHorizonClient gets an horizon client for a specific network
func GetHorizonClient(network string) (*horizonclient.Client, error) {
	switch network {
	case "testnet":
		return horizonclient.DefaultTestNetClient, nil
	case "production":
		return horizonclient.DefaultPublicNetClient, nil
	default:
		return nil, errors.New("network is not supported")
	}
}

// IntToStroops converts units to stroops (1 TFT = 1000000 stroops)
func IntToStroops(x int64) int64 {
	return x * stellarPrecision
}
