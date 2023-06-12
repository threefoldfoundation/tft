package main

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/threefoldfoundation/tft/accountactivation/eth"
	"github.com/threefoldfoundation/tft/accountactivation/state"
	"github.com/threefoldfoundation/tft/accountactivation/stellar"
)

func handleRequests(ctx context.Context, wallet stellar.Wallet, txStorage *stellar.TransactionStorage, blockPersistency *state.ChainPersistency, activationRequests chan eth.AccounActivationRequest) {
loop:
	for {
		select {
		case r := <-activationRequests:
			if r.Network != "stellar" {
				log.Info("Request for unknown network", "network", r.Network)
				continue loop
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
				if errors.Is(err, stellar.ErrAccountAlreadyExists) || errors.Is(err, stellar.ErrInvalidAddress) {
					log.Info(err.Error())
					continue loop
				}
				log.Warn("Account Activation failed", "err", err)
				//Wait a bit before retrying
				select {
				case <-ctx.Done(): //context cancelled
					return
				case <-time.After(time.Second * 10): //timeout
				}
				err = wallet.ActivateAccount(r.Account, r.EthereumTransaction)
			}
			if err := blockPersistency.SaveHeight(r.BlockNumber); err != nil {
				// log but ignore
				log.Error("Failed to save blocknumber", "err", err)
			}

		case <-ctx.Done():
			return
		}
	}
}
