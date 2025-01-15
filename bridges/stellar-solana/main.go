package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	flag "github.com/spf13/pflag"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/api/bridge"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/solana"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/stellar"
)

var Version = "development"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(Version)
		return
	}

	var bridgeCfg bridge.BridgeConfig
	var stellarCfg stellar.StellarConfig
	var solCfg solana.SolanaConfig
	var bridgeMasterAddress string

	flag.StringVar(&bridgeCfg.PersistencyFile, "persistency", "./node.json", "file where last seen blockheight and stellar account cursor is stored")

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

	// Solana stuff
	flag.StringVar(&solCfg.KeyFile, "solana-key", "", "path to the solana keyfile containing the private key used to sign")
	flag.StringVar(&solCfg.NetworkName, "solana-network", "", "the solana network to connect to")
	flag.StringVar(&solCfg.TokenAddress, "solana-token-address", "", "the solana token address to bridge for")

	var debug bool
	flag.BoolVar(&debug, "debug", false, "sets debug level log output")

	flag.Parse()

	if err := stellarCfg.Validate(); err != nil {
		panic(err)
	}

	if err := solCfg.Validate(); err != nil {
		panic(err)
	}

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().Str("network", solCfg.NetworkName).Msg("solana network configured")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host, router, err := bridge.NewHost(ctx, stellarCfg.StellarSeed, bridgeCfg.Relay, bridgeCfg.Psk)
	if err != nil {
		fmt.Println("failed to create host")
		panic(err)
	}

	partialMA, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", host.ID()))
	if err != nil {
		panic(err)
	}

	for _, addr := range host.Addrs() {
		full := addr.Encapsulate(partialMA)
		log.Info().Str("address", full.String()).Msg("p2p node address")
	}

	txStorage := stellar.NewTransactionStorage(stellarCfg.StellarNetwork, bridgeMasterAddress)
	err = txStorage.ScanBridgeAccount(ctx)
	if err != nil {
		panic(err)
	}

	stellarWallet, err := stellar.NewWallet(&stellarCfg, bridgeCfg.DepositFee, bridge.WithdrawFee, txStorage)
	if err != nil {
		panic(err)
	}
	log.Info().Str("network", stellarCfg.StellarNetwork).Str("wallet", stellarWallet.GetAddress()).Msg("Stellar wallet loaded")

	sol, err := solana.New(ctx, &solCfg)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	br, err := bridge.NewBridge(ctx, stellarWallet, sol, &bridgeCfg, host, router)
	if err != nil {
		panic(err)
	}

	err = br.Start(ctx)
	if err != nil {
		panic(err)
	}

	// Start the signer server
	if bridgeCfg.Follower {
		err = bridge.NewSolIDServer(host, sol.Address())
		if err != nil {
			panic(err)
		}
		log.Info().Msg("Registered SolIDService")
		err = bridge.NewSignerServer(host, bridgeMasterAddress, sol, stellarWallet, bridgeCfg.DepositFee)
		if err != nil {
			panic(err)
		}
		log.Info().Msg("Registered SignerService")
	}

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Info().Msg("awaiting signal")
	sig := <-sigs
	log.Info().Str("signal", sig.String()).Msg("signal")
	cancel()
	err = br.Close()
	if err != nil {
		panic(err)
	}

	host.Close()
	log.Info().Msg("exiting")
	time.Sleep(time.Second * 5)
}
