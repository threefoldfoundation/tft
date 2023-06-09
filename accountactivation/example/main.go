package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
	"github.com/threefoldfoundation/tft/accountactivation/eth/contract"

	"github.com/ethereum/go-ethereum/core/types"
)

//Example how to call the `ActivateAccount` function on the accountactivation contract

const gasLimit = 210000

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "Usage:", os.Args[0], "eth_private_key stellaraddress contractaddress eth_node_url")
		os.Exit(1)
	}
	ethPrivateKey := os.Args[1]
	stellarAddress := os.Args[2]
	contractAddress := os.Args[3]
	ethNodeUrl := os.Args[4]

	cl, err := ethclient.Dial(ethNodeUrl)
	if err != nil {
		panic(err)
	}
	// Fetch the price for activating an account on the Stellar network
	contractCaller, err := contract.NewAccountActivationCaller(common.HexToAddress(contractAddress), cl)
	if err != nil {
		panic(err)
	}
	cost, err := contractCaller.NetworkCost(&bind.CallOpts{
		Context: context.TODO(),
	}, "stellar")
	if err != nil {
		panic(err)
	}
	costInEth := decimal.NewFromBigInt(cost, -18)
	fmt.Println("Price to activate an account on the Stellar network:", cost, "Wei (=", costInEth, "Eth)")

	// Call the ActivateAccount function
	contractTransactor, err := contract.NewAccountActivationTransactor(common.HexToAddress(contractAddress), cl)
	if err != nil {
		panic(err)
	}

	privKey, err := crypto.HexToECDSA(strings.TrimPrefix(ethPrivateKey, "0x"))
	if err != nil {
		panic(err)
	}
	chainID, err := cl.ChainID(context.TODO())
	pubKey, _ := privKey.Public().(*ecdsa.PublicKey)
	ethereumAddress := crypto.PubkeyToAddress(*pubKey)
	tx, err := contractTransactor.ActivateAccount(&bind.TransactOpts{
		Context: context.TODO(), From: ethereumAddress,
		Signer: func(a common.Address, t *types.Transaction) (*types.Transaction, error) {
			return types.SignTx(t, types.LatestSignerForChainID(chainID), privKey)
		},
		Value:    cost,
		GasLimit: gasLimit,
	}, "stellar", stellarAddress)
	if err != nil {
		panic(err)
	}
	fmt.Println("Called activateAccount in transaction", tx.Hash())
}
