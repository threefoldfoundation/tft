# Development

## Testnet

### Ethereum

Deploy the [accountactivation smart contract](../solidity/contracts/accountactivation.sol) on your favorite Ethereum testnet.

The owner and beneficiary are by default set to the address that deploys the contract.

Set the cost for activating a Stellar account by calling the `setNetworkCost` method with network `stellar` and cost `1000000000000000` (= 0.001 eth).

## Stellar

Create an account on the Stellar testnet and fund it with sufficient XLM's by calling friendbot to activate it.
