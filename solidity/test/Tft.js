const { expect } = require("chai");

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
});

    // // Transfer 50 tokens from owner to addr1
    // await tftToken.transfer(addr1.address, 50);
    // expect(await tftToken.balanceOf(addr1.address)).to.equal(50);

    // // Transfer 50 tokens from addr1 to addr2
    // await tftToken.connect(addr1).transfer(addr2.address, 50);
    // expect(await tftToken.balanceOf(addr2.address)).to.equal(50);