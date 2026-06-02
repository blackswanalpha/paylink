# idempotency-go (work17)

LinkMint's shared **Idempotency-Key** store + **consumer-dedupe** helpers for Go/chi services. The
Python twin is [`idempotency-python`](../idempotency-python) (`linkmint_idempotency`) — identical Redis
key scheme (`idem:<service>:<route>:<key>`) and JSON record shape, so Go and Python services share one
Redis. This doc is the **guidance doc** for the whole framework (kept in sync with the Python README).

It is the **application-layer complement** to the chain's on-chain anti-replay (invariant **A.7**),
which stays the source of truth for settlement — see [A.7 alignment](#a7-alignment).

## Install

```
require github.com/paylink/idempotency-go v0.0.0
replace github.com/paylink/idempotency-go => ../idempotency-go   // path relative to your go.mod
```

## HTTP store (replay-safe state-mutating endpoints)

```go
idem := idempotency.New(redisClient, config.ServiceName, 24*time.Hour)

fp := idempotency.Fingerprint(rawBody)
cached, err := idem.Begin(ctx, "create", idemKey, fp)
if err != nil {
    // map the lib error to your envelope:
    if errors.Is(err, idempotency.ErrConflict) { return httpx.NewError(httpx.CodeIdempotentConflict, err.Error(), nil) }
    return httpx.NewError(httpx.CodeInternalError, err.Error(), nil)
}
if cached != nil { /* replay */ return writeJSON(w, cached.Status, cached.Body) }
// ... do the work ...
if err != nil { idem.Release(ctx, "create", idemKey); return err } // free the key for a corrected retry
_ = idem.Complete(ctx, "create", idemKey, fp, status, body)
```

`Begin` returns `(nil, nil)` when you own the key, `(cached, nil)` to replay, or a `*ConflictError`
(`errors.Is(err, ErrConflict)`) when the same key arrives with a **different body** (`body_mismatch`) or
while a first request is still **in flight**. The library never imports `httpx`/`chi`/`config`; the
HTTP-status mapping lives at your service boundary.

## Consumer dedupe (at-least-once bus → no double effect)

The bus (work15) is at-least-once, so handlers must be idempotent. The `HandleFunc` only delivers
`(name, payload)` — not the envelope id — so **you** supply a stable dedupe key (a `proof_hash`, a
business id, or a payload fingerprint).

- **`RedisDedupe`** — cheap best-effort short-circuit (skip repeated non-DB work):

  ```go
  dd := idempotency.NewRedisDedupe(redisClient, config.ServiceName, 7*24*time.Hour)
  err := dd.RunOnce(ctx, "proof.settled", proofHash, func() error { return doExpensiveSideEffect() })
  ```

- **`DbDedupe`** — durable exactly-once *effect* (the marker commits with your write):

  ```go
  dd := idempotency.NewDbDedupe("processed_events") // create the table from processed_events.sql
  ran, err := dd.RunOnce(ctx, tx, "proof.settled", proofHash, func() error { return applyOnTx(tx) })
  // commit tx: the dedupe row + the effect are atomic
  ```

## When to use which — the four layers

| Layer | Mechanism | Guarantee | Use when |
|------|-----------|-----------|----------|
| HTTP `Idempotency-Key` | `Store` (Redis, 24h TTL) | replay-safe response per (service,route,key) | a client may retry a state-mutating POST |
| `RedisDedupe` | Redis SETNX marker | best-effort (TTL/eviction-bounded) | skip repeating **non-DB** work for a seen event |
| `DbDedupe` + per-flow `UNIQUE` | Postgres row on the handler's tx | **exactly-once effect**, durable | the handler **writes a row** — this is the arbiter |
| on-chain `proof_hash` | chain state (A.7) | one tx settles one PayLink | settlement — already anti-replayed on-chain |

**Decision rule:** writes a row → DB `UNIQUE` (`DbDedupe` or a domain unique index); expensive non-DB
work → `RedisDedupe`; a client may retry → `Idempotency-Key`; settlement → already anti-replayed
on-chain, don't re-guard it.

## Per-flow uniqueness (the durable arbiters in this system)

From `backendfeatures.md` — each flow's exactly-once guarantee is a DB/chain constraint, not the cache:

| Flow | Durable key |
|------|-------------|
| PayLink create | deterministic `pl_id` (chain re-submission is a no-op) |
| Payment / proof | `payment.payments.proof_hash UNIQUE` + `proof.proofs.proof_hash` PK |
| Refund / dispute | `refund_id` per payment; `rail_dispute_id UNIQUE` |
| Settlement / payout | settlements `UNIQUE (merchant_id, period_start, period_end, currency)`; payout UUID + `Idempotency-Key` |
| Notification delivery | `payload->>'dedupe_key'` UNIQUE (`deliveries_dedupe_uidx`) |
| Fraud decision | cached `(user_id, device_fp, amount, pl_id)` for 60s |

## Fingerprint contract & 409 semantics

`Fingerprint` is a stable SHA-256 over the **canonical** request body. Callers MUST send a stable body
for a given `Idempotency-Key`: the same key with a different body is always a `409 IDEMPOTENT_CONFLICT`
(never a silent replay of the wrong response). A first request still in flight is also a 409.

## A.7 alignment

This framework **never gates settlement** on its Redis/DB marker. The on-chain proof-hash check is the
source of truth for "already settled"; these helpers only make retries cheap and stop duplicate
**app-side** effects (a double HTTP response, a double notification, a double ledger leg) **before** a
request reaches the chain. If Redis is down or a key expires, the worst case is a duplicate attempt that
the chain's anti-replay (and the `proof_hash` DB UNIQUE in payment/proof) still rejects — it **fails
safe toward on-chain truth**. It touches neither fund flow (A.1) nor ledger balancing (A.6).

## Test

```
make test          # unit (no Docker)
make test-integration   # + testcontainers redis:7 / postgres:16
make cover         # combined; DoD gate >= 80%
make lint          # go vet + gofmt
```
