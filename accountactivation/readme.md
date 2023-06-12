# Account Activation

Activate accounts on the Stellar network by paying for it from Ethereum.

## Ethereum

The [accountactivation smart contract](../solidity/contracts/accountactivation.sol) has an `ActivateAccount` method which takes a network (currently only `stellar`) and an account parameter.

The cost to activate an account needs to be added to the call and is sent to the `beneficiary` address set in the contract. This cost can be requested with the `networkCost` method.

## Stellar

## Transactions

Account activation transactions have a hash memo containing the Ethereum transaction id.  

### Activation account

Stellar account that does the activation of accounts with 2 XLM as a starting balance.

Should the secret of this account leak, it can be replaced if the accountactivation smart contract is redeployed so previous activations are not redone on accounts that are merged already.
