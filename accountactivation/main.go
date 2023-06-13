package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/log"
	"github.com/threefoldfoundation/tft/accountactivation/eth"
	"github.com/threefoldfoundation/tft/accountactivation/state"
	"github.com/threefoldfoundation/tft/accountactivation/stellar"
)

var Version = "development"

func main() {
	var cfg Config

	flag.StringVar(&cfg.EthUrl, "ethurl", "ws://localhost:8551", "ethereum rpc url")
	flag.StringVar(&cfg.ContractAddress, "contract", "", "token contract address")

	flag.StringVar(&cfg.PersistencyFile, "persistency", "./state.json", "file where last seen blockheight is stored")

	flag.StringVar(&cfg.StellarSecret, "secret", "", "secret of the stellar account that activates new accounts")
	flag.StringVar(&cfg.StellarNetwork, "network", "testnet", "stellar network, testnet or production")
	flag.Uint64Var(&cfg.RescanFromHeight, "rescanHeight", 0, "if provided, the bridge will rescan all withdraws from the given height")
	version := flag.Bool("version", false, "Print the version and exit")
	var debug bool
	flag.BoolVar(&debug, "debug", false, "sets debug level log output")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s (version %s):\n", os.Args[0], Version)
		flag.PrintDefaults()
	}
	flag.Parse()
	if *version {
		fmt.Println(Version)
		os.Exit(0)
	}
	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	logLevel := log.LvlInfo
	if debug {
		logLevel = log.LvlDebug
	}
	log.Root().SetHandler(log.LvlFilterHandler(logLevel, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))

	log.Info("starting AccountActivation", "version", Version)
	log.Info("Ethereum node", "url", cfg.EthUrl)

	activationAccountAddress, err := stellar.AccountAdressFromSecret(cfg.StellarSecret)
	if err != nil {
		panic(err)
	}
	txStorage := stellar.NewTransactionStorage(cfg.StellarNetwork, activationAccountAddress)
	log.Info("Loading memo's from previous activation transactions", "account", activationAccountAddress)
	err = txStorage.ScanAccount(context.Background())
	if err != nil {
		panic(err)
	}
	blockPersistency := state.NewChainPersistency(cfg.PersistencyFile)
	activationRequests := make(chan eth.AccounActivationRequest)
	cw, err := eth.NewContractWatcher(cfg.EthUrl, cfg.ContractAddress, blockPersistency, activationRequests)
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
	wallet := stellar.NewWallet(cfg.StellarSecret, cfg.StellarNetwork)
	go handleRequests(ctx, wallet, txStorage, blockPersistency, activationRequests)

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Info("awaiting signal")
	sig := <-sigs
	log.Info("signal", "signal", sig)
	cancel()
	cw.Close()
	log.Info("exiting")
}
