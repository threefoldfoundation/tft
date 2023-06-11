package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/threefoldfoundation/tft/accountactivation/eth"
	"github.com/threefoldfoundation/tft/accountactivation/stellar"
)

func main() {
	var cfg Config

	flag.StringVar(&cfg.EthUrl, "ethurl", "ws://localhost:8551", "ethereum rpc url")
	flag.StringVar(&cfg.ContractAddress, "contract", "", "token contract address")

	flag.StringVar(&cfg.PersistencyFile, "persistency", "./state.json", "file where last seen blockheight is stored")

	flag.StringVar(&cfg.StellarSecret, "secret", "", "secret of the stellar account that activates new accounts")
	flag.StringVar(&cfg.StellarNetwork, "network", "testnet", "stellar network, testnet or production")
	flag.Uint64Var(&cfg.RescanFromHeight, "rescanHeight", 0, "if provided, the bridge will rescan all withdraws from the given height")

	var debug bool
	flag.BoolVar(&debug, "debug", false, "sets debug level log output")

	flag.Parse()

	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	logLevel := log.LvlInfo
	if debug {
		logLevel = log.LvlDebug
	}
	log.Root().SetHandler(log.LvlFilterHandler(logLevel, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))

	log.Info("Ethereum node", "url", cfg.EthUrl)

	activationAccountAddress, err := stellar.AccountAdressFromSecret(cfg.StellarSecret)
	if err != nil {
		panic(err)
	}
	txStorage := stellar.NewTransactionStorage(cfg.StellarNetwork, activationAccountAddress)
	log.Info("Loading memo's from previous activation transactions", "account", activationAccountAddress)
	err = txStorage.ScanAccount()
	if err != nil {
		panic(err)
	}

	cw, err := eth.NewContractWatcher(cfg.EthUrl, cfg.ContractAddress, cfg.PersistencyFile)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := cw.Start(ctx, cfg.RescanFromHeight)
		if err != nil {
			panic(err)
		}
	}()

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Info("awaiting signal")
	sig := <-sigs
	log.Info("signal", "signal", sig)
	cancel()
	cw.Close()
	log.Info("exiting")
	time.Sleep(time.Second * 5)
}
