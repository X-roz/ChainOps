# ChainOps & NATS — How It All Works

## What is NATS?

NATS is a lightweight messaging system. The simplest way to think about it: it is a **post office**. One service drops a letter into a named mailbox called a **subject**, and any service that has subscribed to that mailbox picks it up.

Plain NATS is fire-and-forget — if nobody is listening when a message arrives, it is gone. That is where **JetStream** comes in.

---

## What is JetStream?

JetStream is NATS's persistence layer built on top of the core messaging system. The difference is significant:

| | Plain NATS | JetStream |
|---|---|---|
| Message stored on disk? | No | Yes |
| Message survives consumer downtime? | No | Yes |
| Publisher gets delivery confirmation? | No | Yes (ack from server) |
| Consumer can replay old messages? | No | Yes |
| Can delete after all consumers read? | No | Yes (interest retention) |

ChainOps uses JetStream specifically because the ledger service must not miss any block activity even if it restarts or falls behind.

---

## How ChainOps Connects to NATS

The listener service reads its NATS URL and subject name from `application.yml` under the `publisher` section. On startup it opens a TCP connection to the NATS server and immediately upgrades it to a JetStream context.

The connection is configured to be self-healing. If the network drops or the NATS server restarts, the client automatically retries the connection every 2 seconds, indefinitely, with no manual intervention needed.

The subject ChainOps publishes to is `chainops.block.activity`. This is the single mailbox that carries all blockchain wallet activity across every block scanned.

---

## The End-to-End Flow

```
Blockchain RPC Node
        │
        │  polled every 1 minute
        ▼
EVM Listener scans a range of confirmed blocks
        │
        │  for each block where a monitored wallet had activity
        ▼
BlockActivityMessage is assembled
        │
        │  if events > 100, split into batches of 100
        ▼
Each batch published to NATS JetStream
        │
        │  server writes to disk and sends ack back
        ▼
NATS Stream: CHAINOPS_BLOCKS  (persistent)
        │
        ▼
Ledger Service consumes and acknowledges
        │
        │  once all consumers ack (interest retention)
        ▼
Message deleted from stream
```

---

## What Gets Published

Every published message is a snapshot of one block's worth of wallet activity. It always contains the block context at the top level and an array of individual wallet events.

### Top-Level Fields

| Field | Type | Description |
|---|---|---|
| `network_id` | UUID string | FK to `networks.id` in the ledger DB. Identifies which blockchain network this block belongs to. The listener resolves this at startup from `network_key` (e.g. `sepolia`) → UUID via `networks` table lookup; consumers read it as-is and resolve back to a network record when needed. |
| `block_number` | number | The block height that was scanned |
| `block_hash` | string | Unique identifier of that block |
| `block_timestamp` | ISO datetime | When the block was mined |
| `events` | array | One entry per wallet activity detected in this block. May be split across multiple messages — see Batching. |

### Per-Event Fields

| Field | Type | Description |
|---|---|---|
| `wallet_address` | string | The monitored wallet that triggered this event |
| `tx_hash` | string | The transaction on-chain that caused this event |
| `event_type` | enum | What kind of activity happened (see table below) |
| `activity_type` | enum | Direction of the activity relative to the wallet |
| `from_address` | string | Who sent funds or triggered the action |
| `to_address` | string | Who received funds or was the target |
| `amount` | string | Value transferred, in the smallest unit (wei for ETH, micro-units for USDC) |
| `asset` | object | What asset moved — type, symbol, and contract address if ERC-20 |
| `gas` | object | Gas cost details — only present on outgoing transactions |
| `metadata` | object | Extra context depending on event type (selector, contract address, log index) |

---

## Event Types

These describe what kind of on-chain action happened:

| `event_type` | When it is emitted |
|---|---|
| `NATIVE_TRANSFER` | ETH (or native coin) moved directly between two wallets |
| `TOKEN_TRANSFER` | An ERC-20 token (currently USDC) moved between wallets |
| `CONTRACT_INTERACTION` | A monitored wallet called a function on a smart contract |
| `CONTRACT_DEPLOYMENT` | A monitored wallet deployed a new smart contract |
| `NFT_TRANSFER` | An NFT moved — defined in schema, not yet emitted |
| `DEFI_SWAP` | A DEX token swap — defined in schema, not yet emitted |

## Activity Types

These describe the direction of the activity **relative to the monitored wallet**:

| `activity_type` | Meaning |
|---|---|
| `INCOMING` | Value arrived into the monitored wallet |
| `OUTGOING` | Value left the monitored wallet |
| `MINT` | Token was minted directly to the wallet (sender is zero address) |
| `BURN` | Token was burned from the wallet (recipient is zero address) |

---

## NATS Message Headers

Each NATS message carries these headers outside the JSON body. A consumer can read these without deserializing the full payload — useful for routing, filtering, or logging.

| Header | Example | Purpose |
|---|---|---|
| `X-Network-ID` | `a1b59dde-2714-4fa8-b2a8-92ab6bb51590` | UUID of the network this batch belongs to (same value as the `network_id` field in the body) |
| `X-Block-Number` | `10927399` | The block number this batch covers |
| `X-Batch-Index` | `2` | Position of this batch within the block (starts at 1) |
| `X-Total-Batches` | `3` | How many total batches exist for this block |

There is no separate correlation ID. The combination of `X-Network-ID` and `X-Block-Number` uniquely identifies a block's worth of activity. `X-Batch-Index` and `X-Total-Batches` together tell the consumer whether it has received everything for that block.

---

## Batching

A single block can contain many transactions. If a block produces more than 100 wallet events, the publisher splits them into chunks of 100 and sends each chunk as a **separate NATS message**. The split is implemented in `services/listener/publisher/publisher.go` (`chunk()` and `publishBatch()`); the batch size is the package constant `batchSize`.

**Wire shape per batch.** Each batch is a complete, well-formed `BlockActivityMessage` with the same envelope fields and a subset of the original `events` array. All batches for the same block share identical `network_id`, `block_number`, `block_hash`, and `block_timestamp`. Only the `events` array differs.

**Batch identification.** Each message carries the headers `X-Batch-Index` (1-indexed) and `X-Total-Batches`. A single-message block always has `X-Batch-Index: 1` and `X-Total-Batches: 1`. There is no header that needs to change to indicate batching mode — single and batched blocks use the same shape.

**Empty-events blocks are not published.** The listener only publishes when at least one tracked-wallet event was detected. It still advances `networks.last_scanned_block` after processing the block. Consumers cannot rely on a per-block "alive" signal from this stream.

**Consumer obligations:**

- **Do not assume one message = one block.** A block with >100 events arrives as multiple messages with the same `network_id` + `block_number`.
- **Do not buffer-and-flush by block.** Persist each batch's events as they arrive. Batches may be re-delivered out of order under NAK + redeliver semantics (e.g., batch 2 of 3 is acked, batch 1 is NAKed and retried later) — buffering by block creates a hang under that scenario.
- **Idempotency is per-event, not per-batch.** Dedup at the row level (`UNIQUE (tx_hash, indexed_wallet_id, activity_type)` on `wallet_activities`) so a redelivered batch produces no duplicate rows.
- **Do not infer total event count from a single message.** `events.length` is the count *for this batch only*, not for the block.

**Producer ordering guarantee:** within a single block's batches, the publisher sends batch 1, then batch 2, etc., synchronously waiting for each JetStream ack before sending the next. The next block is never started until the previous block's batches all succeed. This means at the broker, batches arrive in publish order *per network*. Consumers should not rely on this for correctness (NAK + redeliver can reorder), but it is the producer's intent.

---

## Schema Versioning

V1 messages do **not** carry a `schema_version` field. The wire contract is the single version defined by `services/listener/schema/block_activity.go`.

Rationale: only one producer, only one consumer, both in-repo, deployed in lockstep. Adding a version field would be ceremony with no current benefit. The cost of "we will need it eventually" is one line of code added later, not a structural redesign.

**When to add `schema_version`:**

1. When a second consumer joins (notifier, webhook dispatcher, analytics aggregator) and they cannot all be rolled forward simultaneously.
2. When the contract is reshaped in a breaking way (renamed field, type change, removed field, new required field). Additive changes (new optional fields) do not require versioning.
3. When public ChainOps deployments diverge from this repo's tip.

**How to add it later:**

Add `SchemaVersion int \`json:"schema_version"\`` to `BlockActivityMessage` defaulted to `2` in the producer. Consumers that don't read the field accept it as an unknown JSON key (forward-compatible). Consumers that do read it reject `> max_known` to a dead-letter destination. Producer flip and consumer rollout must be coordinated in the standard "deploy consumers first, then producer" order.

Until that day comes, every message on `chainops.block.activity` is implicitly schema version 1, identified by the absence of a version field.

---

## Message Retention — When Does NATS Delete a Message?

JetStream supports three retention strategies. ChainOps uses **interest retention**, which means a message is deleted only after every registered consumer has acknowledged it.

| Retention Policy | Message deleted when... | ChainOps uses this? |
|---|---|---|
| `limits` (default) | Age or size limits are hit — consumer reads have no effect | No |
| `interest` | Every named durable consumer has acknowledged the message | **Yes** |
| `workqueue` | Any single consumer acknowledges the message | No |

With interest retention, the NATS admin must create all consumer registrations **before** messages start flowing. A message that arrives with zero registered consumers is deleted immediately, because "nobody is interested." The startup order matters: consumers must be registered first, then the publisher starts.

---

## Access Control — Who Can Do What

### The Permission Model

NATS controls access through users defined in the server configuration. Each user is given an explicit allow-list of subjects they may publish to and subjects they may subscribe to. Anything not on the list is rejected at the server the moment the attempt is made. No other connected client is notified.

ChainOps defines three users:

| User | Role | Can Publish | Can Subscribe | Can Manage Streams |
|---|---|---|---|---|
| `admin` | NATS administrator | All subjects | All subjects | Yes |
| `listener-ops` | ChainOps listener (publisher) | `chainops.block.activity` only | Inbox only (for ack) | No |
| `ledger-ops` | Ledger service (consumer) | Ack and info subjects only | `chainops.block.activity` + inbox | No |

### What Each User Can Actually Access

| Subject | `admin` | `listener-ops` | `ledger-ops` |
|---|---|---|---|
| `chainops.block.activity` (publish) | Yes | **Yes** | No — blocked |
| `chainops.block.activity` (subscribe) | Yes | No — blocked | **Yes** |
| `$JS.API.STREAM.CREATE.*` (create a stream) | Yes | No | No |
| `$JS.API.CONSUMER.INFO.*` (check consumer exists) | Yes | No | Yes (read-only) |
| `$JS.ACK.CHAINOPS_BLOCKS.*` (acknowledge message) | Yes | No | **Yes** |
| `_INBOX.*` (receive server replies) | Yes | Yes | Yes |
| Everything else | Yes | **Blocked** | **Blocked** |

### Why Internal NATS Subjects Are Needed

NATS JetStream works internally through a request-reply protocol. When a publisher sends a message, the server needs a way to send the delivery confirmation (ack) back. It uses a temporary inbox address like `_INBOX.abc123` to do this. Without subscribe permission on `_INBOX.*`, the publisher would never receive the ack and every publish would appear to hang or fail.

Similarly, when a consumer wants to check that its pre-created consumer registration exists before pulling messages, it queries `$JS.API.CONSUMER.INFO.*`. This is a read-only lookup — it cannot create or modify anything. The ledger service has exactly this and nothing more.

Neither the listener service nor the ledger service has permission to call `$JS.API.STREAM.CREATE.*`, which is what actually creates a new stream. Only the admin can do that.

---

## What Happens If a Service Publishes to the Wrong Subject

If `listener-ops` publishes to a subject that no stream is configured to capture (for example, a typo or a misconfigured `application.yml`), JetStream returns an immediate error:

> `nats: no stream found`

The message is rejected and never stored. The publisher receives this as an error from its publish call. The existing error handling in the publisher logs it and surfaces it as a block-level failure. Nothing is silently dropped.

This means stream creation being restricted to admin is also a safety net for misconfiguration — a service cannot accidentally create a new stream by publishing to a wrong subject.

---

## Security Summary

| Concern | How ChainOps addresses it |
|---|---|
| Unauthorized publishing | `listener-ops` is the only user allowed to publish to `chainops.block.activity` |
| Unauthorized subscribing | `listener-ops` cannot subscribe; only `ledger-ops` can read messages |
| Rogue stream creation | Neither service has `$JS.API.STREAM.CREATE.*` permission — only admin |
| Wrong subject published | JetStream returns "no stream found" — message rejected, not silently lost |
| Message loss on consumer downtime | JetStream persists to disk; interest retention holds messages until consumer acks |
| Connection resilience | Publisher reconnects every 2 seconds indefinitely on disconnect |
| Credentials in config | Currently inline in `application.yml`; should move to env vars or secrets manager for production |
| Wire encryption | Not yet enabled; add TLS to the NATS server config for production |
