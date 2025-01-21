# Troubleshooting

## Leader Fails to get signatures

the logs will indicate

```log
ERROR[08-03|07:57:07.393] failed to get signature         peerID="..." err="failed to connect to host id ...
```

A cosigner is unreachable. To check which one, a [tool](../../tools/stellartolip2p) is available to convert a Stellar address to a libp2p peerID.
