package main

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/ethereum/go-ethereum/log"
	"github.com/threefoldfoundation/tft/accountactivation/eth"
	"github.com/threefoldfoundation/tft/accountactivation/stellar"
)

func handleRequests(ctx context.Context, wallet stellar.Wallet, txStorage *stellar.TransactionStorage, activationRequests chan eth.AccounActivationRequest) {
loop:
	for {
		select {
		case r := <-activationRequests:
			if r.Network != "stellar" {
				log.Info("Request for unknown network", "network", r.Network)
			}
			memo := hex.EncodeToString(r.EthereumTransaction.Bytes())
			alreadyHandled := txStorage.TransactionWithMemoExists(memo)

			if alreadyHandled {
				log.Info("Transaction with this memo already executed, skipping")
				continue loop
			}
			err := wallet.ActivateAccount(r.Account, r.EthereumTransaction)
			for err != nil {
				//Errors which should just be ignored
				if errors.Is(err, stellar.ErrAccountAlreadyExists) {
					log.Info(err.Error())
					continue loop
				}
				log.Warn("Account Activation failed", "err", err)
				err = wallet.ActivateAccount(r.Account, r.EthereumTransaction)
			}

		case <-ctx.Done():
			return
		}
	}
}
