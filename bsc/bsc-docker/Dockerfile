FROM debian:bullseye-slim

ARG TARGETARCH

# mainnet or testnet
ARG network=mainnet

RUN apt-get update -y \
  && apt-get install -y curl jq unzip \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

RUN if [ "${TARGETARCH}" = "arm64" ]; then geth_binary="geth-linux-arm64"; else geth_binary="geth_linux"; fi && \ 
  curl https://api.github.com/repositories/265775217/releases/tags/v1.2.8 | jq -c ".assets[] | select( .browser_download_url | contains(\"${network}.zip\") or contains(\"$geth_binary\")) | .browser_download_url" | xargs -n1 curl -LOJ && \
  unzip ${network}.zip -d / && \
  sed -i 's/^HTTPHost.*/HTTPHost = "0.0.0.0"/' /config.toml && \
  sed -i '/^WSPort.*/a WSHost = "0.0.0.0"' /config.toml && \
  sed -i 's/^HTTPVirtualHosts.*/HTTPVirtualHosts = ["*"]/' /config.toml && \
  sed -i '/Node\.LogConfig/,/^$/d' /config.toml && \ 
  mv $geth_binary /usr/local/bin/bsc && \
  chmod +x /usr/local/bin/bsc

ENV BSC_DATADIR=/storage

COPY docker-entrypoint.sh /entrypoint.sh


# NODE P2P
EXPOSE 30311/udp
EXPOSE 30311/tcp

# pprof / metrics
EXPOSE 6060/tcp

# HTTP based JSON RPC API
EXPOSE 8545/tcp
# WebSocket based JSON RPC API
EXPOSE 8546/tcp

ENTRYPOINT ["/entrypoint.sh"]

CMD ["bsc"]
