package eth

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
)

//NetworkConfiguration defines the Ethereum network specific configuration needed by the bridge
type NetworkConfiguration struct {
	NetworkID               uint64
	NetworkName             string
	ContractAddress         common.Address
	MultisigContractAddress common.Address
}

var ethNetworkConfigurations = map[string]NetworkConfiguration{
	"smart-chain-mainnet": {
		NetworkID:               56,
		NetworkName:             "bsc-mainnet",
		ContractAddress:         common.HexToAddress("0x8f0FB159380176D324542b3a7933F0C2Fd0c2bbf"),
		MultisigContractAddress: common.HexToAddress("0xa4E8d413004d46f367D4F09D6BD4EcBccfE51D33"),
	},
	"smart-chain-testnet": {
		NetworkID:               97,
		NetworkName:             "bsc-testnet",
		ContractAddress:         common.HexToAddress("0x4DFe8A53cD9dbA17038cAaDB4cd6743160dAf049"),
		MultisigContractAddress: common.HexToAddress("0x0586d6afA50fA3b47FB51a34b906Ec8Fab5ACE0D"),
	},
}

//GetEthNetworkConfiguration returns the EthNetworkConAfiguration for a specific network
func GetEthNetworkConfiguration(networkname string) (networkconfig NetworkConfiguration, err error) {
	networkconfig, found := ethNetworkConfigurations[networkname]
	if !found {
		err = fmt.Errorf("network %s not supported", networkname)
	}
	return
}

func GetTestnetGenesisBlock() *core.Genesis {
	genesis := &core.Genesis{}
	f, err := os.Open("./genesis/testnet-genesis.json")
	if err != nil {
		panic(err)
	}
	if err := json.NewDecoder(f).Decode(genesis); err != nil {
		panic(err)
	}

	return genesis
}

func GetMainnetGenesisBlock() *core.Genesis {
	genesis := &core.Genesis{}
	f, err := os.Open("./genesis/mainnet-genesis.json")
	if err != nil {
		panic(err)
	}
	if err := json.NewDecoder(f).Decode(genesis); err != nil {
		panic(err)
	}

	return genesis
}
