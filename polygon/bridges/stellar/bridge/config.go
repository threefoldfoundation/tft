package bridge

import "math/big"

const (
	// EthBlockDelay is the amount of blocks to wait before
	// pushing eth transaction to the stellar network
	EthBlockDelay = 3
	// Depositing from Stellar to smart chain fee
	DefaultDepositFee = 50 * StellarPrecision
	// Withdrawing from smartchain to Stellar fee
	DefaultWithdrawFee = int64(1 * StellarPrecision)
)

type BridgeConfig struct {
	EthNetworkName      string
	EthUrl              string
	ContractAddress     string
	AccountJSON         string
	AccountPass         string
	Datadir             string
	RescanBridgeAccount bool
	RescanFromHeight    int64
	PersistencyFile     string
	Follower            bool
	DepositFee          *big.Int
	WithdrawFee         *big.Int
	StellarConfig
}

type StellarConfig struct {
	// Stellar Network
	StellarNetwork string
	// Seed for the stellar bridge signing account
	StellarSeed string
	// Stellar bridge address
	VaultAddress string
	// Stellar fee wallet address
	FeeWallet string
}

func NewBridgeConfig() (cfg BridgeConfig) {

	var depositFee big.Int
	depositFee.SetInt64(DefaultDepositFee)
	cfg.DepositFee = &depositFee

	var withdrawFee big.Int
	withdrawFee.SetInt64(DefaultWithdrawFee)
	cfg.WithdrawFee = &withdrawFee
	return
}
