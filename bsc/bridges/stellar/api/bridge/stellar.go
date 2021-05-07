package bridge

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
	horizoneffects "github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/protocols/horizon/operations"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
)

const (
	TFTMainnet = "TFT:GBOVQKJYHXRR3DX6NOX2RRYFRCUMSADGDESTDNBDS6CDVLGVESRTAC47"
	TFTTest    = "TFT:GA47YZA3PKFUZMPLQ3B5F2E3CJIB57TGGU7SPCQT2WAEYKN766PWIMB3"

	stellarPrecision       = 1e7
	stellarPrecisionDigits = 7
)

// stellarWallet is the bridge wallet
// Payments will be funded and fees will be taken with this wallet
type stellarWallet struct {
	keypair        *keypair.Full
	client         *SignersClient
	signatureCount int
	config         *StellarConfig
}

func newStellarWallet(ctx context.Context, config *StellarConfig, host host.Host, router routing.PeerRouting) (*stellarWallet, error) {
	kp, err := keypair.ParseFull(config.StellarSeed)

	if err != nil {
		return nil, err
	}

	w := &stellarWallet{
		keypair: kp,
		config:  config,
		//client:  client,
	}

	account, err := w.GetAccountDetails(kp.Address())
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, signer := range account.Signers {
		if signer.Key == kp.Address() {
			continue
		}
		keys = append(keys, signer.Key)
	}

	log.Info("required signature count", "signatures", int(account.Thresholds.MedThreshold))
	w.signatureCount = int(account.Thresholds.MedThreshold) - 1

	w.client, err = NewSignersClient(ctx, host, router, keys)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (w *stellarWallet) CreateAndSubmitPayment(ctx context.Context, target string, network string, amount uint64, receiver common.Address, blockheight uint64, txHash common.Hash, message string) error {
	txnBuild, err := w.generatePaymentOperation(amount, target)
	if err != nil {
		return err
	}

	txnBuild.Memo = txnbuild.MemoHash(txHash)

	signReq := SignRequest{
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

	signReq := SignRequest{
		RequiredSignatures: w.signatureCount,
		Message:            message,
	}

	return w.submitTransaction(ctx, txnBuild, signReq)
}

// CreateAndSubmitFeepayment creates and submites a payment to the fee wallet
// only an amount needs to be specified
func (w *stellarWallet) CreateAndSubmitFeepayment(ctx context.Context, amount uint64) error {
	feeWalletAddress := w.keypair.Address()
	if w.config.StellarFeeWallet != "" {
		feeWalletAddress = w.config.StellarFeeWallet
	}

	txnBuild, err := w.generatePaymentOperation(amount, feeWalletAddress)
	if err != nil {
		return errors.Wrap(err, "failed to generate payment operation")
	}

	signReq := SignRequest{
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

	paymentOP := txnbuild.Payment{
		Destination: destination,
		Amount:      big.NewRat(int64(amount), stellarPrecision).FloatString(stellarPrecisionDigits),
		Asset: txnbuild.CreditAsset{
			Code:   asset[0],
			Issuer: asset[1],
		},
		SourceAccount: sourceAccount.AccountID,
	}

	txnBuild := txnbuild.TransactionParams{
		Operations:           []txnbuild.Operation{&paymentOP},
		Timebounds:           txnbuild.NewTimeout(300),
		SourceAccount:        &sourceAccount,
		BaseFee:              txnbuild.MinBaseFee * 3,
		IncrementSequenceNum: true,
	}

	return txnBuild, nil
}

func (w *stellarWallet) submitTransaction(ctx context.Context, txn txnbuild.TransactionParams, signReq SignRequest) error {
	tx, err := txnbuild.NewTransaction(txn)
	if err != nil {
		return errors.Wrap(err, "failed to build transaction")
	}

	client, err := w.GetHorizonClient()
	if err != nil {
		return errors.Wrap(err, "failed to get horizon client")
	}

	txnXDR, err := tx.Base64()
	if err != nil {
		return errors.Wrap(err, "failed to serialize transaction")
	}

	signReq.TxnXDR = txnXDR

	signatures, err := w.client.Sign(ctx, signReq)
	if err != nil {
		return err
	}

	for _, signature := range signatures {
		tx, err = tx.AddSignatureBase64(w.GetNetworkPassPhrase(), signature.Address, signature.Signature)
		if err != nil {
			log.Error("Failed to add signature", "err", err.Error())
			return err
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

	return nil
}

// mint handler
type mint func(ERC20Address, *big.Int, string) error

func (w *stellarWallet) MonitorBridgeAndMint(mintFn mint, persistency *ChainPersistency) error {
	transactionHandler := func(tx hProtocol.Transaction) {
		if !tx.Successful {
			return
		}

		data, err := base64.StdEncoding.DecodeString(tx.Memo)
		if err != nil {
			log.Error("error while decoding transaction memo:", err.Error())
			return
		}

		if len(data) != 20 {
			return
		}
		var ethAddress ERC20Address
		copy(ethAddress[0:20], data)

		effects, err := w.getTransactionEffects(tx.Hash)
		if err != nil {
			log.Error("error while fetching transaction effects:", err.Error())
			return
		}

		asset := w.GetAssetCodeAndIssuer()

		for _, effect := range effects.Embedded.Records {
			if effect.GetAccount() != w.keypair.Address() {
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

				depositedAmount := big.NewInt(int64(parsedAmount))

				err = mintFn(ethAddress, depositedAmount, tx.Hash)
				if err != nil {
					log.Error(fmt.Sprintf("Error occured while minting: %s", err.Error()))
					if err == errInsufficientDepositAmount {
						log.Warn("User is trying to swap less than the fee amount, refunding now", "amount", parsedAmount)
						ops, err := w.getOperationEffect(tx.Hash)
						if err != nil {
							continue
						}
						for _, op := range ops.Embedded.Records {
							if op.GetType() == "payment" {
								paymentOpation := op.(operations.Payment)

								if paymentOpation.To == w.keypair.Address() {
									log.Warn("Calling refund")
									err := w.CreateAndSubmitRefund(context.Background(), paymentOpation.From, uint64(parsedAmount), tx.Hash)
									if err != nil {
										log.Error("error while trying to refund user", "err", err.Error())
									}
								}
							}
						}
					}
					continue
				}
				if w.config.StellarFeeWallet != "" {
					log.Info("Trying to transfer the fees generated to the fee wallet", "address", w.config.StellarFeeWallet)
					err = w.CreateAndSubmitFeepayment(context.Background(), DepositFee)
					if err != nil {
						log.Error("error while to send fees to the fee wallet", "err", err.Error())
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
		}

	}

	// get saved cursor
	blockHeight, err := persistency.GetHeight()
	if err != nil {
		return err
	}

	return w.StreamBridgeStellarTransactions(context.Background(), blockHeight.StellarCursor, transactionHandler)
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

func (w *stellarWallet) StreamBridgeStellarTransactions(ctx context.Context, cursor string, handler func(op hProtocol.Transaction)) error {
	client, err := w.GetHorizonClient()
	if err != nil {
		return err
	}

	opRequest := horizonclient.TransactionRequest{
		ForAccount: w.keypair.Address(),
		Cursor:     cursor,
	}

	return client.StreamTransactions(ctx, opRequest, handler)
}

func (w *stellarWallet) getTransactionEffects(txHash string) (effects horizoneffects.EffectsPage, err error) {
	client, err := w.GetHorizonClient()
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

func (w *stellarWallet) getOperationEffect(txHash string) (ops operations.OperationsPage, err error) {
	client, err := w.GetHorizonClient()
	if err != nil {
		return ops, err
	}

	opsRequest := horizonclient.OperationRequest{
		ForTransaction: txHash,
	}
	ops, err = client.Operations(opsRequest)
	if err != nil {
		return ops, err
	}

	return ops, nil
}

// GetHorizonClient gets the horizon client based on the wallet's network
func (w *stellarWallet) GetHorizonClient() (*horizonclient.Client, error) {
	switch w.config.StellarNetwork {
	case "testnet":
		return horizonclient.DefaultTestNetClient, nil
	case "production":
		return horizonclient.DefaultPublicNetClient, nil
	default:
		return nil, errors.New("network is not supported")
	}
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
