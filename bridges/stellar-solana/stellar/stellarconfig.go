package stellar

import "errors"

type StellarConfig struct {
	// network for the stellar config
	StellarNetwork string
	// seed for the stellar bridge wallet
	StellarSeed string
	// stellar fee wallet address
	StellarFeeWallet string
}

func (c *StellarConfig) Validate() (err error) {
	if c.StellarNetwork != "testnet" && c.StellarNetwork != "production" {
		return errors.New("The Stellar network has to be testnet or production")
	}
	if c.StellarSeed == "" {
		return errors.New("A Stellar secret is required")
	}
	if c.StellarFeeWallet == "" {
		return errors.New("A Fee wallet is required")
	}
	return
}
