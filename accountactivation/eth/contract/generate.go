package contract

// The go binding is generated from the abi which needs to be created first
// see ../../../solidity/development.md

//go:generate abigen --abi ../../../solidity/abi/contracts/accountactivation.sol/accountactivation.json --pkg contract --type AccountActivation --out accountactivation.go
