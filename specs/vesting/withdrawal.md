# Withdrawal from the vesting escrow account

When a person with staked TFT wants to withdraw some of those TFT
(provided they have been unlocked), a multisignature tx is required. The
person withdrawing the funds must prepare the stellar transction. This
transaction is then submitted on the substrate based chain (grid-db). Once
the transaction is submitted, the validators need to reach consensus on
the substrate based blockchain.

- When a transaction is submitted, it is saved in storage
- We keep track of transactions that are being voted on. This way, we can prevent more than 1 withdrawal transaction from being active on the chain at the same time for the same source escrow account
- An event is emitted when a new transaction is submitted. This way validator daemons can easily pick up new transactions.
- The validator spots the tx, and it verifies that executing the transaction would not reduce the funds on the escrow below the amount that is still locked.
- If the validator agrees, he makes a signature for the transaction. This signature is saved on chain, and an event is emitted.
- Once sufficient events have been emitted, validators can pick up the transaction, assemble the needed amount of signatures, and start to attempt to submit the transaction to the stellar network.
- As soon as one of the validators succeeds in submitting the transaction, he marks the transaction as "done". This removes the transaction and associated signatures from storage, and adds an
entry in an "escrow withdrawal" list (so we can track which txes
have been done in the past). This also generates an event, so other
validators know they can stop trying to submit the transaction.
- After X amount of blocks (lets say 1000 -> 1hour at default 6 second
block time), the chain checks if the tx has enough singatures. If it
does not have sufficient signatures, the transaction is removed from
storage. It can optionally be submitted again to start a new vote.
This way we ensure runtime storage is not bloated with garbage
transactions.

Because it takes substrate TFT to make extrinsics, the validators could
expose a small API which allows submitting the withdrawal tx on the
substrate chain. This way the client does not need access to a substrate
wallet.
