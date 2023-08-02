# TFT on Binance Smart Chain

Token address on Binance Smart Chain Testnet: [0xDC5a9199e2604A6BF4A99A583034506AE53F4B34](https://testnet.bscscan.com/address/0xDC5a9199e2604A6BF4A99A583034506AE53F4B34)

You can add the token address to a Web3 compatible wallet. If you are using Metamask, check out following [guide](https://academy.binance.com/en/articles/connecting-metamask-to-binance-smart-chain) on how to connect Metamask to Binance Smart Chain Testnet or Mainnet.

If you have setup Metamask for Binance Chain Testnet, use the [bnb-faucet](https://testnet.binance.org/faucet-smart) to fund your account with some Testnet BNB's.
(You need BNB in order to deploy the contract on BSC)

## Deploy the contract yourself

You can use [remix](https://remix.ethereum.org/#optimize=false&runs=200&evmVersion=null&version=soljson-v0.8.3+commit.8d00100c.js) to compile and deploy the Token contract on any EVM compatible Smart Chain.

### 1: Upload the source code to Remix

Upload the [contract](../solidity/contract) folder to the remix Editor.

### 2: Compile the token contract

Select `tokenV1.sol` and switch to the 2nd tab in left Menu. Execute `compile` action.

**Remark:** Solidity version 0.8.20 introduces the PUSH0(0x5f) opcode which is only supported on ETH mainnet and not on any other chains. Use version 0.8.19 or target the `paris` evm.

### 3: Deploy token contract

If you compiled the token contract, select the 3th tab in left Menu and select following:

- Use `Injected WEB3` if using metamask
- In the contract dropdown, select `TFT - tokenV1.sol`
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

Through Remix

### 7: (Optional) deploy human multisig contract

Configure a multisig contract on <https://app.safe.global/home>

Copy the multisig contract address, click on the 3th tab in the remix editor and click on the token contract implementation in the section `deployed contracts`.

Execute the transaction `addOwner` and provide the multisig contract address and click `transact`.

### Configure the signers for minting

Execute the `setSigners`method through remix or through the human multisig owner contract

## Generating go code based on token / multisig contract ABI

In Remix when you compile a contract you can select ABI button (this will copy the ABI string to your clipboard).

Save the contents to a file.
