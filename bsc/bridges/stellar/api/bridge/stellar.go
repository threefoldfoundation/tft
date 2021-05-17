package bridge

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/protocols/horizon/effects"
	horizoneffects "github.com/stellar/go/protocols/horizon/effects"
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
	keypair *keypair.Full
	network string
	signerWallet
}

type signerWallet struct {
	client         *SignersClient
	signatureCount int
}

func newStellarWallet(network, seed string) (*stellarWallet, error) {
	kp, err := keypair.ParseFull(seed)

	if err != nil {
		return nil, err
	}

	w := &stellarWallet{
		keypair: kp,
		network: network,
	}

	return w, nil
}

func (w *stellarWallet) newSignerWallet(ctx context.Context, host host.Host, router routing.PeerRouting) error {
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

	w.client, err = NewSignersClient(ctx, host, router, keys)
	if err != nil {
		return err
	}
	return nil
}

func (w *stellarWallet) CreateAndSubmitPayment(ctx context.Context, target string, network string, amount uint64, receiver common.Address, blockheight uint64, txHash common.Hash) error {
	// if amount is zero, do nothing
	if amount == 0 {
		return nil
	}

	sourceAccount, err := w.GetAccountDetails(w.keypair.Address())
	if err != nil {
		return errors.Wrap(err, "failed to get source account")
	}

	asset := w.GetAssetCodeAndIssuer()

	paymentOP := txnbuild.Payment{
		Destination: target,
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

	// Only try to request signatures if there are signatures required
	if w.signatureCount > 0 {
		signReq := SignRequest{
			TxnXDR:             xdr,
			RequiredSignatures: w.signatureCount,
			Receiver:           receiver,
			Block:              blockheight,
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

// MonitorBridgeAccountAndMint is a blocking function that keeps monitoring
// the bridge account on the Stellar network for new transactions and calls the
// mint function when a deposit is made
func (w *stellarWallet) MonitorBridgeAccountAndMint(ctx context.Context, mintFn mint, persistency *ChainPersistency) error {
	transactionHandler := func(tx hProtocol.Transaction) {
		if !tx.Successful {
			return
		}
		log.Info("Received transaction on bridge stellar account", "hash", tx.Hash)

		data, err := base64.StdEncoding.DecodeString(tx.Memo)
		if err != nil {
			log.Error("error decoding transaction memo", "error", err.Error())
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
					return
				}

				log.Info("Mint succesfull, saving cursor")
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
	for err != nil {
		log.Warn("Error getting the bridge persistency", "error", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
			continue
		}
		blockHeight, err = persistency.GetHeight()
	}

	return w.StreamBridgeStellarTransactions(ctx, blockHeight.StellarCursor, transactionHandler)
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
	log.Info("Start fetching stellar transactions", "horizon", client.HorizonURL, "account", opRequest.ForAccount, "cursor", opRequest.Cursor)

	for {
		if ctx.Err() != nil {
			return nil
		}

		response, err := client.Transactions(opRequest)
		if err != nil {
			log.Info("Error getting transactions for stellar account", "error", err)
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
		if len(response.Embedded.Records) == 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(10 * time.Second):
			}
		}

	}

}

func (w *stellarWallet) getTransactionEffects(txHash string) (effects effects.EffectsPage, err error) {
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

// GetHorizonClient gets the horizon client based on the wallet's network
func (w *stellarWallet) GetHorizonClient() (*horizonclient.Client, error) {
	switch w.network {
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
	switch w.network {
	case "testnet":
		return network.TestNetworkPassphrase
	case "production":
		return network.PublicNetworkPassphrase
	default:
		return network.TestNetworkPassphrase
	}
}

func (w *stellarWallet) GetAssetCodeAndIssuer() []string {
	switch w.network {
	case "testnet":
		return strings.Split(TFTTest, ":")
	case "production":
		return strings.Split(TFTMainnet, ":")
	default:
		return strings.Split(TFTTest, ":")
	}
}
