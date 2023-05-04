const { ethers } = require("hardhat");

async function main() {
    const w = ethers.Wallet.createRandom()
    console.log(`private key: ${w.privateKey}`)
    console.log(`wallet address: ${w.address}`)
    process.exit(0)
}

main()