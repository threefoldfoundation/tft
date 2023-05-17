package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/multiformats/go-multiaddr"
	flag "github.com/spf13/pflag"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/api/bridge"
	"github.com/threefoldfoundation/tft/bridges/stellar-evm/stellar"

	"github.com/ethereum/go-ethereum/log"
)

func main() {

	var bridgeCfg bridge.BridgeConfig
	var stellarCfg stellar.StellarConfig
	var ethCfg bridge.EthConfig
	var bridgeMasterAddress string

	flag.StringVar(&ethCfg.EthNetworkName, "ethnetwork", "eth-mainnet", "ethereum network name")
	flag.StringVar(&ethCfg.EthUrl, "ethurl", "ws://localhost:8551", "ethereum rpc url")
	flag.StringVar(&ethCfg.ContractAddress, "contract", "", "token contract address")

	flag.StringVar(&bridgeCfg.PersistencyFile, "persistency", "./node.json", "file where last seen blockheight and stellar account cursor is stored")

	flag.StringVar(&ethCfg.EthPrivateKey, "ethkey", "", "ethereum account private key")

	flag.StringVar(&stellarCfg.StellarSeed, "secret", "", "stellar secret")
	flag.StringVar(&stellarCfg.StellarNetwork, "network", "testnet", "stellar network, testnet or production")
	// Stellar account where fees are sent to
	flag.StringVar(&stellarCfg.StellarFeeWallet, "feewallet", "", "stellar fee wallet address")

	flag.BoolVar(&bridgeCfg.RescanBridgeAccount, "rescan", false, "if true is provided, we rescan the bridge stellar account and mint all transactions again")
	flag.Int64Var(&bridgeCfg.RescanFromHeight, "rescanHeight", 0, "if provided, the bridge will rescan all withdraws from the given height")

	flag.BoolVar(&bridgeCfg.Follower, "follower", false, "if true then the bridge will run in follower mode meaning that it will not submit mint transactions to the multisig contract, if false the bridge will also submit transactions")

	flag.StringVar(&bridgeMasterAddress, "master", "", "master stellar public address")
	flag.Int64Var(&bridgeCfg.DepositFee, "depositFee", 50, "sets the depositfee in TFT")

	// P2P Configuration
	flag.StringVar(&bridgeCfg.Psk, "psk", "", "psk for the relay")
	flag.StringVar(&bridgeCfg.Relay, "relay", "", "relay address")

	var debug bool
	flag.BoolVar(&debug, "debug", false, "sets debug level log output")

	flag.Parse()

	if err := stellarCfg.Validate(); err != nil {
		panic(err)
	}

	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
	if debug {
		log.Root().SetHandler(log.LvlFilterHandler(log.LvlDebug, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
	}

	log.Info("connection url provided: ", "url", ethCfg.EthUrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host, router, err := bridge.NewHost(ctx, stellarCfg.StellarSeed, bridgeCfg.Relay, bridgeCfg.Psk)
	if err != nil {
		fmt.Println("failed to create host")
		panic(err)
	}

	ipfs, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ipfs/%s", host.ID().Pretty()))
	if err != nil {
		panic(err)
	}

	for _, addr := range host.Addrs() {
		full := addr.Encapsulate(ipfs)
		log.Info("p2p node address", "address", full.String())
	}

	txStorage := stellar.NewTransactionStorage(stellarCfg.StellarNetwork, bridgeMasterAddress)
	err = txStorage.ScanBridgeAccount()
	if err != nil {
		panic(err)
	}

	stellarWallet, err := stellar.NewWallet(ctx, &stellarCfg, bridgeCfg.DepositFee, bridge.WithdrawFee, txStorage)
	if err != nil {
		panic(err)
	}
	log.Info(fmt.Sprintf("Stellar wallet %s loaded on Stellar network %s", stellarWallet.GetAddress(), stellarCfg.StellarNetwork))

	contract, err := bridge.NewBridgeContract(&ethCfg)
	if err != nil {
		panic(err)
	}

	br, err := bridge.NewBridge(ctx, stellarWallet, contract, &bridgeCfg, host, router)
	if err != nil {
		panic(err)
	}

	err = br.Start(ctx)
	if err != nil {
		panic(err)
	}

	// Start the signer server
	if bridgeCfg.Follower {
		err := bridge.NewSignerServer(host, bridgeMasterAddress, contract, stellarWallet, bridgeCfg.DepositFee)
		if err != nil {
			panic(err)
		}
	}

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Info("awaiting signal")
	sig := <-sigs
	log.Info("signal", "signal", sig)
	cancel()
	err = br.Close()
	if err != nil {
		panic(err)
	}

	host.Close()
	log.Info("exiting")
	time.Sleep(time.Second * 5)
}
