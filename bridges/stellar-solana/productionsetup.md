# Set up the bridge for production

The bridge operates by `vaulting` tokens on the stellar side, and minting new tokens
on the solana side. Thus, the bridge requires a "vault account" on stellar, and
the ability to sign for the `minting authority` on the chosen Solana `mint`. For
production usecases it is highly recommended to run the bridge in a multisig setup.
To do so, we need both a multisig stellar vault, and a multisig mint authority on
Solana. Additionally, the bridge collects fees which are used to cover the expenses
of the actual bridging operations. These fees are collected in a stellar account,
which we will refer to as the `fee wallet`. The bridge only sends TFT to the fee
wallet so we don't need the ability to sign for this. It merely needs a TFT trustline
to receive the fees.

## Stellar setup

### Vault setup

A Stellar account is required for the bridge to receive and hold the Stellar TFT's
on. There are 2 slightly different ways to set up this vault account. In the first
case, the account is one of the multisig signers. In the second case, the account
key's signing weight is set to 0, so the account itself cannot sign. Both cases
are supported. In the first case, the account itself must be created by one of the
cosigners, and be used as the accout on one of the bridge nodes. In the second case,
anyone can set up the account (though all bridge nodes still need to run accounts
which can actually sign). It is highly recommended to go with the second setup,
as this is the __recommended__ way to set up and manage a multisig stellar wallet.

All cosigners need to create a stellar account, and fund it. They then provide the
public key of this account to whoemever sets up the vault account (the used accounts
can be existing accounts used for other purposes, though for security reasons its
recommended to use a dedicated account for signing). **IMPORTANT: the private keys
of these accounts must obviously be kept secret.** 

There also needs to be agreement upon the siginnig threshold: the minimum amount
of signatures required to sign a transaction (i.e. the amount of bridge nodes which
have to agree). There is no strict requirement here, but it is highly recommended
to: require more than half the amount of signers (which implies a majority agreement
for transactions), and to not have the threshold equal the amount of signers, as
to avoid loss of funds when something happens to these keys.

Once all these accounts are created, the person who sets up the vault account aggregates
their addresses. They can then setup the vault account as follows (this is scenario
2 where the vaults own key is set to weight 0):

- Create a new stellar account (keypair).
- Fund this account
- Add a trustline for TFT
- Add all signers, with weight 1
- Set the signing thresholds __and set the master key weight to 0 __

> NOTE: You can use stellar.expert to verify the account is properly configured at
this stage.

> NOTE: If the vaults own key is set to weight 0, the private key can now be discarded.

### Fee Wallet

The fee wallet can be any wallet with a trustline to TFT, and can be set up as required.
It is recommended to use a company managed wallet for this. All bridges can currently
use the same fee wallet, and probably should to reduce management overhead.

## Solana setup

The bridge will mint new tokens on Solana (a token on Solana is also referred to
as a `Mint`), and listen for `Burn` events on this token (see [the guide on how
to use the bridge](./transfers.md) for more info). This requires the bridge to be
able to sign on behalf of the token's `Mint authority`. The bridge uses the `Token-2022`
program on solana, which has multisig capabilities built in. For convenience, it
is recommended to have a single account setup the token first, generate a multisig
account with all cosigners, and then transfer the ownership of the token to this
multisig account.

We will use the official solana CLI tooling to genearte the accounts and set up
the token.

### Requirements

You will need the following tools available on your path (refer to the official
Solana docs for info on how to install them if that hasn't happened yet).

- `solana`
- `solana-keygen`
- `spl-token`

### Setting up the Mint

We will now set up an appropriate `Solana Mint`. This can be skipped if a token
already exists on the Solana network.

> When generating new accounts on Solana, it is possible to `grind` for a `vanity
address`, which contains a certain string. Since this effectively brute forces keys
until one is found with the matching string, longer strings require exponentially
more time. 

#### Create the Mint

Firstly, the person setting up the `Mint` must have an account on the Solana network
which is funded with SOL's (it seems 1 is enough for the initial setup). If this
is not yet the case, it should be done now. The `Mint` is set up at an address,
which requires a keypair. This address is the full address of the `Mint`, and it
is highly recommended to use a vanity keypair for this. While the vanity address
is not strictly required, the fact that we use a metadata-pointer does require to
know the address of the `Mint` up front, so we **MUST** have a generated keypair.
With the keypair present locally, create the `Mint` as follows:

```sh
spl-token create-token --program-2022 --metadata-address TFTqrVLZAMFfbC4bEpfiN1PRTfcYiuHheTs9d297uk6 TFTqrVLZAMFfbC4bEpfiN1PRTfcYiuHheTs9d297uk6.json --decimals 7
```

It is assumed that `TFTqrVLZAMFfbC4bEpfiN1PRTfcYiuHheTs9d297uk6.json` is a keypair
file created by `solana-keygen` which is present in the local directory. `--program-2022`
instructs the CLI to use the `token-2022` program, which is more modern and has
additional features compared to the original token program. `metadata-address TFTqrVLZAMFfbC4bEpfiN1PRTfcYiuHheTs9d297uk6`
initialized the metadata pointer to point the the account holding the `Mint` itself.
This is a recommended setup to avoid having to manage 2 separate accounts, and
causes the metadata which we will set next to be stored in the same account as
the token `Mint`. Lastly, `--decimals 7` is important to initialize the token with
the same amount of decimals (7) as the token on stellar, for the 1 to 1 mapping
(by default tokens have 9 decimals on Solana).

#### Initialize the metadata

Now the metadata can be initialized. Using the same `Mint` address from before
(notice we are only providing the address, not the file as there is no `.json`
extension):

```sh
spl-token initialize-metadata TFTqrVLZAMFfbC4bEpfiN1PRTfcYiuHheTs9d297uk6 $TOKEN_NAME $TOKEN_SYMBOL $METADATA_URL
```

`TOKEN_NAME` is the (full) name for the token, e.g. "ThreeFold Token"
`TOKEN_SYMBOL` is the symbol (short notation) of the token, e.g. "TFT"
`METADATA_URL` is an optional URL where a `metadata.json` file can be found with
  some additional info, such as a token image. Conforming explorers can use this
  to display additional info, e.g. the solana explorer will load the image provided
  by the url in this file and display it on the token page. 

The metadata can be changed at a later point in time. Additional custom fields can
also be set should this be required. An example `metadata.json` file for our token
could be:

```json
{
  "name": "ThreeFold Token",
  "symbol": "TFT",
  "description": "TFT on solana",
  "image": "https://threefoldfoundation.github.io/tft/tft_icon.png"
}
```

#### Creating the multisig address

Similar to stellar, every cosigner needs to have a keypair they will use for signing.
Except for the keypair used by the bridge leader, there is no requirement for these
to be funded. It is highly recommended to use the same threshold as the one on stellar
though this is not a strict requirement. When all addresses and the threshold are
known, the multisig can be created. It is optionally possible to grind for a vanity
address for the multisig account, which can then be used by also providing the `--address-keypair`
flag to the command.

```sh
spl-token --program-2022 create-multisig $THRESHOLD $ADDRESS_1 $ADDRESS_2 ... $ADDRESS_N --address-keypair $VANITY_ADDRESS_KEYPAIR_FILE
```

This command will print the multisig account which has been set up. If no keypair
is provided, you will need to note this for the next commands.

#### Transfering ownership of the Mint to the multisig address

Now that we have a `Mint` with the right `Metadata`, and a multisig address, we
will tranfer the authorities from the person who set up the `Mint` to this multisig.
Since we have 3 authorities (update, mint, and metdata-pointer), we will need to
do 3 calls. These are

```sh
spl-token --program-2022 authorize $MINT_ADDRESS mint $MULTISIG_ADDRESS
```
```sh
spl-token --program-2022 authorize $MINT_ADDRESS metadata $MULTISIG_ADDRESS
```

```sh
spl-token --program-2022 authorize $MINT_ADDRESS metadata-pointer $MULTISIG_ADDRESS
```

Once this is done, the `Mint` is fully configured with all authorities being the
multisig address.
