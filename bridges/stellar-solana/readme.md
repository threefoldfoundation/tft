# EVM chain bridge

## Token contract

For TFT as a btoken on an EVM chain, see the [tft readme](../../readme.md).

Supported networks:

- Ethereum mainnet
- Ethereum sepolia
- Ethereum goerli
- Binance smart chain mainnet
- Binance smart chain testnet

### Token contract security

The token contract functions like:

- `addOwner`
- `removeOwner`
- `upgradeTo`

Must only be calable by a multisig wallet. We can achieve this by doing the following: we deploy the contract and the proxy with a single sig account. We verify that everything is deployed correctly and then we can create a multisig wallet contract. The following contract implementation is proposed: [consensys multisig wallet](https://github.com/ConsenSysMesh/MultiSigWallet).

To most important functions and events of this multisig contract are:

- `SubmitTransaction`: Submits a transaction to a destination contract (in our case it will be our token contract)
- `ConfirmTransaction`: A submitted transaction can be confirmed by the other signers

These function will generate events that we can pick up in our bridge deamon in order to make this depositing / withdrawing from and to stellar an automated system.

We can create a multisig contract deployment with X amount of owners. When the multisig contract is deployed we can make the contract owner of our token contract. When we have verified that the transaction is successful we can start to create a new multisig transaction to remove the initial contract owner (the single signature account). When all the accounts on the multisig wallet have signed this transaction, the initial contract owner will be removed and then only the multisig wallet is an owner of the contract.

A frontend for this multisig wallet can be found here: [gnosis wallet](https://wallet.gnosis.pm/) allows us to create a multisig wallet.

This [guide](https://medium.com/coinmonks/guide-to-using-the-gnosis-multisig-wallet-eth-e76979741162) can help building and sending multisig transactions.

## Bridge Concept

We want to bridge Stellar TFT to following smart chains:

- Ethereum
- Binance Chain
- Huobi Eco Chain
- ..

To do this we need some sort of bridge that can mint tokens on the target smart chain. As explained in the contract section, we can mint tokens on a smart chain by calling the contract `mint` function.

## Bridge

The bridge is a daemon that has 2 different running modes:

- Master
- Follower

### Bridge mode

To improve security because we are dealing with user funds we have chosen to work with Multisignature transactions on both Stellar and the target Smart Chain.

A master bridge is a bridge that will initiate all transactions (deposits/withdraws) and will wait for signatures / confirmation of follower (signer) bridges. All bridges (master/followers) will run with a key that is part of the multisig contract on the target smart chain.

### It scans its stellar address to look for incoming transactions

The bridge monitors a central stellar account that is governed by the threefoldfoundation. When a user sends an amount of TFT to that stellar account, the bridge will pick up this transaction. In the memo text of this transaction is the base64 encoded smart chain address of the receiver (which is first hex decoded).

The bridge checks the amount that are transferred and the target on the smart chain and mints the tokens on the smart chain accordingly. To mint, the bridge calls the `mint` function on the smart contract.

There is replay protection in place, a mint is based on the following:

- Transaction ID (UNIQUE!)
- Target Smart Chain address
- Value (number of tokens)

When the mint occurs, the transaction ID is saved to the contract's storage. It also checks if this transaction ID exists, if it exists the contract does not mint. This is the counter double mint.

Flow: a user will deposit funds into the master bridge wallet, this wallet is [Multignature Stellar Wallet](https://developers.stellar.org/docs/glossary/multisig/). The master will initiate the minting transaction on the smart chain by calling the multisig contract `SubmitTransaction` call with the encoded `Mint` call of our token contract. Follower bridges will listen to Submission events on the multisig contract and confirm the transaction accordingly. Once enough confirmations have been submitted, the multisig contract will call the token contract `Mint` function and the funds will be minted on the target smart chain.

### It reads events from the contract and looks for `withdraw` events

When a user on the smart chain interacts with the smart contract `withdraw` function, the bridge will pick up this event and start a withdrawal from the smart chain back to Stellar.

A withdraw event needs the following information:

- An amount of tokens
- A target blockchain address (Stellar address at first)
- A target blockchain network (`testnet` for example)

Flow: a user will call the token contract `Withdraw`, the Master bridge will initiate a transaction on the Multisignature Stellar Wallet and ask the followers over libp2p for their signatures. When enough signatures are met for a payment operation the transaction will be submitted to the network.

## Running the bridge

### Geth light client

Not supported for now, only full nodes can be used

### Build and run

To Build the bridge see the [buildinstructions](./building.md).

Following parameters can be set when starting the bridge:

| Name          | Description                         | Default                                           |
| ------------- | ------------------------------------ | ------------------------------------------------- |
| --secret      | one of X stellar multisignature keys |
| --account     | json key generated by geth           |
| --password    | json key password                    |
| --eth         | Smart chain client url               | `https://data-seed-preeth-1-s1.binance.org:8545`/ |
| --eth-network | Smart chain network                  | goerli-testnet                               |
| --persistency | Persistency file for the brige       | node.json                                         |
| --contract    | TFT token address on chain           | 0xa8B0DDD11B6Bb53a79E62B8Ae8a1e2f68cd75338        |
| --mscontract  | Multisig token address on chain      | 0x4fD0f6fc13ADFF3D2aAb617702E31c49F715BE32        |
| --follower    | If bridge is follower (signer)       | false                                             |
| --datadir     | Datadir where chain data is stored   | ./storage                                         |

run the bridge with parameters: `./stellar --secret ...`
