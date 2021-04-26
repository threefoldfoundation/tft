# TFT on Binance Smart Chain

Token address on Binance Smart Chain Testnet: https://testnet.bscscan.com/address/0x770b0AA8b5B4f140cdA2F4d77205ceBe5f3D3C7e

You can add the token address to a Web3 compatible wallet. If you are using Metamask, check out following [guide](https://academy.binance.com/en/articles/connecting-metamask-to-binance-smart-chain) on how to connect Metamask to Binance Smart Chain Testnet or Mainnet.

If you have setup Metamask for Binance Chain Testnet, use the [bnb-faucet](https://testnet.binance.org/faucet-smart) to fund your account with some Testnet BNB's.
(You need BNB in order to deploy the contract on BSC)

## Deploy the contract yourself.

You can use [remix](https://remix.ethereum.org/#optimize=false&runs=200&evmVersion=null&version=soljson-v0.8.3+commit.8d00100c.js) to compile and deploy the Token contract on any EVM compatible Smart Chain.

### 1: Upload the source code to Remix

Upload the [contract](../solidity/contract) folder to the remix Editor.

### 2: Compile the token contract

Select `tokenV0.sol` and switch to the 2nd tab in left Menu. Execute `compile` action.

### 3: Deploy token contract

If you compiled the token contract, select the 3th tab in left Menu and select following:

- Use `Injected WEB3` if using metamask
- In the contract dropdown, select `TFT - tokenV0.sol`
- Click `deploy`
- Execute transaction on metamask

Copy the contract address (Click on the copy button in the deployed contracts section)

Now browse to [bscscan-testnet](https://testnet.bscscan.com/) and search for your deployed contract.

### 4: (optional) Deploy proxy contract

Select `proxy.sol` in the source file editor tab, change [this](../solidity/contract/proxy.sol#L30) with your deployed Token contract address.

### 5: Compile the proxy contract

Select `proxy.sol` and switch to the 2nd tab in left Menu. Execute `compile` action.

### 6: Deploy proxy contract

If you compiled the proxy contract, select the 3th tab in left Menu and select following:

- Use `Injected WEB3` if using metamask
- In the contract dropdown, select `Proxy - proxy.sol`
- Click `deploy`
- Execute transaction on metamask

Copy the contract address (Click on the copy button in the deployed contracts section)

Now browse to [bscscan-testnet](https://testnet.bscscan.com/) and search for your deployed contract.

### 7: (Optional) deploy multisig contract

Upload the [contract](../solidity/multisig) folder to the remix Editor.

### 8: Compile the multisig contract

Select `MultiSigWallet.sol` and switch to the 2nd tab in left Menu. Execute `compile` action.

### 9: Deploy multisig contract

If you compiled the multisig contract, select the 3th tab in left Menu and select following:

- Use `Injected WEB3` if using metamask
- In the contract dropdown, select `MultiSigWallet - MultiSigWallet.sol`
- Click on the chevron to the right of deploy
- Fill in the owner addresses in format ["address1", "address2", "...", ...] and fill in the required amount of signatures
- Click `deploy`
- Execute transaction on metamask

Copy the contract address (Click on the copy button in the deployed contracts section)

Now browse to [bscscan-testnet](https://testnet.bscscan.com/) and search for your deployed contract.

### 10: Make multisig contract owner of the Token contract.

Copy the multisig contract address, click on the 3th tab in the remix editor and click on the token contract implementation in the section `deployed contracts`.

Execute the transaction `addOwner` and provide the multisig contract address and click `transact`.