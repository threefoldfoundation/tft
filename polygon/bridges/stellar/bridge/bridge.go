package bridge

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
