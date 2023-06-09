package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
	"github.com/threefoldfoundation/tft/accountactivation/eth/contract"
)

//Example how to call the `ActivateAccount` function on the accountactivation contract

const (
	// retryDelay is the delay to retry calls when there are no peers
	retryDelay = time.Second * 15
	backOffMax = time.Second * 5
	gasLimit   = 210000
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "Usage:", os.Args[0], "eth_private_key stellaraddress contractaddress eth_node_url")
		os.Exit(1)
	}
	//ethPrivateKey := os.Args[1]
	//stellarAddress := os.Args[2]
	contractAddress := os.Args[3]
	ethNodeUrl := os.Args[4]

	cl, err := ethclient.Dial(ethNodeUrl)
	if err != nil {
		panic(err)
	}
	contractCaller, err := contract.NewAccountActivationCaller(common.HexToAddress(contractAddress), cl)
	cost, err := contractCaller.NetworkCost(&bind.CallOpts{
		Context: context.TODO(),
	}, "stellar")
	costInEth := decimal.NewFromBigInt(cost, -18)
	fmt.Println("Price to activate an account on the Stellar network:", cost, "Wei (=", costInEth, "Eth)")
}
