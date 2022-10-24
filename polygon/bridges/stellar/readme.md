# Polygon-Stellar bridge

## Basic flows

### Stellar to Polygon

Since Stellar is the main chain, we call this a **deposit**.

```mermaid
sequenceDiagram
    actor sa as Stellar account
    participant stellar as Stellar network
    participant bridge
    participant polygon as Polygon TFT contract
    sa->>stellar: Transfer to bridge account(amount, polygon target address) 
    bridge ->> stellar: get transactions
    activate bridge
    bridge ->> polygon: mint(amount,target address, stellar tx id)
    deactivate bridge
```

### Polygon to Stellar

Since Stellar is the main chain, we call this a **withdraw**.

```mermaid
sequenceDiagram
    actor pa as Polygon account
    participant polygon as Polygon TFT contract
    participant bridge
    participant stellar as Stellar network
    pa->>polygon: withdraw(amount, stellar target address)
    activate polygon
    polygon->>polygon: burn(amount)
    polygon->> polygon: emit withdraw event
    deactivate polygon
    bridge->>polygon: get events
    activate bridge
    bridge->>stellar: transfer to target address(amount, polygon tx id)
    deactivate bridge

```
