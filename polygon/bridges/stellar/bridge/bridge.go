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
	// network for the stellar config
	StellarNetwork string
	// seed for the stellar bridge wallet
	StellarSeed string
	// Stellar bridge address
	VaultAddress string
	// stellar fee wallet address
	StellarFeeWallet string
}
