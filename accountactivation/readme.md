# Account Activation

Activate accounts on the Stellar network by paying for it from Ethereum.

## Stellar

## Transactions

Account activation transactions have a hash memo containing the Ethereum transaction id.  

### Activation account

Stellar account that does the activation of accounts with 2 XLM as a starting balance.

Should the secret of this account leak, it can be replaced if the accountactivation smart contract is redeployed so previous activations are not redone on accounts that are merged already.
