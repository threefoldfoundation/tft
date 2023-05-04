# Development

## Prerequisites

- Solidity compiler (solc): [Installing the Solidity Compiler](https://docs.soliditylang.org/en/develop/installing-solidity.html#binary-packages)

## Install depdencies

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
```
npx hardhat run scripts/deploy.js
```

## Testing

```sh
npx hardhat test
```
