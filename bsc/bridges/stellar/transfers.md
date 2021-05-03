# Transferring TFT from Stellar to BSC

Transfer the TFT to the bridge address **GBFFWXWBZDILJJAMSINHPJEUJKB3H4UYXRWNB4COYQAF7UUQSWSBUXW5** with the target addredssd  in the meme text in a specially encoded way.

## Encoding the target address

Hex decode the target address and then base64 encode it again.

Example in python to send to 0x65e491D7b985f77e60c85105834A0332fF3002CE:

```python
b= bytes.fromhex("65e491D7b985f77e60c85105834A0332fF3002CE")
base64.b64encode(b).decode("utf-8")
'ZeSR17mF935gyFEFg0oDMv8wAs4='
```

## Fee

To cover the costs of the bridge ( like the multisig interactions with the Binance chain), a fee of 50 TFT is charged. Make sure the amount received on the bridge's Stellar address is larger than 50 TFT.