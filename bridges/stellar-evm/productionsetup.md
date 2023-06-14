# Set up the bridge for production

## Stellar vault

A Stellar account is required for the bridge to receive and hold the Stellar TFT's on.

Prepare Stellar accounts for the master and the cosigners ( by different people keeping their secrets secret) and add them as signers with weight 1 to the vault account.

If the multisig is defined as a x out of y, make sure no-one can access x secrets, if not add an extra signer, increasing x and y.

if x==y, add at least one extra signer, increasing y. These should not run an active cosigner but if someone loses the signer secret or no longer wants to co-operate, the funds on the Stellar vault are lost.

Set the tresholds of the Stellar account to x and the master weight to 0.
