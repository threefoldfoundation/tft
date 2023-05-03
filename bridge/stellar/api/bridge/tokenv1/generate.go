package tokenv1

//go:generate mkdir -p tmp
//go:generate solc --abi --overwrite ../../../../../solidity/contract/tokenV1.sol -o tmp
//go:generate sh -c "jq . tmp/TFT.abi > tokenv1.abi"
//go:generate rm -rf tmp
//go:generate abigen --abi tokenv1.abi --pkg tokenv1 --type Token --out token.go
