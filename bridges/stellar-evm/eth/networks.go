package eth

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// NetworkConfiguration defines the Ethereum network specific configuration needed by the bridge
type NetworkConfiguration struct {
	NetworkID       uint64
	NetworkName     string
	ContractAddress common.Address
}

var ethNetworkConfigurations = map[string]NetworkConfiguration{
	"eth-mainnet": {
		NetworkID:       1,
		NetworkName:     "eth-mainnet",
		ContractAddress: common.HexToAddress("0x8f0FB159380176D324542b3a7933F0C2Fd0c2bbf"),
	},
	"sepolia-testnet": {
		NetworkID:       11155111,
		NetworkName:     "sepolia-testnet",
		ContractAddress: common.HexToAddress("0x3022415B85F4d1E6ce8E9a25904f018455607416"),
	},
	"goerli-testnet": {
		NetworkID:       5,
		NetworkName:     "goerli-testnet",
		ContractAddress: common.HexToAddress("0x33f92Ffd12A518ec3fe15875cAc8C8af45cF791d"),
	},
	"smart-chain-mainnet": {
		NetworkID:       56,
		NetworkName:     "bsc-mainnet",
		ContractAddress: common.HexToAddress("0x8f0FB159380176D324542b3a7933F0C2Fd0c2bbf"),
	},
	"smart-chain-testnet": {
		NetworkID:       97,
		NetworkName:     "bsc-testnet",
		ContractAddress: common.HexToAddress("0x4DFe8A53cD9dbA17038cAaDB4cd6743160dAf049"),
	},
	"hardhat": {
		NetworkID:       31337,
		NetworkName:     "homestead",
		ContractAddress: common.HexToAddress("0x4DFe8A53cD9dbA17038cAaDB4cd6743160dAf049"),
	},
}

// GetEthNetworkConfiguration returns the EthNetworkConAfiguration for a specific network
func GetEthNetworkConfiguration(networkname string) (networkconfig NetworkConfiguration, err error) {
	networkconfig, found := ethNetworkConfigurations[networkname]
	if !found {
		err = fmt.Errorf("network %s not supported", networkname)
	}
	return
}
