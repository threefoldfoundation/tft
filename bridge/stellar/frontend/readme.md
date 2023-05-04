# ThreeFold - Binance Smart Chain Dapp

Decentralised application for TFT on Binance Smart Chain (TESTNET).

## Run locally

copy `.env.testnet` to `.env.development` to use the testnet setting or `.env.production` for the production settings.

```sh
yarn
yarn start
```

## Connect metamask to binance chain testnet

[binance academy](https://academy.binance.com/nl/articles/connecting-metamask-to-binance-smart-chain)

## Open Dapp

Browse to [http://localhost:3000](http://localhost:3000) and connect your wallet, if you have TFT on binance chain, you should see them on the screen.

## Deployed instances

- [production](https://bridge.eth.threefold.io/)
- [testnet](https://bridge.testnet.threefold.io/)

## Development

If you want to run this app to connect to your own bridge setup in development you can change `.env.development` file to the corresponding contract addresses where you TFT contract is deployed.
