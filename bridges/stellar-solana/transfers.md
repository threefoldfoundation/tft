# Transferring TFT between Stellar and Solana

## From Solana to Stellar

The bridge monitors the `Mint` account and looks for transactions with the following
format:

- 2 or 3 operations
- one of the operations invokes the MemoProgram
- one of the operations invokes the Token-2022 program with a BurnToChecked instruction
- If there are 3 operations, the 3rd operations invokes the `compute-budget` program

The instructions are then parsed and processed by the bridge. In order to withdraw
tokens from Solana to Stellar, the following format must be followed:

- Add the destination account which will receive the stellar TFT as Memo. There
  is no additional encoding.
- Burn the amount of tokens which must be withdrawn. In a BurnToChecked, the amount
  of decimals used in the token must also be provided.
- Optionally set a custom compute-budget limit

> WARNING: If the destination account does not exist, or does not have a TFT trustline,
this will result in a loss of tokens (though the transaction can be picked up later
if the bridge is run from scratch).

## From Stellar to Solana

Transfer the TFT to the bridge address with the target address in the MEMO_HASH.

### Solana address encoding

A Solana address in readable form is simply a base58 encoded 32byte public key.
Because this can't be specified in a MEMO_TEXT field, the address must instead
be decoded to the raw bytes, which can then be set as MEMO_HASH. Depending on the
tooling/libraries used to create the stellar transaction, you might have to encode
this raw value to set it.

## Fees

- From Stellar to Solana:

  To cover the costs of the bridge, a default fee of 50 TFT is charged. This fee can be modified if it does not cover the gas price for the bridge.

  Make sure the amount received on the bridge's Stellar address is larger than the Fee.

- From Solana to Stellar:

  a fee of 1 TFT is deducted from the withdrawn amount

## Refunds

When the supplied memo text of a deposit transaction can not be decoded to a valid
Solana address, the deposited TFT's are sent back minus 1 TFT to cover the transaction
fees of the bridge and to make a DOS attack on the bridge more expensive. This is
also the case if the deposit amount is not large enough to cover the bridge fees.
