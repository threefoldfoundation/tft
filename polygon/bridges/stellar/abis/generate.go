package abis

//go:generate mkdir -p tmp
//go:generate solc --abi --overwrite ../../../../solidity/contract/tokenV0.sol -o tmp
//go:generate mv tmp/TFT.abi .
//go:generate rm -rf tmp
