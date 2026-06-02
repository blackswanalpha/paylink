# idempotency-python (work17)

LinkMint's shared **Idempotency-Key** store + **consumer-dedupe** helpers for Python/FastAPI services
(`linkmint_idempotency`). The Go twin is [`idempotency-go`](../idempotency-go) — identical Redis key
scheme (`idem:<service>:<route>:<key>`) and JSON record shape, so Go and Python services share one
Redis. The Go [README](../idempotency-go/README.md) carries the canonical guidance tables; this doc
mirrors them with Python snippets.

It is the **application-layer complement** to the chain's on-chain anti-replay (invariant **A.7**),
which stays the source of truth for settlement — see [A.7 alignment](#a7-alignment).

## Install

Services vendor it via their Dockerfile (build context = repo root), like `linkmint-eventbus`:

```dockerfile
COPY linkmint-backend/idempotency-python /idempotency-python
RUN pip install /idempotency-python
```

## HTTP store (replay-safe state-mutating endpoints)

Construct once on `app.state` (the service passes its own `redis.asyncio` client — it satisfies the
`RedisLike` protocol structurally):

```python
from linkmint_idempotency import IdempotencyStore, fingerprint
app.state.idempotency = IdempotencyStore(app.state.redis, service="paylink-service", ttl_seconds=86400)
```

In a router:

```python
fp = fingerprint(req.model_dump(mode="json"))
if idempotency_key:
    cached = await idem.begin("create", idempotency_key, fp)
    if cached is not None:
        return JSONResponse(status_code=cached.http_status, content=cached.body)
try:
    row = await service.create(cmd)
except Exception:
    if idempotency_key:
        await idem.release("create", idempotency_key)   # free the key for a corrected retry
    raise
if idempotency_key:
    await idem.complete("create", idempotency_key, fp, 201, body)
```

`begin` raises `IdempotencyConflict` (reason `body_mismatch` | `in_flight`) which the service maps to
`409 IDEMPOTENT_CONFLICT` via one FastAPI exception handler:

```python
@app.exception_handler(IdempotencyConflict)
async def _on_conflict(_: Request, exc: IdempotencyConflict) -> JSONResponse:
    return error_response(ErrorCode.IDEMPOTENT_CONFLICT, str(exc))   # 409, standard envelope
```

The library imports no FastAPI — the status mapping lives at the service boundary.

## Consumer dedupe (at-least-once bus → no double effect)

The bus (work15) is at-least-once and the `handle(name, payload)` chokepoint gets no envelope id, so
**you** supply a stable dedupe key (a `proof_hash`, a business id, or `fingerprint(payload)`).

```python
from linkmint_idempotency import RedisDedupe, DbDedupe

# Cheap best-effort short-circuit (skip repeated non-DB work):
dd = RedisDedupe(app.state.redis, service="notification-service", ttl_seconds=7 * 86400)
await dd.run_once("payment.settled", proof_hash, lambda: do_side_effect())

# Durable exactly-once effect (marker commits with the handler's write):
dd = DbDedupe("processed_events")   # table from migrations/processed_events.sql
async with sessionmaker.begin() as conn:
    ran, _ = await dd.run_once(conn, "payment.settled", proof_hash, lambda: apply(conn))
```

## When to use which — the four layers

| Layer | Mechanism | Guarantee | Use when |
|------|-----------|-----------|----------|
| HTTP `Idempotency-Key` | `IdempotencyStore` (Redis, 24h TTL) | replay-safe response per (service,route,key) | a client may retry a state-mutating POST |
| `RedisDedupe` | Redis SETNX marker | best-effort (TTL/eviction-bounded) | skip repeating **non-DB** work for a seen event |
| `DbDedupe` + per-flow `UNIQUE` | Postgres row on the handler's tx | **exactly-once effect**, durable | the handler **writes a row** — this is the arbiter |
| on-chain `proof_hash` | chain state (A.7) | one tx settles one PayLink | settlement — already anti-replayed on-chain |

**Decision rule:** writes a row → DB `UNIQUE` (`DbDedupe` or a domain unique index, e.g.
notification's `deliveries_dedupe_uidx`); expensive non-DB work → `RedisDedupe`; a client may retry →
`Idempotency-Key`; settlement → already anti-replayed on-chain, don't re-guard it.

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

`fingerprint` is a stable SHA-256 over the **canonical** (sorted-key, compact) request body. Callers
MUST send a stable body per `Idempotency-Key`: the same key with a different body is always a
`409 IDEMPOTENT_CONFLICT` (never a silent replay of the wrong response). A first request still in
flight is also a 409.

## A.7 alignment

This framework **never gates settlement** on its Redis/DB marker. The on-chain proof-hash check is the
source of truth for "already settled"; these helpers only make retries cheap and stop duplicate
**app-side** effects (a double HTTP response, a double notification, a double ledger leg) **before** a
request reaches the chain. If Redis is down or a key expires, the worst case is a duplicate attempt the
chain's anti-replay (and the `proof_hash` DB UNIQUE in payment/proof) still rejects — it **fails safe
toward on-chain truth**. It touches neither fund flow (A.1) nor ledger balancing (A.6).

## Test

```
pip install -e ".[dev]"
ruff check . && black --check . && mypy src
pytest                       # unit (fakeredis) + integration (testcontainers, skipped without Docker); 80% gate
```
