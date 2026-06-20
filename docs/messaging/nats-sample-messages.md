# NATS Sample Messages — ChainOps Block Activity

All messages are published to subject `chainops.block.activity`. Each message carries four NATS headers and a JSON body. Headers are shown above each payload.

---

## 1. Native ETH — Incoming Transfer

A monitored wallet receives 0.5 ETH from another address. Gas is not included because the receiver never pays gas.

**Headers**
```
X-Network-ID:    ethereum
X-Block-Number:  21500000
X-Batch-Index:   1
X-Total-Batches: 1
```

**Body**
```json
{
  "network_id": "ethereum",
  "block_number": 21500000,
  "block_hash": "0xabc123def456aaa000111222333444555666777888999aaabbbcccdddeeefff",
  "block_timestamp": "2025-03-15T10:30:00Z",
  "events": [
    {
      "wallet_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "tx_hash": "0x111aaa222bbb333ccc444ddd555eee666fff777aaa888bbb999ccc000ddd111e",
      "event_type": "NATIVE_TRANSFER",
      "activity_type": "INCOMING",
      "from_address": "0xSenderWallet0000000000000000000000000000",
      "to_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "amount": "500000000000000000",
      "asset": {
        "type": "NATIVE",
        "symbol": "ETH"
      }
    }
  ]
}
```

> `amount` is in wei. 500000000000000000 wei = 0.5 ETH.

---

## 2. Native ETH — Outgoing Transfer

A monitored wallet sends 1 ETH to another address. Gas details are always present on outgoing transactions.

**Headers**
```
X-Network-ID:    ethereum
X-Block-Number:  21500001
X-Batch-Index:   1
X-Total-Batches: 1
```

**Body**
```json
{
  "network_id": "ethereum",
  "block_number": 21500001,
  "block_hash": "0xbbb222ccc333ddd444eee555fff666aaa777bbb888ccc999ddd000eee111fff2",
  "block_timestamp": "2025-03-15T10:31:00Z",
  "events": [
    {
      "wallet_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "tx_hash": "0x222bbb333ccc444ddd555eee666fff777aaa888bbb999ccc000ddd111eee222f",
      "event_type": "NATIVE_TRANSFER",
      "activity_type": "OUTGOING",
      "from_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "to_address": "0xRecipientWallet000000000000000000000000",
      "amount": "1000000000000000000",
      "asset": {
        "type": "NATIVE",
        "symbol": "ETH"
      },
      "gas": {
        "fee_paid": "420000000000000",
        "fee_asset": "ETH",
        "gas_used": 21000,
        "effective_gas_price": "20000000000"
      }
    }
  ]
}
```

> `fee_paid` = `gas_used` × `effective_gas_price` in wei.

---

## 3. USDC — Incoming Token Transfer

A monitored wallet receives 100 USDC. The asset block includes the ERC-20 contract address. `metadata` carries the log position within the block for exact replay.

**Headers**
```
X-Network-ID:    ethereum
X-Block-Number:  21500002
X-Batch-Index:   1
X-Total-Batches: 1
```

**Body**
```json
{
  "network_id": "ethereum",
  "block_number": 21500002,
  "block_hash": "0xccc333ddd444eee555fff666aaa777bbb888ccc999ddd000eee111fff222aaa3",
  "block_timestamp": "2025-03-15T10:32:00Z",
  "events": [
    {
      "wallet_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "tx_hash": "0x333ccc444ddd555eee666fff777aaa888bbb999ccc000ddd111eee222fff333a",
      "event_type": "TOKEN_TRANSFER",
      "activity_type": "INCOMING",
      "from_address": "0xSomeSender000000000000000000000000000000",
      "to_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "amount": "100000000",
      "asset": {
        "type": "ERC20",
        "symbol": "USDC",
        "contract_address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
      },
      "metadata": {
        "log_index": 5,
        "tx_index": 12
      }
    }
  ]
}
```

> USDC has 6 decimals: 100000000 = 100.000000 USDC.

---

## 4. Contract Interaction — Outgoing

A monitored wallet calls a function on a smart contract (e.g. approving a token spend, interacting with a DEX). The `selector` in metadata is the first 4 bytes of the transaction data and identifies which function was called.

**Headers**
```
X-Network-ID:    ethereum
X-Block-Number:  21500003
X-Batch-Index:   1
X-Total-Batches: 1
```

**Body**
```json
{
  "network_id": "ethereum",
  "block_number": 21500003,
  "block_hash": "0xddd444eee555fff666aaa777bbb888ccc999ddd000eee111fff222aaa333bbb4",
  "block_timestamp": "2025-03-15T10:33:00Z",
  "events": [
    {
      "wallet_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "tx_hash": "0x444ddd555eee666fff777aaa888bbb999ccc000ddd111eee222fff333aaa444b",
      "event_type": "CONTRACT_INTERACTION",
      "activity_type": "OUTGOING",
      "from_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "to_address": "0xSmartContractAddress0000000000000000000",
      "amount": "0",
      "asset": {
        "type": "NATIVE",
        "symbol": "ETH"
      },
      "gas": {
        "fee_paid": "2100000000000000",
        "fee_asset": "ETH",
        "gas_used": 105000,
        "effective_gas_price": "20000000000"
      },
      "metadata": {
        "selector": "0xa9059cbb",
        "status": 1
      }
    }
  ]
}
```

> `status: 1` means the transaction succeeded on-chain. `status: 0` means it reverted.

---

## 5. Contract Deployment — Outgoing

A monitored wallet deploys a new smart contract. `to_address` is empty because there is no recipient — the contract address is generated by the network and returned in metadata.

**Headers**
```
X-Network-ID:    ethereum
X-Block-Number:  21500004
X-Batch-Index:   1
X-Total-Batches: 1
```

**Body**
```json
{
  "network_id": "ethereum",
  "block_number": 21500004,
  "block_hash": "0xeee555fff666aaa777bbb888ccc999ddd000eee111fff222aaa333bbb444ccc5",
  "block_timestamp": "2025-03-15T10:34:00Z",
  "events": [
    {
      "wallet_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "tx_hash": "0x555eee666fff777aaa888bbb999ccc000ddd111eee222fff333aaa444bbb555c",
      "event_type": "CONTRACT_DEPLOYMENT",
      "activity_type": "OUTGOING",
      "from_address": "0xABCDEF1234567890abcdef1234567890ABCDEF12",
      "to_address": "",
      "amount": "0",
      "asset": {
        "type": "NATIVE",
        "symbol": "ETH"
      },
      "gas": {
        "fee_paid": "10000000000000000",
        "fee_asset": "ETH",
        "gas_used": 500000,
        "effective_gas_price": "20000000000"
      },
      "metadata": {
        "status": 1,
        "contract_address": "0xNewlyDeployedContract000000000000000000"
      }
    }
  ]
}
```

---

## 6. Batched Block — Multiple Events Across Two Batches

When a block produces more than 100 wallet events, they are split. Both batches share the same block context. Only `events`, `X-Batch-Index`, and `X-Total-Batches` differ.

**Batch 1 of 2 — Headers**
```
X-Network-ID:    ethereum
X-Block-Number:  21500010
X-Batch-Index:   1
X-Total-Batches: 2
```

**Batch 1 of 2 — Body (abbreviated)**
```json
{
  "network_id": "ethereum",
  "block_number": 21500010,
  "block_hash": "0xfff666aaa777bbb888ccc999ddd000eee111fff222aaa333bbb444ccc555ddd6",
  "block_timestamp": "2025-03-15T10:40:00Z",
  "events": [
    { "...first 100 events..." }
  ]
}
```

**Batch 2 of 2 — Headers**
```
X-Network-ID:    ethereum
X-Block-Number:  21500010
X-Batch-Index:   2
X-Total-Batches: 2
```

**Batch 2 of 2 — Body (abbreviated)**
```json
{
  "network_id": "ethereum",
  "block_number": 21500010,
  "block_hash": "0xfff666aaa777bbb888ccc999ddd000eee111fff222aaa333bbb444ccc555ddd6",
  "block_timestamp": "2025-03-15T10:40:00Z",
  "events": [
    { "...remaining events..." }
  ]
}
```

> The consumer knows it has received the full block when `X-Batch-Index` equals `X-Total-Batches`.
