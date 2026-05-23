# ChainOps — V1 Scope of Work

**Status:** Draft
**Owner:** Vignesh Sundarajan
**Last updated:** 2026-05-23

---

## 1. Overview

ChainOps V1 is a forward-only Ethereum wallet monitoring and operational reporting platform deployed against the **Sepolia testnet**. Users register a wallet address; from the moment of registration onward, ChainOps observes on-chain activity for that wallet, indexes it into a durable audit ledger, and exposes it through a dashboard, search/filter views, and exportable reports.

The V1 is deliberately scoped to demonstrate **production-minded blockchain infrastructure thinking** — distributed systems design, event-driven architecture, async processing, observability, and operational reliability — without the operational burden of a fully indexed historical product. It is an open-source, publicly hostable MVP intended as a portfolio-grade demonstration of architectural decision-making.

---

## 2. Product Goals

V1 succeeds if the following are true:

- A user can register an Ethereum wallet on Sepolia and, within ~60 seconds of any subsequent ETH or USDC transfer involving that wallet, see the transaction appear in their dashboard.
- The transaction state transitions correctly from _pending_ through _confirmed_ to _finalized_, and reorgs during the unconfirmed window are handled without corrupting the audit ledger.
- A user can filter, search, and export their wallet's activity to CSV/XLSX.
- A premium feature gate is enforced via an on-chain subscription contract, demonstrating Web3-native access control.
- The system exposes meaningful operational metrics (listener lag, RPC error rate, queue depth, reorg counts) via Prometheus, with Grafana dashboards.
- The full stack is reproducible locally via `docker-compose up`, and deployable to free/cheap hosting tiers.

---

## 3. Explicit Non-Goals (V1)

These are out of scope and **must not be built** in V1. They exist as a forcing function against feature creep.

- **No historical backfill.** ChainOps does not index transactions that occurred before a wallet was registered. The product story is "flight recorder from the moment you plug in."
- **No mainnet support.** Sepolia only.
- **No multi-chain support.** Ethereum (Sepolia) only.
- **No third-party indexer integration** (Alchemy/Etherscan/Covalent APIs are not used for historical lookups in V1).
- **No user-provided API key vault.** ChainOps does not custody user credentials for external services.
- **No mempool / pending-pre-block monitoring.** Only mined blocks are observed.
- **No recurring on-chain subscriptions.** The subscription contract is one-shot, fixed-duration access. No keeper, no allowance-based auto-debit.
- **No Kubernetes as the deployment substrate.** K8s manifests are produced as a separate `infra/kubernetes/` artifact for portfolio purposes but are not how the live demo is hosted.
- **No tokens beyond ETH and USDC.**
- **No email/SMS notifications.** In-app notifications only (real-time feed counts as the notification surface in V1).
- **No custodial behaviour.** ChainOps never holds user private keys. Wallet connection is read-only.
- **No email / password / OAuth authentication.** Authentication is wallet-based only via Sign-In With Ethereum (SIWE, EIP-4361). No password storage, no email verification flow, no third-party OAuth providers.
- **No account recovery.** Loss of access to the sign-in wallet means loss of access to the ChainOps account. This is a deliberate property of the SIWE-only auth model, not a bug.

---

## 4. In-Scope Features

### 4.1 Identity and authentication (SIWE)

- Authentication is Sign-In With Ethereum (EIP-4361). The user connects their wallet, requests a nonce from the backend, signs a structured message containing the nonce, and the backend verifies the signature.
- The wallet address that signs the message becomes the user's **sign-in wallet** (also called the "primary wallet").
- Internal user identity is an opaque `user_id` (UUID). The sign-in wallet is stored as an auth method attached to the `user_id`, not as the primary key. This preserves optionality for adding other auth methods in V2+ without re-architecting the schema.
- Session is issued as a short-lived JWT tied to the `user_id`.

### 4.2 Wallet registration and multi-wallet support

- A user has one sign-in wallet and zero-or-more additional **monitored wallets**.
- The sign-in wallet is automatically registered as a monitored wallet on first sign-in.
- To register an additional monitored wallet, the user must **prove ownership** by signing a SIWE-style challenge from that wallet. This is enforced — V1 does not allow monitoring of wallets the user cannot sign for. Rationale: this positions ChainOps as a personal/treasury tool rather than a surveillance tool over arbitrary public addresses.
- The system records the registration block height per wallet as that wallet's monitoring start point. Activity before registration is not indexed.
- **Free tier:** 1 monitored wallet (the sign-in wallet).
- **Premium tier:** up to 5 monitored wallets (sign-in wallet + 4 additional).

### 4.3 Forward-only transaction monitoring

- Native ETH transfers (incoming or outgoing) involving any registered wallet are captured.
- USDC (ERC-20 Transfer events) involving any registered wallet are captured. The Sepolia USDC contract address is configured statically.
- **Any outgoing transaction** (i.e., where `from` is a monitored wallet) is also captured, regardless of whether it is a transfer. This is required to power gas-spending analytics (§4.8). The ledger records these as a separate event class with the gas cost attached.
- Each captured event is persisted to the audit ledger with full provenance: tx hash, block number, block hash, from, to, amount (where applicable), gas used, effective gas price, event class, observed-at timestamp.

### 4.4 Transaction state machine

Transactions move through explicit states:

- `seen` — observed in a block, < threshold confirmations
- `confirmed` — ≥ 25 confirmations reached, treated as durable
- `reverted` — was previously `seen`, now no longer present in the canonical chain (reorg)

The confirmation threshold is configurable; **25 is the default**.

### 4.5 Real-time feed

- Dashboard shows new transactions as they arrive, with their current confirmation count and state.

### 4.6 Filtering, search, history view

- Filter by wallet (when multi-wallet), token (ETH/USDC), direction (in/out), date range, state.
- Search by counterparty address or tx hash.

### 4.7 Export (premium)

- CSV and XLSX export of filtered transaction history.
- Gated by active subscription.

### 4.8 Advanced analytics (premium)

All gated by active subscription. All views are computed by aggregation over the audit ledger.

- **Transaction trends.** Daily, weekly, and monthly counts of transactions across the user's monitored wallets, with breakdowns by direction.
- **Monthly inflow/outflow.** Per-month aggregate value flowing into and out of the user's monitored wallets, broken down by token.
- **Token usage breakdown.** Share of activity across tokens (ETH vs USDC in V1). Acknowledged as a thin view at the V1 two-token scope; designed to scale meaningfully when additional tokens are added in V2+.
- **Gas spending analytics.** Total ETH spent on transaction fees across all outgoing transactions from the user's monitored wallets, broken down by month and by destination contract / counterparty. Requires the listener to capture all outgoing txs from monitored wallets (see §4.3).

**Aggregation strategy for V1:** live SQL queries against the ledger, executed on request. Materialized views and pre-computed roll-ups are an explicit V2+ optimization, deferred until volume warrants them.

### 4.9 Premium gate via on-chain subscription

- A single Solidity contract deployed on Sepolia. One subscription tier only.
- **Price:** 30 test-USDC. **Duration:** 30 days per subscription. Both are contract constants, not user-configurable.
- Users call `subscribe()` which pulls 30 test-USDC via ERC-20 `transferFrom` (requires prior `approve` call by the user — two-tx flow accepted as the standard ERC-20 baseline).
- **Re-subscription extends, not resets.** If a user re-subscribes while still active, the new 30 days are added to their existing expiry. Implementation: `expiry = max(block.timestamp, currentExpiry) + 30 days`.
- **Subscription payment must originate from the user's sign-in wallet.** The contract emits `Subscribed(msg.sender, newExpiry, amountPaid)`; the backend maps `msg.sender` to a user via the sign-in-wallet record. If the paying wallet is not anyone's sign-in wallet, the event is ignored.
- ChainOps backend listens for `Subscribed` events (or queries on-chain state) to determine premium access.
- **On subscription expiry (soft-revoke):** premium-gated features (export, analytics, additional wallets) become inaccessible, but historical data is retained and the listener continues to index all previously-registered wallets. The user can re-subscribe to instantly restore access. No grace period, no deletion. Rationale: cheapest to build, kindest to the user, easiest to upgrade to harder revocation later.
- **Premium gates in V1:** multi-wallet (beyond 1), export (§4.7), advanced analytics (§4.8).
- **Pricing note:** on Sepolia, test-USDC has no economic value. The contract is a demonstration of subscription mechanics, not a revenue mechanism. A real-economy version would belong on mainnet or an L2 with proper pricing logic — out of scope.

### 4.10 Operational dashboard

- Listener checkpoint (last block processed)
- Listener lag (chain head height − checkpoint)
- RPC error rate, retry counts, provider fallback events
- Queue depth (RabbitMQ)
- Reorg event count
- DB write throughput
- API request latency / error rate
- Active subscriptions count

---

## 5. Architecture Overview

### 5.1 Services

| Service           | Language           | Responsibility                                                                                                                                                                                                                                                                                                                                                                                     |
| ----------------- | ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `listener-go`     | Go                 | Polls Sepolia RPC every 60s, advances from last checkpoint to chain head, extracts ETH and USDC events plus all outgoing txs for registered wallets, emits to MQ, persists checkpoint.                                                                                                                                                                                                             |
| `ledger-api-java` | Java / Spring Boot | Owns the audit ledger and user/auth model. Consumes events from MQ. Serves REST APIs to the frontend. Handles SIWE nonce issuance and signature verification. Enforces transaction state machine and reorg handling. Listens for `Subscribed` events from the on-chain contract to maintain premium-access state. **Reporting/export is a module within this service**, not a separate deployment. |
| `smart-contracts` | Solidity / Foundry | Subscription contract deployed on Sepolia. Verified on Etherscan.                                                                                                                                                                                                                                                                                                                                  |
| `frontend`        | React              | Dashboard, feed, filters, export trigger, wallet connection.                                                                                                                                                                                                                                                                                                                                       |

**Reporting module note:** Export runs synchronously inside the API thread for V1. If export latency exceeds ~2 seconds or memory pressure is observed, the upgrade path is to extract the module into a separate service with async job semantics (job ID returned, file generated in background, download endpoint exposed). Document this trigger condition in service runbook.

### 5.2 Data flow (happy path)

```
Sepolia RPC ── poll ──> listener-go ── emit ──> RabbitMQ
                            │                       │
                       checkpoint                   ▼
                          (PG)               ledger-api-java
                                                    │
                                                    ▼
                                            audit ledger (PG)
                                                    │
                                                    ▼
                                            React frontend
```

### 5.3 Infrastructure choices and rationale

**RabbitMQ over Kafka.** The durable source of truth is the blockchain itself plus the listener's persisted checkpoint. If a downstream consumer crashes, the listener can re-emit from chain history; we do not need Kafka's log-replay semantics. RabbitMQ is operationally simpler, has a usable free managed tier (CloudAMQP), and fits the throughput profile of an MVP. **Revisit if** event volume meaningfully exceeds 100/sec, or if a new consumer needs to replay historical events without re-reading the chain.

**PostgreSQL for the ledger.** Strong consistency, mature tooling, fits the audit-grade requirement, easy to run locally and in managed form (Supabase / Neon free tiers).

**In-memory caching only in V1.** No Redis. Defer cache layer until there is measured read pressure on an actual hot path. Premature caching is premature complexity.

**Polling-based listener (60s) rather than WebSocket subscriptions.** Polling is operationally simpler — no reconnect logic, no dropped-message handling, no stateful subscription to manage across restarts. The listener's contract is "advance from `last_processed_block + 1` to current head on every tick," which makes restarts and catch-up trivial.

**25-confirmation finality threshold.** Conservative default consistent with industry practice for non-trivial value transfers on Ethereum. Configurable per deployment.

**Polyglot service split (Go listener + Java API).** Go is well-suited to I/O-bound RPC polling and concurrent fan-out via goroutines/channels. Spring Boot offers mature ergonomics for the business-logic and audit layer (validation, transactional boundaries, REST conventions, observability integrations). The split is deliberate — it's there to demonstrate sensible polyglot service composition, not because it's required by throughput.

---

## 6. Reliability and Observability Requirements

### 6.1 Listener guarantees

- **Resumable on restart.** Listener persists `last_processed_block` after every successful block batch. On boot, it resumes from `last_processed_block + 1`.
- **Idempotent emission.** Re-emitting an event already seen by the ledger must not produce a duplicate ledger row. Idempotency key: `(tx_hash, log_index)` for ERC-20 events; `tx_hash` for native transfers.
- **Reorg-aware.** When the listener observes that a previously-emitted block is no longer in the canonical chain, it emits a corresponding `reverted` signal so the ledger can transition affected rows.

### 6.2 RPC resilience

- Exponential backoff on transient RPC failures.
- Configurable max-retries per RPC call.
- Per-call timeout (e.g., 10s).
- If a primary RPC endpoint fails repeatedly, fall through to a configured secondary (e.g., Alchemy → Infura). **Multi-provider fallback is an MVP feature**, not a stretch goal — single-provider dependency is an unacceptable operational risk even for a demo.

### 6.3 Metrics (Prometheus)

- `chainops_listener_last_processed_block`
- `chainops_listener_chain_head_lag_blocks`
- `chainops_listener_rpc_errors_total{provider, method}`
- `chainops_listener_blocks_processed_total`
- `chainops_listener_reorgs_observed_total`
- `chainops_mq_publish_errors_total`
- `chainops_mq_queue_depth{queue}`
- `chainops_ledger_writes_total{state}`
- `chainops_api_request_duration_seconds{route}`

### 6.4 Logging

- Structured (JSON) logs across all services.
- Correlation IDs propagated from API request → ledger → (where applicable) listener context.

---

## 7. Deployment Plan

| Component                                     | Hosting                                                                                             |
| --------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| Frontend (React)                              | Vercel                                                                                              |
| `listener-go`                                 | Single VPS via docker-compose                                                                       |
| `ledger-api-java` (includes reporting module) | Single VPS via docker-compose                                                                       |
| PostgreSQL                                    | Single VPS via docker-compose (managed Supabase/Neon as fallback if VPS resource pressure observed) |
| RabbitMQ                                      | Single VPS via docker-compose                                                                       |
| Prometheus + Grafana                          | Single VPS via docker-compose                                                                       |
| Sepolia RPC                                   | Alchemy free tier (primary) + Infura free tier (secondary fallback) — external                      |

**VPS sizing target:** 4GB RAM, 2 vCPU minimum. Recommendation: Hetzner CX22 (≈€4-5/month) as the boring correct choice. The full stack (Postgres + RabbitMQ + Spring Boot + Go listener + Prometheus + Grafana) does not fit comfortably on a 1-2GB box — Spring Boot alone can consume ~500MB of heap.

**Operational tradeoff (accepted):** single VPS means single point of failure and no high availability. If the box crashes the entire system is down until it recovers. This is acceptable for a portfolio MVP; documented here so it is not mistaken for an oversight.

**Local development** is via the same `docker-compose.yml` used in production. Local and prod environments differ only in environment variables (RPC endpoints, secrets, DB credentials). This deliberate symmetry is the operational simplicity that justified choosing single-VPS over a managed PaaS.

**Kubernetes manifests** live under `infra/kubernetes/` as a documented production-deployment artifact, but the live demo does not run on K8s. This separation is intentional: the manifests demonstrate the deployment architecture; docker-compose runs the demo without the operational overhead.

---

## 8. Smart Contract — Minimal V1 Design

The V1 subscription contract is intentionally one of the smallest contracts that can serve a real product purpose.

**Parameters (locked):**

- `paymentToken`: Sepolia test-USDC contract address (static constant)
- `SUBSCRIPTION_PRICE`: 30 USDC (30 \* 10^6, accounting for USDC's 6 decimals)
- `SUBSCRIPTION_DURATION`: 30 days (Solidity native `30 days` literal)
- Single tier — no plan upgrades, no proration

**Behaviour:**

- A user first calls `USDC.approve(subscriptionContract, 30 * 10^6)` to authorize transfer (standard ERC-20 two-tx pattern, accepted UX cost for V1).
- The user then calls `subscribe()` on the subscription contract. The contract pulls 30 USDC via `transferFrom(msg.sender, address(this), 30 * 10^6)` and updates the user's expiry: `subscriptionExpiry[msg.sender] = max(block.timestamp, subscriptionExpiry[msg.sender]) + 30 days`.
- The contract emits `Subscribed(address indexed user, uint256 newExpiry, uint256 amountPaid)`.
- The backend listens for `Subscribed` events and reconciles them against known sign-in wallets to grant premium access at the account level.
- **Payment must originate from the user's sign-in wallet.** Subscriptions paid from any other address are unattributable in V1 and are ignored by the backend.
- No keeper. No automatic renewal. When the access window expires, the user re-subscribes manually (soft-revoke; see §4.9).

**Contract design constraints:**

- **Immutable.** No proxy, no upgradeability. If logic changes, deploy a new contract address and migrate.
- **No pausability.** Adds complexity without clear V1 benefit. If a bug is found, deploy a replacement.
- **Owner-only `withdraw()`.** Collected USDC can be withdrawn by the contract owner. Use OpenZeppelin `Ownable`. Owner = deployer wallet for V1.
- **Checks-Effects-Interactions pattern** for reentrancy safety, even though USDC is a known-good token. Cheap discipline.

**Out of scope for the V1 contract:** refunds, plan upgrades, beneficiary parameter, gasless approvals via `permit()`, streaming payments, NFT-bound subscriptions.

---

## 9. Resolved Decisions (log)

For auditability and to prevent silent drift, decisions that have been made and locked are recorded here.

- **2026-05-23 — Authentication model:** SIWE-only. No email / OAuth. Sign-in wallet doubles as account identity (mapped via UUID `user_id` internally).
- **2026-05-23 — Premium gates:** multi-wallet (beyond 1), export, and four named advanced analytics views (transaction trends, monthly inflow/outflow, token usage breakdown, gas spending).
- **2026-05-23 — Subscription payment constraint:** must originate from the user's sign-in wallet. Payments from other addresses are ignored.
- **2026-05-23 — Multi-wallet ownership proof:** required. Adding a monitored wallet beyond the sign-in wallet requires a SIWE-style signature from that wallet.
- **2026-05-23 — Listener scope expansion:** listener captures all outgoing transactions from monitored wallets (not just ETH/USDC transfers) to support gas analytics.
- **2026-05-23 — Subscription parameters:** 30 test-USDC for 30 days, single tier, 5-wallet premium limit (1 for free).
- **2026-05-23 — Subscription contract design:** ERC-20 approve+transferFrom two-tx flow; re-subscription extends rather than resets expiry; immutable contract (no proxy, no pausability); owner-only `withdraw()` via OpenZeppelin `Ownable`; CEI pattern for reentrancy safety.
- **2026-05-23 — Subscription expiry behaviour:** soft-revoke (premium features lock, indexing continues, no data deletion, no grace period).
- **2026-05-23 — Backend hosting:** single VPS (target 4GB RAM, Hetzner CX22 recommended), docker-compose for runtime. K8s manifests are a separate portfolio artifact, not the live deployment substrate.
- **2026-05-23 — Reporting:** module within `ledger-api-java`, synchronous export inside the API thread. Extraction trigger: export latency > ~2s or observed memory pressure.

## 10. Open Questions

These are decisions deliberately left unresolved. They must be answered before the relevant component is implemented.

1. **Audit ledger schema.** Single `events` table with type discriminator (transfer-eth / transfer-usdc / outgoing-tx) vs per-class tables. Tradeoffs around query shape, analytics aggregation cost, and future multi-token support.
2. **Reorg handling depth.** Maximum reorg depth the listener tracks. Sepolia is generally stable but the listener still needs a defined window of recent block hashes for comparison.
3. **Notification channel in V1.** In-app feed only is the default; revisit whether a polled "unread" count is sufficient or whether SSE/WebSocket push to the frontend is needed.
4. **Listener → ledger reorg signal contract.** Message shape for reorg events: how does the listener tell the ledger "this previously-emitted block is no longer canonical"?
5. **SIWE session lifetime and refresh.** JWT TTL, refresh strategy, revocation on wallet-disconnect.
6. **Subscription event reconciliation lag.** What is the acceptable latency between a `Subscribed` event landing on-chain and the backend granting premium access? Directly tied to the listener's polling cadence and event-fan-out path.

---

## 11. Out of Scope — Roadmap (V2+)

These features were considered for V1 and deferred. They are listed here so that V1 architectural decisions do not foreclose them.

- **Historical backfill** for newly registered wallets, powered by a third-party indexer (Alchemy/Etherscan). Architecturally this requires a separate backfill service with its own RPC budget and rate-limit-aware scheduling, isolated from the real-time listener.
- **Recurring subscription** via allowance-based monthly charges and an off-chain keeper service.
- **Multi-chain support** (Polygon, Arbitrum, Base).
- **Mainnet support**, including pricing in real USD and proper finality thresholds per chain.
- **Email/webhook notifications** for transaction events.
- **API access** for third-party integrations.
- **Multi-token support** beyond ETH/USDC (USDT, DAI, WBTC, custom ERC-20s).
- **Streaming-payment subscriptions** (Superfluid-style) as an alternative to one-shot purchases.
- **Mempool monitoring** for sub-block-time visibility on pending transactions.
- **User-provided API key vault** for advanced users who want to plug in their own Alchemy/Infura keys.
- **Email or OAuth authentication** as an alternative or supplement to SIWE.
- **Subscription paid from a non-sign-in wallet**, via either a `beneficiary` parameter on the contract or an off-chain wallet-linking flow.
- **Pre-computed analytics roll-ups** (materialized views or scheduled aggregation jobs) for performance when ledger size and query volume grow.
- **Account recovery flow** (e.g., social recovery, multi-sig owned account).

---

## 12. Definition of Done for V1

V1 is considered done when:

1. A user can sign in to the deployed frontend via SIWE using a Sepolia wallet.
2. The sign-in wallet is auto-registered as a monitored wallet; a free-tier user can register no additional wallets.
3. A test ETH transfer to a monitored wallet appears in the user's feed within 90 seconds of being mined.
4. A test USDC transfer to a monitored wallet appears in the user's feed within 90 seconds of being mined.
5. The transaction's confirmation count visibly progresses and reaches the `confirmed` state at 25 confirmations.
6. A simulated reorg (or an observed real one) results in the affected transaction transitioning to `reverted` without ledger corruption.
7. The user can filter and search their wallet's history. Export is rejected for non-premium users with a clear message.
8. A user can subscribe via the on-chain contract using their sign-in wallet, and within a documented latency window the backend grants premium access.
9. A premium user can register up to N monitored wallets (each requiring ownership-proof signature) and access export plus all four advanced analytics views.
10. Grafana shows all metrics listed in §6.3 with live data, including active subscription count.
11. The full system can be brought up locally via `docker-compose up` from a clean clone.
12. README explains the architecture, the deployment, the scope boundaries, and the reasoning behind the key technical decisions.
