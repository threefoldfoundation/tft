package main

import (
	"errors"

	"github.com/threefoldfoundation/tft/accountactivation/stellar"
)

type Config struct {
	EthUrl           string
	ContractAddress  string
	RescanFromHeight uint64
	PersistencyFile  string
	StellarNetwork   string // stellar network
	StellarSecret    string // secret of the stellar account that activates new accounts
}

func (c *Config) Validate() (err error) {
	if c.StellarNetwork != "testnet" && c.StellarNetwork != "production" {
		return errors.New("The Stellar network has to be testnet or production")
	}
	if !stellar.IsValidStellarSecret(c.StellarSecret) {
		return errors.New("Invalid account activation secret")
	}
	return
}
