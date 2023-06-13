# Building the bridge

## local build

This is a normal go project so just execute `go build`.

## Build a docker image

To build a docker image with the latest git tag as version:

```sh
docker build -t tft-stellar-evm-bridge:$(git describe --abbrev=0 --tags | sed 's/^v//')  .
```

## Building the frontend docker image

In the `frontend` folder execute

```sh
docker build -t tft-stellar-evm-bridge-ui:$(git describe --abbrev=0 --tags | sed 's/^v//') . --no-cache
```

## Remarks for building the docker images

On an Apple Silicon chip, add `--platform linux/amd64`

## Publishing the helm charts

Create a folder `packagedcharts` or empty it if it already exists.

Depending on which charts are updated execute the following commands in the `packagedcharts folder:

```sh
helm package ./frontend/helm/bsc-bridge-ui
```

### Update the index

```sh
curl -O https://raw.githubusercontent.com/threefoldfoundation/helmcharts/main/index.yaml
helm repo index . --merge index.yaml --url https://github.com/threefoldfoundation/tft/releases/download/$(git describe --abbrev=0 --tags)
```

Upload the helm package in the release and replace the `index.yaml` file in github at `threefoldfoundation/helmcharts/index.yaml`.
