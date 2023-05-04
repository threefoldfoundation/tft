const { ethers } = require("hardhat");

async function signHash(signers, hash) {
  let sigs = [];

  for (let i = 0; i < signers.length; i = i + 1) {
    const sig = await signers[i].signMessage(ethers.utils.arrayify(hash));

    const splitSig = ethers.utils.splitSignature(sig);
    sigs.push({ v: splitSig.v, r: splitSig.r, s: splitSig.s });
  }

  return sigs;
}

module.exports = {
  signHash,
}