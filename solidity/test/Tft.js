const { expect } = require("chai");
const { ethers } = require("hardhat");
const { signHash } = require("./utils");

describe("Token contract", function () {
  let tftToken

  // First deploy token contract
  before("deploy contract instance", async function (){
    const Token = await hre.ethers.getContractFactory("contracts/tokenV1.sol:TFT")
  
    tftToken = await Token.deploy();
    await tftToken.deployed()
  })

  it("Should be able to set owner", async function() {
    const [owner, addr1] = await ethers.getSigners();

    await tftToken.addOwner(addr1.address);
    expect(await tftToken.owner_list, [owner.address, addr1.address]);
  });

  it("Should be able to remove an owner", async function() {
    const [owner, addr1] = await ethers.getSigners();

    await tftToken.removeOwner(addr1.address);
    expect(await tftToken.owner_list, [owner.address]);
  });

  it("Should be able to set signers", async function() {
    const [owner, addr1, addr2] = await ethers.getSigners();

    await tftToken.setSigners([owner.address, addr1.address, addr2.address], 3);
    expect(await tftToken.getSigners(), [owner.address, addr1.address, addr2.address]);
  });

  it("Should be able to mint", async function() {
    const [owner, addr1, addr2, addr3] = await ethers.getSigners();

    await tftToken.setSigners([owner.address, addr1.address, addr2.address], 3);
    expect(await tftToken.getSigners(), [owner.address, addr1.address, addr2.address]);

    let abiEncoded = ethers.utils.defaultAbiCoder.encode(["address", "uint256", "string"], [addr3.address, 100, "sometxid"]);
    let digest = ethers.utils.keccak256(abiEncoded);

    owner.sign

    let [sig1, sig2, sig3] = await signHash([owner, addr1, addr2], digest);

    await tftToken.mintTokens(addr3.address, 100, "sometxid", [sig1, sig2, sig3]);
    expect(await tftToken.balanceOf(addr3.address)).to.equal(100);
  });
});

    // // Transfer 50 tokens from owner to addr1
    // await tftToken.transfer(addr1.address, 50);
    // expect(await tftToken.balanceOf(addr1.address)).to.equal(50);

    // // Transfer 50 tokens from addr1 to addr2
    // await tftToken.connect(addr1).transfer(addr2.address, 50);
    // expect(await tftToken.balanceOf(addr2.address)).to.equal(50);