const { ethers } = require("hardhat");

// scripts/deploy.js
async function main () {
  // We get the contract to deploy
  const Token = await ethers.getContractFactory("contracts/tokenV1.sol:TFT")
  const token = await Token.deploy()
  await token.deployed()
  console.log('Token deployed to:', token.address);

  const network = await ethers.getDefaultProvider().getNetwork();
  console.log("Network name=", network.name);
  console.log("Network chain id=", network.chainId);

  const [owner, addr1, addr2] = await ethers.getSigners();
  await token.setSigners([owner.address, addr1.address, addr2.address], 3);
}
  
  main()
    .then(() => process.exit(0))
    .catch(error => {
      console.error(error);
      process.exit(1);
    });