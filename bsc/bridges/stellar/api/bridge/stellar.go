package bridge

import (
	"context"
	"encoding/base64"
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
	withdrawFee            = 0.1 * stellarPrecision
	// Fee amount is the amount of fee that is required
	// to process a Stellar to Binance transaction
	feeAmount = 47 * stellarPrecision
)

// stellarWallet is the bridge wallet
type stellarWallet struct {
	keypair *keypair.Full
	client  *SignersClient
	config  *StellarConfig
}

func newStellarWallet(ctx context.Context, config StellarConfig, host host.Host, router routing.PeerRouting) (*stellarWallet, error) {
	kp, err := keypair.ParseFull(config.StellarSeed)

	if err != nil {
		return nil, err
	}

	w := &stellarWallet{
		keypair: kp,
		config:  &config,
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

	w.client, err = NewSignersClient(ctx, host, router, keys)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (w *stellarWallet) CreateAndSubmitPayment(ctx context.Context, target string, network string, amount uint64, receiver common.Address, blockheight uint64, txHash common.Hash, message string) error {
	// if amount is zero, do nothing
	if amount == 0 {
		return nil
	}

	if network != w.config.StellarNetwork {
		return fmt.Errorf("cannot withdraw on network: %s, while the bridge is running on: %s", network, w.config.StellarNetwork)
	}

	sourceAccount, err := w.GetAccountDetails(w.keypair.Address())
	if err != nil {
		return errors.Wrap(err, "failed to get source account")
	}

	asset := w.GetAssetCodeAndIssuer()

	operations := []txnbuild.Operation{}

	if target != w.config.StellarFeeWallet {
		amount = amount - withdrawFee
		feeWalletAddress := w.keypair.Address()
		if w.config.StellarFeeWallet != "" {
			feeWalletAddress = w.config.StellarFeeWallet
		}
		log.Info("Fee wallet address", "address", feeWalletAddress)
		// Transfer the fee amount to the fee wallet
		feePayoutOperation := txnbuild.Payment{
			Destination: feeWalletAddress,
			Amount:      big.NewRat(int64(withdrawFee), stellarPrecision).FloatString(stellarPrecisionDigits),
			Asset: txnbuild.CreditAsset{
				Code:   asset[0],
				Issuer: asset[1],
			},
			SourceAccount: sourceAccount.AccountID,
		}

		operations = append(operations, &feePayoutOperation)
	}

	// Payout the amount minus the fee back to the user
	payoutOperation := txnbuild.Payment{
		Destination: target,
		Amount:      big.NewRat(int64(amount), stellarPrecision).FloatString(stellarPrecisionDigits),
		Asset: txnbuild.CreditAsset{
			Code:   asset[0],
			Issuer: asset[1],
		},
		SourceAccount: sourceAccount.AccountID,
	}
	operations = append(operations, &payoutOperation)

	txnBuild := txnbuild.TransactionParams{
		Operations:           operations,
		Timebounds:           txnbuild.NewTimeout(300),
		SourceAccount:        &sourceAccount,
		BaseFee:              txnbuild.MinBaseFee * 3,
		IncrementSequenceNum: true,
		Memo:                 txnbuild.MemoHash(txHash),
	}

	tx, err := txnbuild.NewTransaction(txnBuild)
	if err != nil {
		return errors.Wrap(err, "failed to build transaction")
	}
	client, err := w.GetHorizonClient()
	if err != nil {
		return errors.Wrap(err, "failed to get horizon client")
	}

	xdr, err := tx.Base64()
	if err != nil {
		return errors.Wrap(err, "failed to serialize transaction")
	}

	// signature count is the amount of signers needed to perform medium threshold operations (payments, ..)
	// we subtract by 1 because end the end of this function the called will also sign, which will fullfill
	// the amount of required signatures
	signatureCount := int(sourceAccount.Thresholds.MedThreshold) - 1
	log.Info("required signature count", "signatures", int(signatureCount))

	signReq := SignRequest{
		TxnXDR:             xdr,
		RequiredSignatures: signatureCount,
		Receiver:           receiver,
		Block:              blockheight,
		Message:            message,
	}

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
		// save cursor
		cursor := tx.PagingToken()
		err := persistency.saveStellarCursor(cursor)
		if err != nil {
			log.Error("error while saving cursor:", err.Error())
			return
		}

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

				if parsedAmount <= feeAmount {
					log.Warn("User is trying to swap less than the fee amount, reverting now", "amount", parsedAmount)
					ops, err := w.getOperationEffect(tx.Hash)
					if err != nil {
						continue
					}
					for _, op := range ops.Embedded.Records {
						if op.GetType() == "payment" {
							paymentOpation := op.(operations.Payment)

							if paymentOpation.To == w.keypair.Address() {
								log.Warn("Calling refund")
								err := w.CreateAndSubmitPayment(context.Background(), paymentOpation.From, w.config.StellarNetwork, uint64(parsedAmount), common.Address{}, 0, common.Hash{}, tx.Hash)
								if err != nil {
									log.Error("error while trying to refund user", "err", err.Error())
								}
							}
						}
					}

					continue
				}

				// Calculate amount minus the fee for allowing this mint
				amount_minus_fee := parsedAmount - feeAmount

				// Parse amount
				eth_amount := big.NewInt(int64(amount_minus_fee))

				err = mintFn(ethAddress, eth_amount, tx.Hash)
				if err != nil {
					log.Error(fmt.Sprintf("Error occured while minting: %s", err.Error()))

					log.Info("Going to try to refund due to a failed mint")
					err := w.CreateAndSubmitPayment(context.Background(), tx.Account, w.config.StellarNetwork, uint64(parsedAmount), common.Address{}, 0, common.Hash{}, tx.Hash)
					if err != nil {
						log.Error("error while trying to refund user for a failed mint", "err", err.Error())
					}
					continue
				}

				if w.config.StellarFeeWallet != "" {
					log.Info("Trying to transfer the fees generated to the fee wallet", "address", w.config.StellarFeeWallet)
					err = w.CreateAndSubmitPayment(context.Background(), w.config.StellarFeeWallet, w.config.StellarNetwork, uint64(feeAmount), common.Address{}, 0, common.Hash{}, "")
					if err != nil {
						log.Error("error while trying to refund user", "err", err.Error())
					}
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
