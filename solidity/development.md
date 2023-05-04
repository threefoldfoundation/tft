# Development

## Prerequisites

- Solidity compiler (solc): [Installing the Solidity Compiler](https://docs.soliditylang.org/en/develop/installing-solidity.html#binary-packages)

## regenerate setup,svg

```sh
dot -Tsvg setup.dot -o setup.svg
```

## Testing

Try running some of the following tasks:

```shell
npx hardhat help
npx hardhat test
REPORT_GAS=true npx hardhat test
npx hardhat node
npx hardhat run scripts/deploy.js
```
