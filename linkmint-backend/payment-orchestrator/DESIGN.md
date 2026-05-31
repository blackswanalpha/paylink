# payment-orchestrator — design (flow02 step 2)

## Goal
Coordinate a PayLink's payment lifecycle: from "awaiting payment" through chain settlement to a
terminal state, driving transitions and reacting to chain events — **without** holding funds (A.1),
verifying proofs (work03), or handling rail callbacks (work04).

## Why not import `paylink-chain/internal/*`?
work02 only needs **authoritative on-chain status** and **live events**, both available as JSON over
the node's JSON-RPC and the `/ws` datastream. Byte-exact tx wire format / signing (the reason
`standard.md` tells proof-validator + adapters to import the chain packages) is **not** needed here.
Speaking JSON keeps the orchestrator decoupled and sidesteps Go's `internal/` import barrier.

## Lifecycle state machine — a projection, not a new authority
The chain's `PayLink` FSM (`paylink-chain/internal/fsm/paylink_fsm.go`) has
`NONE → CREATED → {VERIFIED | CANCELLED | FAILED}`. The orchestrator mirrors exactly the payable
slice:

```
AWAITING_PAYMENT ── settle (chain VERIFIED)  ─→ SETTLED   (terminal)
                 ── cancel (chain CANCELLED) ─→ CANCELLED (terminal)
                 ── fail   (chain FAILED)    ─→ FAILED    (terminal)
```

No state exists that the chain does not have. `lifecycle.Project(current, chainStatus)` is the single
transition function; it is **idempotent** (target == current ⇒ no-op) and **monotonic** (terminal
states reject all moves), which is the lifecycle-layer half of A.7.

## Idempotency strategy (A.7 — no double-advance)
Three independent guards, any one of which suffices:
1. **FSM terminal guard** — a second `VERIFIED` after `SETTLED` is a no-op; a `CANCELLED` after
   `SETTLED` is rejected (illegal, swallowed with a warning).
2. **Event dedupe** — `applied_chain_events (paylink_id, seq)` PK; a redelivered event inserts zero
   rows and is skipped. Append-only audit trail.
3. **`Idempotency-Key`** on `POST /v1/payments` (Redis, 24h TTL) — replays the cached response.

Apply is one transaction: `SELECT … FOR UPDATE` the payment row → dedupe insert → project → update.
Concurrent events / a reconcile racing an event cannot double-apply.

## Two paths to advance, one atomic core
- **Subscriber (fast path):** WS `/ws`, filter `{entityTypes:[paylink], eventKinds:[verified,
  cancelled,failed]}`. The chain datastream is at-most-once (no replay-from-seq on resubscribe), so a
  reconnect can miss events.
- **Read reconcile (safety net):** `GET /v1/payments/{id}` reads `paylink_getPayLink` and advances if
  the chain is ahead — guaranteeing the response is consistent with on-chain state and closing the
  reconnect gap. Degrades gracefully: a transient chain error returns the stored record, never 5xx.

## Boundaries
- **Record vs flow:** initiate validates the PayLink via **paylink-service** (`GET /v1/paylinks/{id}`,
  must be `CREATED`); settlement truth comes from the **chain**, never from paylink-service.
- **One payment per PayLink:** `payments.paylink_id` is `UNIQUE` (A.7 — one PayLink settles once).
- **Persistence:** one schema (`payment`); numbered embedded migrations; no cross-schema FKs
  (`paylink_id` is an opaque ref).
- **Events out:** `payment.initiated|settled|cancelled|failed` via a `Publisher` seam (transport =
  Kafka/SQS, work15).

## Out of scope (deferred)
Proof verification + broadcast (work03), rail callbacks (work04), escrow/refund (work20/22),
double-entry settlement ledger (work16/23), auth gateway (work05), real event transport (work15).
