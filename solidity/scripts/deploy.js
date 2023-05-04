const hre = require("hardhat");

// scripts/deploy.js
async function main () {
  // We get the contract to deploy
  const Token = await hre.ethers.getContractFactory("contracts/tokenV1.sol:TFT")
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