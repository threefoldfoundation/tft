# Account Activation

Activate accounts on the Stellar network by paying for it from Ethereum.

## Ethereum

Contract address: [0xE04a9665bbA9B7954572802A9864dD1d03326792](https://etherscan.io/address/0xE04a9665bbA9B7954572802A9864dD1d03326792)

The [accountactivation smart contract](../solidity/contracts/accountactivation.sol) has an `ActivateAccount` method which takes a network (currently only `stellar`) and an account parameter.

The cost to activate an account needs to be added to the call. This cost can be requested with the `networkCost` method.

### beneficiary

When the `activateAccount` method is called, the cost is sent to the `beneficiary` address set in the contract.

This way the contract itself does not pile up eth which then has to be withdrawn by the owner ( which can be a multisig contract) or an allowed withdrawer, making the process easier and saving gas for the operating the service.

## Stellar

## Transactions

Account activation transactions have a hash memo containing the Ethereum transaction id.  

### Activation account

Stellar account that does the activation of accounts with 2 XLM as a starting balance.

Should the secret of this account leak, it can be replaced if the accountactivation smart contract is redeployed so previous activations are not redone on accounts that are merged already.
