# Development Stellar-Polygon bridge

## Running a Bor node

You need a Bor node on the Mumbai testnet for the bridge to communicate with.

One option is to use docker:

```sh
docker run -v $(pwd)/bordatadir:/datadir -p 8546:8546 maticnetwork/bor:v0.2.17 bor --syncmode=snap --bor-mumbai --bor.withoutheimdall --datadir /datadir --ws --ws.addr 0.0.0.0 --bootnodes "enode://320553cda00dfc003f499a3ce9598029f364fbb3ed1222fdc20a94d97dcc4d8ba0cd0bfa996579dcc6d17a534741fb0a5da303a90579431259150de66b597251@54.147.31.250:30303,enode://095c4465fe509bd7107bbf421aea0d3ad4d4bfc3ff8f9fdc86f4f950892ae3bbc3e5c715343c4cf60c1c06e088e621d6f1b43ab9130ae56c2cacfd356a284ee4@18.213.200.99:30303,enode://90676138b9823f4b834dd4fb2f95da9f54730a74ff9deb4782c4be98232f1797806a62375d9b6d305af49f7c0be69a9adcad7eb533091bd15b77dd5997b256e2@54.227.107.44:30303"
```

Bootnodes and snapshots can be found on <https://monitor.stakepool.dev.br/snapshots>.

Snapshots are also available at <https://snapshot.polygon.technology> and <https://forum.polygon.technology/t/updated-snapshots-for-mainnet-and-mumbai/9564>.
