# Publish the bridge UI

## Building the docker image

In this folder execute

```sh
docker build -t bsc-bridge-ui:$(git describe --abbrev=0 --tags | sed 's/^v//') . --no-cache
```
