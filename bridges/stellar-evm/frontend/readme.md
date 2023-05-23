# ThreeFold - Ethereum Dapp

Decentralised application for TFT on Ethereum (TESTNET).

## Run locally

copy `.env.testnet` to `.env.development` to use the testnet setting or `.env.production` for the production settings.

```sh
yarn
yarn start
```

If you're having node > v16, `export NODE_OPTIONS=--openssl-legacy-provider` first as webpack 4 uses node's md4 hashes.

## Open Dapp

Browse to [http://localhost:3000](http://localhost:3000) and connect your wallet, if you have TFT on the Ethereum chain, you should see them on the screen.

## Deployed instances

- [production](https://bridge.eth.threefold.io/)
- [testnet](https://bridge.testnet.threefold.io/)

## Development

If you want to run this app to connect to your own bridge setup in development you can change `.env.development` file to the corresponding contract addresses where you TFT contract is deployed.
