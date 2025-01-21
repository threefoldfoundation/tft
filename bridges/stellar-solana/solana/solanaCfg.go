package solana

import "github.com/gagliardetto/solana-go"

type SolanaConfig struct {
	// KeyFile path for the account to sign solana transaction with.
	KeyFile string
	// NetworkName of the solana network to connect to
	NetworkName string
	// TokenAddress of the Solana token to use in the bridge
	TokenAddress string
}

func (cfg SolanaConfig) Validate() error {
	var err error
	switch cfg.NetworkName {
	case "local":
	case "devnet":
	case "testnet":
	case "production":
	default:
		err = ErrSolanaNetworkNotSupported
	}

	if err != nil {
		_, err = solana.PublicKeyFromBase58(cfg.TokenAddress)
	}

	return err
}
