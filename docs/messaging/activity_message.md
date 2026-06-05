# ChainOps Messaging — `BlockActivityMessage`

**Status:** Draft v1
**Owner:** Vicky
**Last updated:** 2026-06-05
**Authoritative source (Go):** [`services/listener/schema/block_activity.go`](../../services/listener/schema/block_activity.go)

This document specifies the wire contract between the `listener-go` service (producer) and any downstream consumer (V1: `ledger-api-java`). It is the *only* coupling point between the two services; both sides must conform.

---

## 1. Purpose

A `BlockActivityMessage` is the listener's published statement that:

> "For network N, at block B (hash H, timestamp T), I observed the following on-chain activity affecting wallets currently in `indexed_wallets`."

One message represents **one block**. The message carries a list of `ActivityEvent` items — one per (tracked wallet × observed event) pairing. A single transaction can produce multiple events in the same message when it affects multiple tracked wallets, or affects the same wallet from multiple perspectives (e.g., self-transfer).

Messages are emitted only for blocks at depth ≥ `SAFE_BLOCK_BUFFER` behind the chain head (Sepolia: 12). Per V1 design, events in a message are treated as **final** by consumers — there is no `seen → confirmed → reverted` state machine and no reorg-correction protocol in V1 (see `V1_SCOPE.md §4.6`).

---

## 2. Producer / Consumer

| Role | Service | Notes |
|---|---|---|
| Producer | `listener-go` | One listener instance per network. Each instance publishes only its configured network's blocks. |
| Consumer (V1) | `ledger-api-java` | Inserts events into `wallet_activities`. Must implement deduplication (see §5). |
| Consumer (V2+) | notifier, webhook dispatcher, alerting service, etc. | Each is an independent consumer with its own queue bound to the same exchange. |

---

## 3. Transport topology (RabbitMQ)

| Concern | Convention |
|---|---|
| Exchange name | `chainops.activity` |
| Exchange type | `topic` |
| Exchange durability | `durable: true` |
| Routing key | `chainops.activity.<network_key>` — e.g., `chainops.activity.sepolia` |
| Consumer queue (V1 ledger) | `chainops.activity.ledger` |
| Queue durability | `durable: true` |
| Queue binding | `chainops.activity.#` (catches all networks) |
| Message persistence | `delivery_mode: 2` (persistent) |
| Publisher confirms | **Required.** Listener only advances `networks.last_scanned_block` after the broker confirms publication. |
| Consumer acks | **Manual.** Consumer ACKs only after the DB write commits. NACK with requeue on transient failures; NACK without requeue (or send to DLQ) on poison messages. |

A topic exchange with the `<network_key>` routing pattern means V2 chains (`base`, `arbitrum`, `optimism`) require no exchange or queue changes — only a new listener instance with that network configured. The ledger's existing queue binding (`chainops.activity.#`) consumes them automatically.

---

## 4. Content-Type and encoding

| Property | Value |
|---|---|
| `content_type` | `application/json` |
| `content_encoding` | `utf-8` |
| Numeric encoding | All on-chain numeric values (wei, raw token amounts, gas prices) are encoded as **decimal strings** to avoid JSON number precision loss. |
| Address encoding | Hex strings, `0x`-prefixed, **EIP-55 mixed-case checksum format**. |
| Hash encoding | Hex strings, `0x`-prefixed, 66 characters total (`0x` + 64 hex). |
| Timestamp encoding | RFC 3339 (`2026-06-05T12:34:56Z`). |

---

## 5. Delivery semantics

**At-least-once.** Publisher confirms guarantee the broker durably has each message before the listener advances its checkpoint. On listener restart mid-batch, blocks may be re-published. Consumers MUST dedupe.

**Recommended consumer dedup key:**

```
(tx_hash, indexed_wallet_id, activity_type)
```

Same transaction can produce both `INCOMING` and `OUTGOING` rows (e.g., self-transfer between two tracked wallets) — the `activity_type` discriminator is part of the key. Implement as a `UNIQUE` constraint on `wallet_activities` and rely on `ON CONFLICT DO NOTHING` (Postgres) for idempotent inserts.

**Message ordering** is per-network. Within a single network's stream, messages arrive in monotonic block order *barring listener restarts that re-emit a range*. Consumers must not assume strict ordering and must rely on `block_number` for chronological reasoning.

**Poison messages** (un-parseable, schema-violating) are NACKed without requeue and routed to a dead-letter queue (`chainops.activity.dlq`) for inspection. Consumer crashes mid-handle result in NACK-with-requeue.

---

## 6. Versioning

Every message carries a top-level `schema_version` (integer). The current version is **1**.

Compatibility rules:

- **Producer always sets** the highest version it knows.
- **Consumer rejects** (NACK to DLQ) any message whose `schema_version` is higher than the consumer's supported max — fail loud rather than silently drop fields.
- **Additive changes** (new optional fields) do NOT bump the version. Existing consumers ignore unknown fields.
- **Breaking changes** (renames, removals, type changes, new required fields) DO bump the version. Producer and all consumers must be rolled in coordinated order: deploy consumers that support both N and N+1, then flip the producer to emit N+1, then retire N support.

> **Note on the Go source:** the current `schema/block_activity.go` does not yet include `SchemaVersion`. This field MUST be added at the `BlockActivityMessage` level before the first published message — it cannot be retrofitted cleanly later. Suggested field: `SchemaVersion int \`json:"schema_version"\``.

---

## 7. Message structure

### 7.1 `BlockActivityMessage` (envelope)

| Field | JSON key | Type | Required | Description |
|---|---|---|---|---|
| `SchemaVersion` | `schema_version` | int | Y | Contract version. V1: `1`. |
| `NetworkID` | `network_id` | UUID string | Y | FK to `networks.id`. Single source of truth for which chain this block belongs to. |
| `BlockNumber` | `block_number` | uint64 | Y | Canonical block number of this block on `network_id`. |
| `BlockHash` | `block_hash` | string | Y | 0x-prefixed 32-byte block hash. Retained for audit/provenance only; not used for state-machine logic in V1. |
| `BlockTimestamp` | `block_timestamp` | RFC 3339 string | Y | Block header timestamp (chain-local time). |
| `Events` | `events` | array of `ActivityEvent` | Y | One entry per (tracked wallet × event) pairing observed in this block. May be **empty** when the listener processed a block with no tracked-wallet activity but wants to advance its known-good frontier. |

**Empty-events policy:** Whether to emit zero-event messages (as a heartbeat/frontier signal) is an open question. V1 default: do not emit zero-event messages. The listener advances `networks.last_scanned_block` without publishing. Revisit if consumers need a per-block "I am alive at block N" signal.

### 7.2 `ActivityEvent`

| Field | JSON key | Type | Required | Description |
|---|---|---|---|---|
| `WalletAddress` | `wallet_address` | string (address) | Y | The **perspective wallet** — the tracked address this event is about. Must match a row in `indexed_wallets` for this network at the time the consumer processes the message. |
| `TxHash` | `tx_hash` | string | Y | 0x-prefixed transaction hash. |
| `EventType` | `event_type` | enum string | Y | The *nature* of the underlying transaction (§8.1). |
| `ActivityType` | `activity_type` | enum string | Y | The *perspective* — what the transaction did relative to `wallet_address` (§8.2). |
| `FromAddress` | `from_address` | string (address) | Y* | Sender. Always present except where semantically meaningless. |
| `ToAddress` | `to_address` | string (address) | Y* | Receiver. Null permitted for `CONTRACT_DEPLOYMENT`. |
| `Amount` | `amount` | decimal string | Y | Raw on-chain integer value as a decimal string (no decimal point). Interpretation depends on `Asset`. For native ETH: wei. For ERC-20: raw token units (decimals applied at read time via `assets`/`asset_deployments`). For `CONTRACT_INTERACTION` with no value transfer: `"0"`. |
| `Asset` | `asset` | object (`Asset`) | N | Present when the event involves a defined asset. Absent for pure contract calls with no value transfer. |
| `GasDetails` | `gas` | object (`GasDetails`) | N | Present on `OUTGOING` events for the paying wallet. Absent on `INCOMING` (the receiver doesn't pay gas). |
| `Metadata` | `metadata` | object (free-form) | N | Event-specific structured data — function selector, contract address created, decoded params, etc. See §10 for current keys. |

**Perspective semantics:** if a transaction transfers value between two tracked wallets A and B, the message contains TWO `ActivityEvent` entries — one with `wallet_address = A, activity_type = OUTGOING` and one with `wallet_address = B, activity_type = INCOMING`. Likewise a self-transfer produces two entries with the same `wallet_address` but different `activity_type`. Consumers MUST handle both rows and rely on dedup (§5) for idempotency.

### 7.3 `Asset`

| Field | JSON key | Type | Required | Description |
|---|---|---|---|---|
| `AssetType` | `type` | string | Y | High-level category. V1 values: `"NATIVE"` (ETH), `"ERC20"`. |
| `Symbol` | `symbol` | string | Y | Display symbol — `"ETH"`, `"USDC"`. |
| `ContractAddress` | `contract_address` | string (address) | N | Token contract address. Absent for `NATIVE`. |

### 7.4 `GasDetails`

| Field | JSON key | Type | Required | Description |
|---|---|---|---|---|
| `FeePaid` | `fee_paid` | decimal string | Y | Total fee paid by the sender in wei (`gas_used × effective_gas_price`). |
| `FeeAsset` | `fee_asset` | string | Y | Asset symbol the fee was paid in. V1: always `"ETH"` on Sepolia. |
| `GasUsed` | `gas_used` | uint64 | Y | Gas units consumed by the transaction. |
| `EffectiveGasPrice` | `effective_gas_price` | decimal string | N | EIP-1559 effective gas price in wei (`base_fee + priority_tip`). |

---

## 8. Enums

### 8.1 `EventType` — nature of the underlying transaction

| Value | Meaning |
|---|---|
| `NATIVE_TRANSFER` | Plain ETH transfer (no contract interaction, value > 0). |
| `TOKEN_TRANSFER` | ERC-20 `Transfer` event involving a tracked wallet. |
| `CONTRACT_INTERACTION` | Outgoing transaction calling a contract method (with or without ETH value). |
| `CONTRACT_DEPLOYMENT` | Outgoing transaction creating a new contract (`to_address` is null). |
| `NFT_TRANSFER` | Reserved for V2+ — ERC-721 / ERC-1155 transfers. **Not emitted in V1.** |
| `DEFI_SWAP` | Reserved for V2+ — DEX swap detection. **Not emitted in V1.** |

### 8.2 `ActivityType` — perspective of the tracked wallet

| Value | Meaning |
|---|---|
| `INCOMING` | Value (or token) flowed into `wallet_address`. |
| `OUTGOING` | Value (or token) flowed out of `wallet_address`, or `wallet_address` paid gas for a contract interaction / deployment. |
| `MINT` | ERC-20 tokens were minted to `wallet_address` (Transfer event with `from = 0x0`). |
| `BURN` | ERC-20 tokens were burned from `wallet_address` (Transfer event with `to = 0x0`). |

---

## 9. Examples

### 9.1 Incoming native ETH transfer

A tracked wallet received 0.05 ETH:

```json
{
  "schema_version": 1,
  "network_id": "a1b59dde-2714-4fa8-b2a8-92ab6bb51590",
  "block_number": 10927399,
  "block_hash": "0x4f7d3...redacted",
  "block_timestamp": "2026-05-26T16:53:20Z",
  "events": [
    {
      "wallet_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "tx_hash": "0x944b3864c73b76000b55f0ae79539932eafeeeb7515b317eed9facf406a0185b",
      "event_type": "NATIVE_TRANSFER",
      "activity_type": "INCOMING",
      "from_address": "0xabc1234567890abcdef1234567890abcdef12345",
      "to_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "amount": "50000000000000000",
      "asset": {
        "type": "NATIVE",
        "symbol": "ETH"
      }
    }
  ]
}
```

### 9.2 Incoming USDC transfer

```json
{
  "schema_version": 1,
  "network_id": "a1b59dde-2714-4fa8-b2a8-92ab6bb51590",
  "block_number": 10927420,
  "block_hash": "0x...",
  "block_timestamp": "2026-05-26T16:57:32Z",
  "events": [
    {
      "wallet_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "tx_hash": "0xdeadbeef...",
      "event_type": "TOKEN_TRANSFER",
      "activity_type": "INCOMING",
      "from_address": "0xabc1234567890abcdef1234567890abcdef12345",
      "to_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "amount": "100000000",
      "asset": {
        "type": "ERC20",
        "symbol": "USDC",
        "contract_address": "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238"
      }
    }
  ]
}
```

(100 USDC, since USDC has 6 decimals: `100_000000`.)

### 9.3 Outgoing contract interaction (e.g., approve)

```json
{
  "schema_version": 1,
  "network_id": "a1b59dde-2714-4fa8-b2a8-92ab6bb51590",
  "block_number": 10927421,
  "block_hash": "0x...",
  "block_timestamp": "2026-05-26T16:57:44Z",
  "events": [
    {
      "wallet_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "tx_hash": "0xcafebabe...",
      "event_type": "CONTRACT_INTERACTION",
      "activity_type": "OUTGOING",
      "from_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "to_address": "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238",
      "amount": "0",
      "gas": {
        "fee_paid": "210000000000000",
        "fee_asset": "ETH",
        "gas_used": 46000,
        "effective_gas_price": "4565217391"
      },
      "metadata": {
        "selector": "0x095ea7b3",
        "status": 1
      }
    }
  ]
}
```

### 9.4 USDC mint (from address = 0x0)

```json
{
  "schema_version": 1,
  "network_id": "a1b59dde-2714-4fa8-b2a8-92ab6bb51590",
  "block_number": 10927450,
  "block_hash": "0x...",
  "block_timestamp": "2026-05-26T17:03:20Z",
  "events": [
    {
      "wallet_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "tx_hash": "0xfade1234...",
      "event_type": "TOKEN_TRANSFER",
      "activity_type": "MINT",
      "from_address": "0x0000000000000000000000000000000000000000",
      "to_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "amount": "500000000",
      "asset": {
        "type": "ERC20",
        "symbol": "USDC",
        "contract_address": "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238"
      }
    }
  ]
}
```

### 9.5 Self-transfer (one wallet, two events in one message)

A tracked wallet sends 1 ETH to itself:

```json
{
  "schema_version": 1,
  "network_id": "a1b59dde-2714-4fa8-b2a8-92ab6bb51590",
  "block_number": 10927501,
  "block_hash": "0x...",
  "block_timestamp": "2026-05-26T17:13:32Z",
  "events": [
    {
      "wallet_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "tx_hash": "0xself1234...",
      "event_type": "NATIVE_TRANSFER",
      "activity_type": "OUTGOING",
      "from_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "to_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "amount": "1000000000000000000",
      "asset": { "type": "NATIVE", "symbol": "ETH" },
      "gas": {
        "fee_paid": "441000000000000",
        "fee_asset": "ETH",
        "gas_used": 21000,
        "effective_gas_price": "21000000000"
      }
    },
    {
      "wallet_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "tx_hash": "0xself1234...",
      "event_type": "NATIVE_TRANSFER",
      "activity_type": "INCOMING",
      "from_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "to_address": "0x32056651573c19C329c9619DAF25A72e0D8a48dC",
      "amount": "1000000000000000000",
      "asset": { "type": "NATIVE", "symbol": "ETH" }
    }
  ]
}
```

---

## 10. Reserved `metadata` keys (current)

Field-level free-form `metadata` per `ActivityEvent`. Consumers should treat unknown keys as forward-compatible no-ops.

| Key | Type | Emitted when | Meaning |
|---|---|---|---|
| `selector` | string | `CONTRACT_INTERACTION` | First 4 bytes of `tx.data` as `0x`-prefixed hex (the function selector). |
| `status` | int | All outgoing events | Tx receipt status: `1` success, `0` revert. |
| `contract_address` | string | `CONTRACT_DEPLOYMENT` | Address of the contract created by this transaction. |

Adding a new metadata key is non-breaking (consumers must ignore unknowns). Removing one is breaking and requires a `schema_version` bump.

---

## 11. Producer responsibilities

The listener MUST:

- Emit messages for blocks at depth ≥ `SAFE_BLOCK_BUFFER` only.
- Wait for publisher-confirm before advancing `networks.last_scanned_block`.
- Set `schema_version` to the highest version it knows.
- Encode all on-chain numeric values as decimal strings.
- Emit one `ActivityEvent` per (tracked wallet × observed event) pairing — never collapse multi-wallet impacts into a single row.
- Set `gas` on `OUTGOING` events only.
- Set `metadata.status` on every outgoing event (reflecting tx receipt success/revert).

The listener MUST NOT:

- Emit messages for blocks below `SAFE_BLOCK_BUFFER`.
- Emit `seen` / `confirmed` / `reverted` state — there is no such concept in V1.
- Emit messages for wallets not currently in `indexed_wallets`.
- Mutate already-emitted messages or expect the consumer to retract.

---

## 12. Consumer responsibilities

The consumer MUST:

- Implement dedup keyed at minimum on `(tx_hash, indexed_wallet_id, activity_type)`.
- ACK messages only after the DB write commits.
- NACK to DLQ on schema-version-too-high or parse failure.
- Resolve `wallet_address` → `indexed_wallet_id` per `network_id` at write time.
- Treat unknown `metadata` keys as no-ops (forward compatibility).

The consumer MUST NOT:

- Auto-ack.
- Assume strict in-order delivery within a network (use `block_number` for chronology).
- Reject messages with unknown event-type or activity-type values purely on enum grounds — log + DLQ instead.

---

## 13. Open questions

1. **Empty-events messages.** Default in V1: not emitted. Revisit if consumers need a per-block alive signal.
2. **Address checksumming.** Spec mandates EIP-55 mixed-case. Current Go producer hex-formats via `common.Address.Hex()`, which already produces checksummed output. Confirmed; no producer change required.
3. **`Amount` precision floor on the DB side.** Schema `wallet_activities.amount` is `NUMERIC(38, 18)`, which cannot losslessly hold a full uint256. Producer emits raw integers; consumer chooses whether to store raw or normalized. Decision tracked in `V1_SCOPE.md §6.1`.
4. **`schema_version` field absent in current Go struct.** MUST be added before first published message.
5. **DLQ topology.** `chainops.activity.dlq` mentioned in §3; declare and bind before first publish.

---

## 14. Change log

| Date | Version | Change |
|---|---|---|
| 2026-06-05 | 1 (draft) | Initial specification derived from `services/listener/schema/block_activity.go`. |
