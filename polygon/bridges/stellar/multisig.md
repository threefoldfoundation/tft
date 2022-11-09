# Multisig

For obvious security reasons, the bridge is deployed as multiple instances on multiple servers each with their own signing keys. A majority of the signatures is required for the bridge accounts to mint on Polygon or to transfer on Stellar.

## Stellar multisig

This is easy. The bridge leader fetches the signers and the medium treshold from the bridge account and gathers signatures from the cosigners before it submits the transaction envelope to the Stellar network.

## Polygon multisig

This is not so easy. While a solid solution would be to add an external multisig contract as owner to the TFT contract, this has several downsides:

- Every signer's Polygon account constantly needs to be funded with enough Matic.
- Expensive in transaction fees.
- When the set of signers changes, a new multisigcontract needs to be constructed to replace the old set ( manual gathering of accounts and replacement of the bridge owner by a manual multisig owner).

Another solution is to link a Polygon address to the signer's Stellar account using a data-entry and have these addresses available as **Signers** in the contract.
The leaders collects the signatures from the cosigners and invokes the mint function. The mint function then validates the supplied signatures.

This does mean that the accounts of the signers on the bridge Stellar account have to be propagated to the TFT contract which is currently done hrough a `SetSigners` function only callable by the owner of the token contract.
