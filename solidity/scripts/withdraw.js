const { ethers } = require("hardhat");

// scripts/deploy.js
async function main () {
  const args = process.argv

  if (args.length < 5) {
    console.log("Usage: withdraw.js <token_address> <private key> <provider url> <network id> <stellar address> <amount>")
    console.log("Common networkIds:  mainnet: 1, goerli-testnet: 5, sepolia-testnet: 11155111")
    process.exit(1)
  }

  const tokenAddress = args[2]
  const privateKey = args[3]
  console.log("privateKey=", privateKey)

  const providerUrl = args[4]
  console.log("providerUrl=", providerUrl)

  const networkId = parseInt(args[5])
  console.log("networkId=", networkId)

  const stellarAddress = args[6]
  const amount = parseInt(args[7])

  // The address from the above deployment example
  // let contractAddress = "0x2bD9aAa2953F988153c8629926D22A6a5F69b14E";
  const provider = new ethers.providers.WebSocketProvider(providerUrl, networkId)

  console.log(await provider.getNetwork())

  const wallet = new ethers.Wallet(privateKey).connect(provider);

  // We get the contract to deploy
  const Token = await ethers.getContractFactory("contracts/tokenV1.sol:TFT", wallet)
  const token = Token.attach(tokenAddress)


  console.log("signer=", token.signer.address)
  const balance = await token.balanceOf(token.signer.address)
  console.log("balance=", balance.toString())

  await token.withdraw(amount, stellarAddress, "stellar")
}
  
  main()
    .then(() => process.exit(0))
    .catch(error => {
      console.error(error);
      process.exit(1);
    });