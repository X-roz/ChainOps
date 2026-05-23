# ChainOps — V1 Scope of Work

**Status:** Locked
**Owner:** Vicky
**Last updated:** 2026-05-24

---

## 1. Overview

ChainOps is a subscription-aware blockchain monitoring infrastructure platform. V1 is a forward-only Ethereum wallet activity indexer deployed against the **Sepolia testnet** as the live demo network, built with **network-agnostic code** so that future deployments can target Ethereum mainnet, Arbitrum, Base, or Optimism via configuration changes rather than code rewrites.

V1 demonstrates production-minded blockchain infrastructure thinking: a global wallet-indexing layer with shared scanning across subscribers, a lifecycle-aware account state machine (free trial → active → dormant), SIWE-based identity, an on-chain subscription contract gating premium features, and the observability/deployment maturity expected of a real backend system. It is an open-source MVP intended as a portfolio-grade demonstration of architectural decision-making, and as a codebase that could plausibly become a real product on mainnet with V2 work.

---

## 2. Product Goals

V1 succeeds if:

- A user can sign in via SIWE, receive a 20-day free trial with one tracked wallet, and watch their Sepolia activity (ETH + USDC transfers, plus all outgoing transactions) appear in their dashboard within ~90 seconds of being mined.
- Transactions transition correctly from *seen* to *confirmed*, and reorgs during the unconfirmed window result in `reverted` without ledger corruption.
- The account lifecycle (FREE_TRIAL → ACTIVE → EXPIRED → DORMANT, plus user-initiated CANCELED / DELETED) is enforced by a scheduled job, with the UI accurately reflecting current state.
- Premium users can track up to 5 wallets and access export plus the four named analytics views, gated by on-chain subscription state.
- The same wallet tracked by multiple users is **scanned once globally**, with activity fanned out to all subscribed users.
- The system exposes meaningful operational metrics via Prometheus, with Grafana dashboards.
- The full stack runs locally via `docker-compose up`, and the codebase is structured so that a V2 deployment to a different EVM network requires only configuration changes (RPC endpoint, asset contract addresses, network-appropriate confirmation threshold).

---

## 3. Explicit Non-Goals

- **Live multi-chain deployment.** V1 deploys a single listener instance against Sepolia. The code is network-agnostic; the deployment is single-network. Running additional listeners for other chains is V2.
- **Mainnet deployment.** Sepolia only. Mainnet has gas economics, audit requirements, regulatory considerations, and operational SLAs that V1 does not engage with.
- **Historical backfill.** Reactivating a DORMANT account does NOT recover the activity that occurred during the dormant window. The UI displays an explicit gap notification. Backfill is a paid V2 feature.
- **Non-EVM chains.** Solana, Bitcoin, Cosmos, etc. are out of scope indefinitely.
- **Mempool / pre-confirmation monitoring.** Only mined blocks are observed.
- **Recurring on-chain subscriptions.** The subscription contract is one-shot, fixed-duration (30 days for 30 test-USDC). No keeper, no allowance-based auto-debit, no `PAST_DUE` state.
- **No custodial behaviour.** ChainOps never holds user private keys. Wallet connection is read-only for monitoring.
- **No email / password / OAuth authentication.** SIWE-only.
- **No account recovery.** Loss of the sign-in wallet means loss of the account.
- **Multiple auth wallets per user.** The `auth_wallets` table shape permits N rows per user in V2, but V1 enforces exactly one auth wallet per user at the application layer.
- **Alerting subsystem.** Email, webhook, Telegram notifications are V2. V1 has in-app feed only.
- **Tokens beyond ETH and USDC on Sepolia.** The generic `assets` / `asset_deployments` schema supports extension; V1 seeds two rows.
- **Kubernetes as the live deployment substrate.** K8s manifests live under `infra/kubernetes/` as a portfolio artifact; the live demo runs on a single VPS via docker-compose.

---

## 4. In-Scope Features

### 4.1 Identity and authentication (SIWE)

- Authentication is Sign-In With Ethereum (EIP-4361). User connects wallet, backend issues nonce, user signs structured message, backend verifies signature.
- Internal `users.id` (UUID) is the primary identity, decoupled from any specific wallet address.
- The signed wallet is recorded in `auth_wallets` with `role = 'OWNER'` and `verified_at` timestamp.
- **V1 enforces one auth wallet per user.** Schema permits multiple; application layer rejects additional rows in V1.
- Session: short-lived JWT keyed on `users.id`.

### 4.2 Account lifecycle and free trial

Account states (`users.account_status`):

- `ACTIVE` — eligible for monitoring; subscription valid (paid or trial).
- `EXPIRED` — subscription ended; grace window has not yet elapsed.
- `DORMANT` — subscription ended, grace elapsed, account paused. Tracked wallets removed from global indexing if no other subscribers remain. Historical data retained.
- `CANCELED` — user-initiated termination.
- `DELETED` — user-initiated account removal.

Subscription states (`subscriptions.subscription_status`):

- `FREE_TRIAL` — within 20-day trial window.
- `ACTIVE` — paid, within paid subscription window.
- `EXPIRED` — past `expires_at`.
- `CANCELED` — user explicitly canceled.

**Free trial:**

- Every new user receives a 20-day free trial activated automatically on first SIWE sign-in.
- Trial users have 1 tracked wallet, real-time monitoring, basic feed.
- Trial users do NOT have export, advanced analytics, or additional tracked wallets.
- At trial expiry without payment: FREE_TRIAL → EXPIRED → (after grace) DORMANT.

**DORMANT → ACTIVE transition:**

- User pays via on-chain subscription contract.
- Backend reconciles `Subscribed` event to `user_id` via `auth_wallets`, transitions account to ACTIVE.
- Tracked wallets re-registered in global indexing (`active_subscriber_count` incremented; `is_globally_active` flipped true if needed).
- Real-time monitoring resumes from current chain head.
- **No backfill of the dormant gap in V1.** UI displays: "Monitoring paused from [date] to [date]. Activity during this period is not available."

**Lifecycle enforcement:**

- Scheduled job (Spring `@Scheduled`, daily cadence) scans for expired subscriptions and transitions account states accordingly.
- EXPIRED → DORMANT grace window: TBD (see Open Questions). Suggested default: 3 days.

### 4.3 Wallet tracking and multi-wallet support

- A user has zero or more **tracked wallets** in `user_tracked_wallets`. These are *separate* from `auth_wallets`.
- The sign-in wallet is auto-registered as the user's first tracked wallet on first sign-in.
- Additional tracked wallets require ownership proof: a SIWE-style signature from that wallet.
- **Free / trial tier:** 1 tracked wallet.
- **Premium tier:** up to 5 tracked wallets.
- Wallet operations: add, remove (soft delete via `is_removed`), pause, resume.
- Removing or pausing decrements `indexed_wallets.active_subscriber_count`; if count reaches zero, `is_globally_active` is set false and the scanner stops scanning that wallet.

### 4.4 Forward-only transaction monitoring

For each globally-active wallet, the listener captures:

- Native ETH transfers (incoming or outgoing).
- USDC (ERC-20 Transfer) events involving the wallet.
- All outgoing transactions from the wallet, regardless of type (to support gas analytics — see §4.10).

Each event is persisted to `wallet_activities` with full provenance: tx hash, block number, block hash, asset deployment, raw amount, event type, direction, counterparty address, gas data, observed-at timestamp, state, confirmation count.

"Forward-only" means: nothing before a wallet's `monitoring_started_at` is indexed in V1.

### 4.5 Global indexing layer

The architectural pattern that makes the platform efficient when multiple users monitor the same wallet.

- `indexed_wallets` is the listener's source of truth for which wallets to scan. Primary key: `(wallet_address, network_id)`.
- Each row carries `active_subscriber_count` (denormalized) and `is_globally_active` (cached for scan filtering).
- Adding a wallet to `user_tracked_wallets` **transactionally** upserts `indexed_wallets` and increments the count.
- Removing / pausing a tracked wallet decrements the count; on zero, `is_globally_active` flips false.
- The listener scans only wallets where `is_globally_active = TRUE`.
- `wallet_activities` rows are shared at read time: API queries join `user_tracked_wallets` to `wallet_activities` on `(wallet_address, network_id)`.
- Result: if 50 users track `0xabc`, the chain is scanned for `0xabc` exactly once. Activity events are stored once and fanned out at read time.

**Concurrency note:** the `active_subscriber_count` increment/decrement MUST occur in the same DB transaction as the `user_tracked_wallets` insert/update. Otherwise concurrent add/remove operations can leak the count.

### 4.6 Transaction state machine

`wallet_activities.state`:

- `seen` — observed in a block, below confirmation threshold.
- `confirmed` — ≥ threshold confirmations reached, treated as durable.
- `reverted` — was previously `seen`; reorg detected, no longer in canonical chain.

Confirmation threshold is per-network (config). Sepolia default: **25**.

### 4.7 Real-time feed

- Dashboard surfaces new activities with current state and confirmation count.

### 4.8 Filtering, search, history view

- Filter by tracked wallet, asset, direction (in/out), date range, state.
- Search by counterparty address or tx hash.

### 4.9 Export (premium)

- CSV and XLSX export of filtered activity. Synchronous execution inside the API thread; reporting is a module within `ledger-api-java`.
- Gated by active **paid** subscription. Trial users do not have export.

### 4.10 Advanced analytics (premium)

All gated by active paid subscription. Computed by live SQL aggregation over `wallet_activities` in V1; pre-computed roll-ups are a V2 optimization.

- **Transaction trends.** Daily, weekly, monthly counts across tracked wallets, broken down by direction.
- **Monthly inflow/outflow.** Per-month aggregate value flowing in and out, broken down by asset.
- **Token usage breakdown.** Share of activity across assets (ETH vs USDC in V1; scales as more `asset_deployments` are added).
- **Gas spending analytics.** Total ETH spent on gas across outgoing transactions, broken down by month and counterparty.

### 4.11 Premium gate via on-chain subscription

- Single Solidity contract deployed on Sepolia (one contract per supported network in V2+).
- **Price:** 30 test-USDC. **Duration:** 30 days.
- ERC-20 two-tx flow: `USDC.approve(contract, 30e6)` then `contract.subscribe()`.
- Re-subscription extends rather than resets: `expiry = max(block.timestamp, currentExpiry) + 30 days`.
- Contract emits `Subscribed(address indexed user, uint256 newExpiry, uint256 amountPaid)`.
- **Payment must originate from the user's sign-in wallet.** Backend reconciles `msg.sender` to a user via `auth_wallets`. Payments from non-auth-wallets are ignored.
- Backend listens for `Subscribed` events (and/or queries on-chain state) to maintain premium access in `subscriptions`.
- On expiry: ACTIVE → EXPIRED → DORMANT per lifecycle (§4.2). Premium features lock; historical data retained.

### 4.12 Operational dashboard

- Listener checkpoint per network (`global_last_scanned_block`)
- Listener lag (chain head − checkpoint), per network
- RPC error rate, retry counts, provider fallback events
- Queue depth (RabbitMQ)
- Reorg event count
- DB write throughput
- API request latency / error rate
- Active subscriptions count, trial conversion rate
- Globally-active wallet count, average subscribers per wallet
- Lifecycle transition counts per state pair

---

## 5. Architecture Overview

### 5.1 Services

| Service | Language | Responsibility |
|---|---|---|
| `listener-go` | Go | Network-agnostic; per-instance configured with one network's RPC and asset addresses. Polls RPC every 60s. Reads `indexed_wallets` for globally-active wallets to scan. Advances from `global_last_scanned_block` to chain head. Captures ETH transfers, USDC transfers, and all outgoing txs for tracked wallets. Emits to MQ. Persists per-network scan progress. Detects reorgs. |
| `ledger-api-java` | Java / Spring Boot | Owns audit ledger, user/auth model, subscription state, lifecycle scheduled jobs, reporting/export module. SIWE nonce + signature verification. Consumes events from MQ. Listens for `Subscribed` events from on-chain contract. Serves REST APIs. |
| `smart-contracts` | Solidity / Foundry | One subscription contract per network. V1 deploys one on Sepolia. |
| `frontend` | React | Dashboard, feed, lifecycle UI (trial countdown, dormant notice), wallet connect (SIWE), subscription flow (approve + subscribe), filters, export trigger. |

**Reporting / export** is a module inside `ledger-api-java`, not a separate service. Synchronous in V1. Extraction trigger: latency > 2s or memory pressure observed.

### 5.2 Real-time data flow

```
Sepolia RPC ── poll ──> listener-go ── per-wallet events ──> RabbitMQ
                            │                                    │
                       per-network                                ▼
                        checkpoint                       ledger-api-java
                          (PG)                                  │
                                                                ▼
                                                        wallet_activities (PG)
                                                                │
                                                          fan-out via
                                                       user_tracked_wallets
                                                                │
                                                                ▼
                                                       React frontend
```

### 5.3 Subscription event path

```
User → USDC.approve()  →  subscriptionContract.subscribe()
                                       │
                                       ▼
                              Subscribed event emitted
                                       │
                       listener-go (or dedicated subscription listener)
                                       │
                                       ▼
                                   RabbitMQ
                                       │
                                       ▼
                            ledger-api-java consumes
                                       │
                  reconciles msg.sender → users.id via auth_wallets
                                       │
                                       ▼
                  subscriptions row upserted
                  account_status → ACTIVE
                  re-activate tracked wallets in indexed_wallets
```

### 5.4 Infrastructure choices and rationale

**RabbitMQ over Kafka.** Source of truth is the chain plus per-network checkpoints; no need for log replay. RabbitMQ is simpler operationally, has a free managed tier (CloudAMQP) if VPS resources are pressured.

**PostgreSQL** for ledger and identity. Strong consistency, mature tooling, single database serves both real-time writes and analytics aggregations at V1 scale.

**Polling listener (60s).** Operationally simpler than WebSocket; checkpoint-based catch-up after restarts; well-suited to per-block batch processing.

**Polyglot service split.** Go for I/O-bound RPC polling and concurrent fan-out. Spring Boot for business logic, lifecycle scheduling, transactional boundaries, and REST.

**Network-agnostic listener code.** A single Go binary takes its network configuration (RPC URL, chain ID, USDC contract address, native asset symbol, confirmation threshold) from env vars / config file. V1 deploys exactly one instance pointed at Sepolia. V2 adds chains by spinning up additional instances configured for those chains. No code change required — only new rows in `networks` and `asset_deployments`, deploying the subscription contract on the new chain, and starting a new listener instance.

---

## 6. Data Model

The full schema is at `docs/schema/ChainOps_Schema.sql`. Summary:

| Table | Purpose |
|---|---|
| `users` | Internal identity (UUID). Holds `account_status`, `current_plan`. |
| `auth_wallets` | Wallets allowed to sign in via SIWE. V1: exactly one per user (enforced in app layer). |
| `user_tracked_wallets` | Wallets a user has registered for monitoring. V1: 1 (free/trial) or 5 (premium). |
| `networks` | Supported chains. V1 seed: Sepolia. |
| `assets` | Asset definitions (ETH, USDC). |
| `asset_deployments` | (asset × network) → contract address + decimals. V1 seed: ETH/Sepolia (native), USDC/Sepolia. |
| `indexed_wallets` | Global listener state. PK: `(wallet_address, network_id)`. Holds `active_subscriber_count`, `is_globally_active`, `global_last_scanned_block`. |
| `wallet_activities` | Audit ledger. Stores observed on-chain events with state, confirmations, block hash, direction, counterparty, gas data. |
| `subscriptions` | Cached projection of on-chain subscription state. On-chain contract is source of truth. |
| `plan_features` | Capability flags per plan. V1: one row (`PREMIUM`). |

### 6.1 Schema-level requirements (V1 blockers)

These fixes to the current draft schema must be applied before any service code is written:

- **`wallet_activities` is missing critical columns.** Add: `state` (seen/confirmed/reverted), `confirmation_count`, `block_hash`, `direction` (in/out), `counterparty_address`, `gas_used`, `effective_gas_price`. Untyped `metadata JSONB` is not a substitute for queryable columns the analytics views depend on.
- **`wallet_activities.amount`** should be `NUMERIC(78, 0)` to hold full uint256 raw on-chain values. Decimals are applied at read time via `asset_deployments.decimals`. Never mix display amounts and raw amounts in the same column.
- **`indexed_wallets` primary key** must be `(wallet_address, network_id)`, not `wallet_address` alone. The same wallet on different chains is a different row.
- **Indexes required:** `wallet_activities (wallet_address, network_id, occurred_at DESC)`, `wallet_activities (tx_hash)` for dedup, `user_tracked_wallets (user_id) WHERE is_removed = FALSE`.
- **`users.primary_wallet_address`** is redundant with `auth_wallets WHERE role='OWNER'`. Drop it; use `auth_wallets` as the source of truth.
- **`subscriptions`** needs a constraint preventing multiple `ACTIVE` rows per user — partial unique index on `(user_id) WHERE subscription_status = 'ACTIVE'`. Or refactor to one row per user with a state column.
- **`user_tracked_wallets.UNIQUE(user_id, wallet_address)`** should be partial: `WHERE is_removed = FALSE`. Otherwise a user cannot re-add a previously-removed wallet.
- **Address and tx_hash columns:** size to EVM exactly (`VARCHAR(42)` and `VARCHAR(66)`) or store as `BYTEA`. The current `VARCHAR(100)` / `VARCHAR(255)` is wasteful and signals non-EVM intent we don't have.
- **`backfill_jobs` table:** remove from V1 schema entirely. Add in V2 alongside the backfill worker. Adding a table later is a pure forward-compatible migration.

---

## 7. Reliability and Observability Requirements

### 7.1 Listener guarantees

- **Resumable on restart.** Per-(wallet, network), `global_last_scanned_block` is persisted after every successful block batch. Listener resumes from `last + 1`.
- **Idempotent emission.** Re-emitting a previously-seen event must not produce a duplicate `wallet_activities` row. Dedup key: `(tx_hash, wallet_address, network_id)` for native transfers; `(tx_hash, log_index, wallet_address, network_id)` for ERC-20 events.
- **Reorg-aware.** Listener tracks a configurable window of recent block hashes. When a previously-emitted block is no longer canonical, it emits a reorg signal; the ledger transitions affected rows to `reverted`.

### 7.2 RPC resilience

- Exponential backoff on transient failures.
- Configurable max retries, per-call timeout (10s default).
- Multi-provider fallback (Alchemy primary, Infura secondary). Single-provider dependency is unacceptable even for the demo.

### 7.3 Metrics (Prometheus)

- `chainops_listener_last_processed_block{network}`
- `chainops_listener_chain_head_lag_blocks{network}`
- `chainops_listener_rpc_errors_total{network,provider,method}`
- `chainops_listener_blocks_processed_total{network}`
- `chainops_listener_reorgs_observed_total{network}`
- `chainops_listener_globally_active_wallets{network}`
- `chainops_mq_publish_errors_total`
- `chainops_mq_queue_depth{queue}`
- `chainops_ledger_writes_total{event_type,state}`
- `chainops_api_request_duration_seconds{route}`
- `chainops_active_subscriptions`
- `chainops_lifecycle_transitions_total{from_state,to_state}`

### 7.4 Logging

- Structured JSON across all services.
- Correlation IDs propagated where applicable.

---

## 8. Deployment Plan

| Component | Hosting |
|---|---|
| Frontend (React) | Vercel |
| `listener-go` (one Sepolia instance) | Single VPS via docker-compose |
| `ledger-api-java` | Single VPS via docker-compose |
| PostgreSQL | Single VPS via docker-compose |
| RabbitMQ | Single VPS via docker-compose |
| Prometheus + Grafana | Single VPS via docker-compose |
| Sepolia RPC | Alchemy free tier (primary) + Infura free tier (fallback) — external |

**VPS sizing:** 4GB RAM, 2 vCPU minimum (Hetzner CX22 recommended).

**Operational tradeoff:** single VPS = SPOF. Acceptable for portfolio demo on testnet. Risk profile is low because Sepolia carries no real-money obligations.

**Local development** uses the same `docker-compose.yml` as production. Differences are environment variables only.

**Kubernetes manifests** under `infra/kubernetes/` are a documented production-deployment artifact, not the runtime substrate for the live demo.

---

## 9. Smart Contract — V1 Design (Sepolia)

**Parameters (locked):**

- `paymentToken`: Sepolia test-USDC contract address.
- `SUBSCRIPTION_PRICE`: 30 USDC (30 * 10^6 accounting for USDC's 6 decimals).
- `SUBSCRIPTION_DURATION`: 30 days (Solidity native literal).
- Single tier.

**Behaviour:**

- User calls `USDC.approve(contract, 30 * 10^6)` then `contract.subscribe()`.
- Contract pulls USDC via `transferFrom`, updates `subscriptionExpiry[msg.sender] = max(block.timestamp, currentExpiry) + 30 days`.
- Emits `Subscribed(address indexed user, uint256 newExpiry, uint256 amountPaid)`.
- Owner-only `withdraw()` via OpenZeppelin `Ownable`.
- Immutable. No proxy, no pausability.
- Checks-Effects-Interactions pattern for reentrancy safety.

**Per-chain consideration:** when V2 deploys to Arbitrum/Base/etc., a new contract instance is deployed on that chain with that chain's USDC address. Backend maps `(network_id, contract_address)` → subscription deployment.

**Out of scope for V1 contract:** refunds, plan upgrades, beneficiary parameter, `permit()` gasless approvals, streaming payments, NFT-bound subscriptions.

---

## 10. Resolved Decisions (log)

- **2026-05-23 — Authentication model:** SIWE-only. UUID `user_id` as primary internal identity.
- **2026-05-23 — Premium gates:** multi-wallet, export, four named analytics views.
- **2026-05-23 — Subscription payment constraint:** must originate from sign-in wallet.
- **2026-05-23 — Multi-wallet ownership proof:** required.
- **2026-05-23 — Listener scope expansion:** captures all outgoing txs from monitored wallets.
- **2026-05-23 — Subscription parameters:** 30 test-USDC for 30 days, single tier, 5-wallet premium / 1-wallet free.
- **2026-05-23 — Subscription contract design:** ERC-20 approve+transferFrom; expiry stacking; immutable; owner withdraw; CEI.
- **2026-05-23 — Backend hosting:** single VPS, docker-compose. K8s as separate artifact.
- **2026-05-23 — Reporting:** module within `ledger-api-java`, synchronous.
- **2026-05-24 — V1 product scope expanded** to include account lifecycle (FREE_TRIAL → ACTIVE → EXPIRED → DORMANT → CANCELED → DELETED), 20-day free trial, global indexing layer with `active_subscriber_count` fan-out, `auth_wallets` ↔ `user_tracked_wallets` separation, and `plan_features` table (one row in V1).
- **2026-05-24 — Multi-chain shape:** schema is multi-chain (`networks`, `assets`, `asset_deployments`); listener code is network-agnostic via config; V1 deploys exactly one listener instance against Sepolia. V2 adds chains via config + new contract deployments; no code changes.
- **2026-05-24 — One auth wallet per user in V1.** Schema permits N rows for V2.
- **2026-05-24 — Backfill cut entirely from V1.** DORMANT users (trial or paid) accept the data gap on reactivation. UI displays gap notification. Backfill is V2.
- **2026-05-24 — DORMANT lifecycle enforced** by a scheduled job (daily cadence). EXPIRED → DORMANT grace window pending (Open Question).

---

## 11. Open Questions

1. **Audit ledger schema final shape.** Apply §6.1 fixes; finalize column set on `wallet_activities` before writing the listener emitter or the ledger writer.
2. **Reorg handling depth.** Maximum reorg depth tracked per network. Window of recent block hashes the listener keeps in memory / persists.
3. **Listener → ledger reorg signal contract.** Message shape for "this previously-emitted block is no longer canonical."
4. **SIWE session lifetime and refresh.** JWT TTL, refresh strategy, revocation on wallet-disconnect.
5. **Subscription event reconciliation lag.** Acceptable latency between `Subscribed` event landing on-chain and backend granting premium access. UX implication: "I paid, why am I not premium yet?"
6. **EXPIRED → DORMANT grace window.** Suggested default: 3 days. Confirm.
7. **Notification surface for state transitions.** In-app feed confirms via dashboard; revisit whether trial-expiry needs proactive prompting in UI.
8. **Concurrency model for `active_subscriber_count` updates.** DB transaction wrapping add/remove + count update is the V1 plan; confirm whether row-level locking is sufficient or optimistic-concurrency retries are needed.

---

## 12. Out of Scope — Roadmap (V2+)

- **Historical backfill** for paid users on DORMANT → ACTIVE transition, via `backfill_jobs` worker queue and rate-limit-aware RPC scheduling.
- **Live multi-chain runtime:** additional listener instances deployed for Arbitrum, Base, Optimism, Ethereum mainnet.
- **Mainnet deployment** with audit, real-money pricing economics, regulatory review, SRE story.
- **Recurring subscription** via allowance-based monthly charges and off-chain keeper.
- **Multi-token support** beyond ETH/USDC (USDT, DAI, WBTC, custom ERC-20s).
- **Email / webhook / Telegram alerts** as notification channels.
- **API access** for third-party integrations.
- **Multiple auth wallets per user** (backup wallet, hardware + hot wallet).
- **Team accounts / multi-tenant organizations.**
- **Mempool monitoring** for pending-tx visibility.
- **User-provided API key vault.**
- **Streaming-payment subscriptions** (Superfluid-style).
- **Pre-computed analytics roll-ups** (materialized views, scheduled aggregation).
- **Account recovery flow** (social recovery, multi-sig owned account).
- **`PAST_DUE` subscription status** when recurring billing is introduced.

---

## 13. Definition of Done for V1

1. A user can sign in via SIWE on Sepolia. Account is created, FREE_TRIAL activated, sign-in wallet auto-registered as tracked wallet.
2. Within 90 seconds of a Sepolia ETH or USDC transfer involving a tracked wallet, the activity appears in the user's feed.
3. Transactions transition `seen → confirmed` and a simulated reorg correctly produces `reverted`.
4. Free / trial user can register at most 1 tracked wallet; attempts beyond this are rejected with a clear UI message and CTA to subscribe.
5. Premium user can register up to 5 tracked wallets, each requiring an ownership-proof signature.
6. Two users tracking the same wallet result in exactly one entry in `indexed_wallets` and one set of `wallet_activities` rows, fanned out to both at read time.
7. Scheduled lifecycle job correctly transitions accounts: FREE_TRIAL → EXPIRED at trial expiry, EXPIRED → DORMANT after grace, ACTIVE → EXPIRED at subscription expiry.
8. DORMANT user reactivating via on-chain subscription is transitioned to ACTIVE; their tracked wallets become globally active again; UI displays the gap notification.
9. Subscription payment from the sign-in wallet is reconciled within the documented latency window.
10. Premium-only features (export, advanced analytics, additional tracked wallets) are rejected for non-premium users with clear messages.
11. Grafana shows all metrics in §7.3 with live data.
12. Full system runs locally via `docker-compose up` from a clean clone.
13. Listener can be reconfigured via env vars to point at a different EVM network without code changes (validated by running it against a second Sepolia RPC endpoint or a local Anvil instance).
14. README explains the architecture, deployment, scope boundaries, and reasoning.
