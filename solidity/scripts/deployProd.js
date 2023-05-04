const { ethers } = require("hardhat");

async function main () {
  const args = process.argv

  if (args.length < 5) {
    console.log("Usage: deployProd.js <private key> <provider url> <network id>")
    process.exit(1)
  }

  const privateKey = args[2]
  

  const providerUrl = args[3]
  console.log("providerUrl=", providerUrl)

  const networkId = args[4]
  console.log("networkId=", networkId)

  const provider = new ethers.providers.WebSocketProvider(providerUrl, 5)
  
  console.log(await provider.getNetwork())

  const wallet = new ethers.Wallet(privateKey).connect(provider);
  const balance = await wallet.getBalance()

  console.log(`Wallet balance: ${ethers.utils.formatEther(balance)}`)
  // We get the contract to deploy
  const Token = await ethers.getContractFactory("contracts/tokenV1.sol:TFT", wallet)
  const token = await Token.deploy()
  await token.deployed()
  console.log('Token deployed to:', token.address);
}
  
  main()
    .then(() => process.exit(0))
    .catch(error => {
      console.error(error);
      process.exit(1);
    });