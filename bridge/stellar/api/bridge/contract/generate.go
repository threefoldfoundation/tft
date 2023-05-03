package contract

//go:generate mkdir -p tmp
//go:generate solc --abi --overwrite ../../../../../solidity/contract/tokenV0.sol -o tmp
//go:generate sh -c "jq . tmp/TFT.abi > TFT.abi"
//go:generate rm -rf tmp
//go:generate abigen --abi TFT.abi --pkg contract --type Token --out token.go
