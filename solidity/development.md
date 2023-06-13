# Development

## Prerequisites

- Solidity compiler (solc): [Installing the Solidity Compiler](https://docs.soliditylang.org/en/develop/installing-solidity.html#binary-packages)

## Install dependencies

```sh
yarn
```

## Generating ABI

```sh
npx hardhat compile
yarn run hardhat export-abi
```

Abis will be

## Deploying on local chain

In one shell

```shell
npx hardhat node
```

In another shell:

```sh
npx hardhat run scripts/deploy.js
```

## Deploying on a public chain

```sh
node scripts/deployProd.js <private key> <provider url> <network id>
```

The provider url is the ws or wss url of the Ethereum node you are connecting to.

Network id:

- mainnet: 1
- goerli-testnet: 5
- sepolia-testnet: 11155111

## Testing

```sh
npx hardhat test
```
