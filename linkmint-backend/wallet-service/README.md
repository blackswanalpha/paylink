# wallet-service (work24)

A **read-side** surface over on-chain PLN for the LinkMint custom chain (lVM). It indexes `chain.*`
events into the `wallet` schema and serves balances, transaction history, staking positions/rewards,
and treasury stats — plus a **non-custodial** staking-intent endpoint that returns an *unsigned*
transaction.

**Non-custodial (A.1):** the service never holds private keys or funds. Balances are chain truth
(read-through cached); staking intents are returned **unsigned** for the client to sign. The
build→sign→broadcast send path is out of scope (that is work34).

- **Stack:** Go / chi · **Port:** `8102` · **Schema:** `wallet` (one per service; no cross-schema FKs)
- **Consumes:** Kafka topic `chain` (group `wallet-service`). **Publishes:** nothing.
- **Reuses:** `pkg/lvm` (unsigned-tx builders, byte-exact wire format), the lVM JSON-RPC
  (`paylink_getAccount`/`paylink_getNonce`/`paylink_tokenStats`/`paylink_chainInfo`), `eventbus-go`,
  `idempotency-go` (DbDedupe), `telemetry-go`. **No `ledger-go`** — records no money flows.

## API (`/v1`, standard error envelope)

| Method & path | Auth | Description |
|---|---|---|
| `GET /v1/wallets/{addr}` | self-scoped | Balance + nonce (read-through RPC cache; `stale:true` when served from cache with the chain down) |
| `GET /v1/wallets/{addr}/transactions?limit=&cursor=` | self-scoped | Paginated movement history (newest first) |
| `GET /v1/staking/positions?addr=` | self-scoped | Staking position (staked / pending / rewards / slashed / active) |
| `GET /v1/staking/rewards?addr=&limit=&cursor=` | self-scoped | Append-only reward history |
| `POST /v1/staking/intent` | self-scoped | **Unsigned** stake/unstake tx + fee estimate (A.1 — no keys) |
| `GET /v1/treasury/stats` | **public** | Supply / burn / fee / validator-reward / treasury aggregates |

Plus `GET /internal/healthz`, `GET /internal/readyz` (postgres is the only hard dep; the chain is
soft — reported `degraded` while the read-side keeps serving), and `GET /metrics`.

### `/v1/staking/intent`

Request: `{ "addr": "0x…", "action": "stake" | "unstake", "amount": "<decimal string>" }`

Response: `{ unsigned_tx, signable_bytes (base64), signable_bytes_hex, nonce, chain_id, fee_estimate }`.
The `unsigned_tx` has empty `signature`/`pubKey` and a zero `hash` — the client computes
`SHA256(signable_bytes)`, signs it (P-256), attaches its pubkey, and broadcasts via work34. Stake/unstake
txs carry no protocol fee today, so `fee_estimate.amount` is `"0"` (the shape is forward-compatible).

## Chain indexer (consumer)

Each `chain.*` event is projected idempotently (DbDedupe on the same tx as the write):

| Event | Effect |
|---|---|
| `chain.account.transfer` | two `transactions` rows (out on sender, in on receiver) |
| `chain.validator.staked` | `staking_positions` (staked + active) + stake history |
| `chain.validator.unstake_started` | stake → pending + cooldown; history |
| `chain.validator.unstake_completed` | clear pending; history |
| `chain.validator.slashed` | reduce stake to remainder + accrue slashed; history |
| `chain.validator.rewarded` | cumulative rewards + reward history (`validator_reward`) |
| `chain.fee.collected` | treasury fee / validator-reward / treasury deltas |
| `chain.fee.distributed` | per-validator fee-share reward history (`fee_share`) |
| `chain.token.burned` | authoritative cumulative burn |

## Develop

```sh
make build          # go build ./...
make test           # unit tests (no Docker)
make cover          # unit + integration (testcontainers postgres); DoD gate >=80%
make lint           # go vet + gofmt -l
make run            # listens on :8102 (reads env; see .env.example)
```

See `DESIGN.md` for the architecture and the design decisions (balance read-through cache, non-custodial
intent path, treasury accounting).
