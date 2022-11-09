# Development

## Testnet

Polygon's testnet is called Mumbai

As block explorer for the Mumbai testnet, you can use <https://mumbai.polygonscan.com>.

### Connect to Polygon/Matic's Testnet Mumbai using Metamask

- [Install Metamask](https://metamask.io/download/).
- Open Metamask and select Custom RPC from the networks dropdown.

    ![Metamask custom RPC network](https://user-images.githubusercontent.com/56790126/198002275-41984b4c-aaf0-4ade-8ab2-2c5bb34b9a87.png)

- Put in a Network name - “Matic Mumbai Testnet”
- URL field: "https://rpc-mumbai.maticvigil.com"
- Chain ID: 80001
- (Optional) Currency Symbol: "MATIC"
- (Optional) Block Explorer URL: "https://mumbai.polygonscan.com/"
- Click save

#### Get some testnet Matic

There is a [faucet available](https://faucet.polygon.technology) where you can get some Matic tokens to use during development or testing.

### Deploy the contract yourself

You can use [remix](https://remix.ethereum.org/#optimize=false&runs=200&evmVersion=null&version=soljson-v0.8.3+commit.8d00100c.js) to compile and deploy the Token contract on any EVM compatible Smart Chain. You can try it out on Polygon's testnet: Mumbai.

#### 1: Upload the source code to Remix

Upload the [contract](../solidity/contract) folder to the remix Editor.

#### 2: Compile the token contract

Select `tokenV1.sol` and switch to the 2nd tab in left Menu. Execute `compile` action.

#### 3: Deploy token contract

If you compiled the token contract, select the 3th tab in left Menu and select following:

- Use `Injected Provider - Metamask`
- In the contract dropdown, select `TFT - tokenV1.sol`
- Click `deploy`
- Confirm transaction on Metamask

Copy the contract address (Click on the copy button in the deployed contracts section)
Now browse to [polygonscan](https://mumbai.polygonscan.com/) and search for your deployed contract.

#### 4: (optional) Deploy proxy contract

Select `proxy.sol` in the source file editor tab, change [this](../solidity/contract/proxy.sol#L30) with your deployed Token contract address.

#### 5: Compile the proxy contract

Select `proxy.sol` and switch to the 2nd tab in left Menu. Execute `compile` action.

#### 6: Deploy proxy contract

If you compiled the proxy contract, select the 3th tab in left Menu and select following:

- Use `Injected Provider - Metamask`
- In the contract dropdown, select `Proxy - proxy.sol`
- Click `deploy`
- Execute transaction on Metamask

Copy the contract address (Click on the copy button in the deployed contracts section)
Now browse to [polygonscan](https://mumbai.polygonscan.com/) and search for your deployed contract.
