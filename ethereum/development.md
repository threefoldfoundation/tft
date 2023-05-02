# Testnet

Ethereum Testnet: [Sepolia](https://ethereum.org/en/developers/docs/networks/#sepolia)

## Run a Sepolia node with docker

Based on <https://docs.chain.link/chainlink-nodes/resources/run-an-ethereum-client/>
and <https://github.com/xhyumiracle/eth2launch>

```sh
docker pull ethereum/client-go:latest
mkdir -p .geth-sepolia 
docker run --name eth -p 8546:8546 -v $(pwd)/.geth-sepolia:/geth -it ethereum/client-go --sepolia --ws --ipcdisable --ws.addr 0.0.0.0 --ws.origins="*" --datadir /geth
```
