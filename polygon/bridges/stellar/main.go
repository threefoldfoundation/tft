package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/ethereum/go-ethereum/log"
	"github.com/threefoldfoundation/tft/polygon/bridges/stellar/bridge"
)

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))

	bridgeCfg := bridge.NewBridgeConfig()
	flag.StringVar(&bridgeCfg.EthNetworkName, "ethnetwork", "smart-chain-testnet", "eth network name (defines storage directory name)")
	flag.StringVar(&bridgeCfg.EthUrl, "ethurl", "ws://localhost:8576", "ethereum rpc url")
	flag.StringVar(&bridgeCfg.ContractAddress, "contract", "", "TFT smart contract address")

	flag.StringVar(&bridgeCfg.Datadir, "datadir", "./storage", "chain data directory")
	flag.StringVar(&bridgeCfg.PersistencyFile, "persistency", "./node.json", "file where last seen blockheight and stellar account cursor is stored")

	flag.StringVar(&bridgeCfg.AccountJSON, "account", "", "ethereum account json")
	flag.StringVar(&bridgeCfg.AccountPass, "password", "", "ethereum account password")

	flag.StringVar(&bridgeCfg.StellarSeed, "secret", "", "stellar secret")
	flag.StringVar(&bridgeCfg.StellarNetwork, "network", "testnet", "stellar network, testnet or production")
	// Stellar address where fees are sent to
	flag.StringVar(&bridgeCfg.FeeWallet, "feewallet", "", "stellar fee wallet address")

	flag.BoolVar(&bridgeCfg.RescanBridgeAccount, "rescan", false, "if true is provided, we rescan the bridge stellar account and mint all transactions again")
	flag.Int64Var(&bridgeCfg.RescanFromHeight, "rescanHeight", 0, "if provided, the bridge will rescan all withdraws from the given height")

	flag.BoolVar(&bridgeCfg.Follower, "follower", false, "if true then the bridge will run in follower mode meaning that it will not submit mint transactions to the multisig contract, if false the bridge will also submit transactions")

	flag.StringVar(&bridgeCfg.VaultAddress, "address", "", "bridge stellar address")

	flag.Parse()

	//TODO cfg.Validate()

	log.Info("Bor connection", "url", bridgeCfg.EthUrl)

	br, err := bridge.NewBridge(&bridgeCfg)
	if err != nil {
		panic(err)
	}
	bridgeContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = br.Start(bridgeContext)
	if err != nil {
		panic(err)
	}

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Info("awaiting signal")
	sig := <-sigs
	log.Info("signal", "signal", sig)

	log.Info("exiting")
	cancel()
	//Give everything some time to close off
	time.Sleep(time.Second * 5)
}
