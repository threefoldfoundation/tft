# Development Stellar-Polygon bridge

## Running a Bor node

You need a [Bor node](./bornode.md) on the Mumbai testnet for the bridge to communicate with.

## Stellar testnetaccounts

The bridge needs an activated account on the Stellar testnet with a trustline to [testnet TFT](https://github.com/threefoldfoundation/tft-stellar/blob/master/development.md#tft-on-stellar-testnet)as the bridge vault address.

For each signer, another activated account on the Stellar testnet is required with  a data entry with key `PolygonAddress` and as value the signer's polygon address.
