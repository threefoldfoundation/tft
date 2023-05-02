package eth

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// NetworkConfiguration defines the Ethereum network specific configuration needed by the bridge
type NetworkConfiguration struct {
	NetworkID               uint64
	NetworkName             string
	ContractAddress         common.Address
	MultisigContractAddress common.Address
}

var ethNetworkConfigurations = map[string]NetworkConfiguration{
	"eth-mainnet": {
		NetworkID:               1,
		NetworkName:             "eth-mainnet",
		ContractAddress:         common.HexToAddress("0x8f0FB159380176D324542b3a7933F0C2Fd0c2bbf"),
		MultisigContractAddress: common.HexToAddress("0xa4E8d413004d46f367D4F09D6BD4EcBccfE51D33"),
	},
	"sepolia-testnet": {
		NetworkID:               11155111,
		NetworkName:             "sepolia-testnet",
		ContractAddress:         common.HexToAddress("0x3022415B85F4d1E6ce8E9a25904f018455607416"),
		MultisigContractAddress: common.HexToAddress("0xD59EE55B6B819a02f0eC0b3e1Bc435cA06CAE309"),
	},
	"goerli-testnet": {
		NetworkID:               5,
		NetworkName:             "goerli-testnet",
		ContractAddress:         common.HexToAddress("0xa8B0DDD11B6Bb53a79E62B8Ae8a1e2f68cd75338"),
		MultisigContractAddress: common.HexToAddress("0x4fD0f6fc13ADFF3D2aAb617702E31c49F715BE32"),
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
