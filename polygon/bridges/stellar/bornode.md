# Running a Bor node

You need a Bor node on the Mumbai testnet for the bridge to communicate with.

## Running a light client

One option is to use docker:

```sh
docker run -v $(pwd)/bordatadir:/datadir -p 8546:8546 maticnetwork/bor:v0.2.17 bor --syncmode=light --bor-mumbai  --datadir /datadir --ws --ws.addr 0.0.0.0 --bootnodes "enode://320553cda00dfc003f499a3ce9598029f364fbb3ed1222fdc20a94d97dcc4d8ba0cd0bfa996579dcc6d17a534741fb0a5da303a90579431259150de66b597251@54.147.31.250:30303"
```

## Running a full heimdall+Bor node

Bootnodes and snapshots can be found on <https://monitor.stakepool.dev.br/snapshots>.

Snapshots are also available at <https://snapshot.polygon.technology> and <https://forum.polygon.technology/t/updated-snapshots-for-mainnet-and-mumbai/9564>.

### Heimdall

Heimdall needs an Ethereum rpc endpoint, public ones for mainnet and Goerli can be found on < <https://www.allthatnode.com/ethereum.dsrv>>
