# Binance Smart Chain node Docker image

## Build

## BSC mainnet

```sh
docker build -t bscchain-mainnet .
```

### BSC testnet

```sh
docker build -t bscchain-testnet --build-arg network=testnet .
```

## Run

Assuming there is a `data/bsctestnet` directory where you execute this and are running a testnet node:

```sh
docker run -v $(pwd)/data/bsctestnet:/storage --name binance-smart-chain-node \
-p 127.0.0.1:8545:8545 -p 127.0.0.1:8546:8546 -p 127.0.0.1:6060:6060 -p 30311:30311 -p 30311:30311/udp \
bscchain-testnet --syncmode light --cache 4096 --ws
```

Blockchain data will be stored at `./data/bsctestnet` folder.

`config.toml` will be created if it does not exist.

## Check sync status

```sh
docker exec binance-smart-chain-node bsc attach --exec eth.syncing

docker logs -f binance-smart-chain-node
```

## JSONRPC

* HTTP JSONRPC at port 8545
* WebSocket at 8546
* IPC (unix socket) at /data/bsc/.ethereum/geth.ipc

Test it using [geth_linux](https://github.com/binance-chain/bsc/releases) binary:

```sh
geth_linux attach http://localhost:8545
geth_linux attach ws://localhost:8546
geth_linux attach /data/bsc/.ethereum/geth.ipc
# Last one needs root privileges
```

## More info

[original BSC repo](https://github.com/binance-chain/bsc)
