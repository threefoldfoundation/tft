# Transferring TFT between Stellar and BSC

## Bridge UI

The easiest way is to use the  user interface for transfers using the bridge at <https://bridge.bsc.threefold.io/>.

More technical details on how to transfer without the UI are explained below.

## From BSC to Stellar

The `withdraw` method must be called on contract **0x8f0FB159380176D324542b3a7933F0C2Fd0c2bbf** with the following parameters:

- blockchain_address: Your stellar address
- network: stellar
- amount: any amount that does not exceed your balance (unsigned integer with a precision of 7 decimals, so 1 TFT = 10000000 )

## From Stellar to BSC

Transfer the TFT to the bridge address **GBFFWXWBZDILJJAMSINHPJEUJKB3H4UYXRWNB4COYQAF7UUQSWSBUXW5** with the target address in the memo text in a specially encoded way.

### Encoding the target address

Hex decode the target address and then base64 encode it again.

Example in python to generate the memo text to send to 0x65e491D7b985f77e60c85105834A0332fF3002CE:

```python
b= bytes.fromhex("65e491D7b985f77e60c85105834A0332fF3002CE")
base64.b64encode(b).decode("utf-8")
'ZeSR17mF935gyFEFg0oDMv8wAs4='
```

### Fees

- From Stellar to BSC:

   To cover the costs of the bridge a fee of 100 TFT is charged. This fee can be modified if it does not cover the gas price for the bridge.

   Make sure the  amount received on the bridge's Stellar address is larger than the Fee..

- From BSC to Stellar:

   a fee of 1 TFT is deducted from the withdrawn amount

## Refunds

When the supplied memo text of a deposit transaction can not be decoded to a valid BSC address, the deposited TFT's are sent back minus 1 TFT to cover the transaction fees of the bridge and to make a DOS attack on the bridge  more expensive.
