# escrow-manager — design notes (work20)

## Non-custodial by construction (invariant A.1)

The service coordinates **state**, never money. Concretely:

- The `escrow.escrows` table has **no balance columns**; `amount`/`currency` mirror the PayLink
  only so release/refund events carry a complete instruction.
- There is **no wallet and no chain-write client** anywhere in the module — the only chain
  coupling is *inbound*: the `chain.paylink.verified` bus event (published by chain-event-mirror
  from the lVM datastream) marks an escrow `funded` (A.3 — settlement truth from chain).
- `escrow.released` and `escrow.refunded` are **instructions** for the settlement layer
  (work23+): `{escrow_id, pl_id, payee_addr|refund_to, amount, currency, funded, tx_hash}`.
  Nothing moves when they are published.

## The `funded` flag (not a state)

The FSM is condition-shaped (WAITING → CONDITIONS_MET → RELEASED | REFUNDED | DISPUTED);
funding is orthogonal — it can arrive before or after the condition is satisfied. Modeling it
as a column flag (set once by the consumer, with `funded_tx_hash`) keeps both orderings
symmetric: whichever of {funding, satisfaction} completes the pair triggers
ConditionsMet+Release **in one transaction** (CONDITIONS_MET never persists). A timeout refund
of an unfunded escrow simply carries `funded:false` — there is nothing to move.

## Atomicity & idempotency (A.7)

- One escrow per PayLink: `pl_id UNIQUE` → 409 `ESCROW_EXISTS`.
- `POST /v1/escrows` honors `Idempotency-Key` (Redis Begin/Complete, caller-salted fingerprint).
- Approvals: `PRIMARY KEY (escrow_id, approver_addr)` — N-of-M re-approval is a no-op.
- Consumer: work17 **DbDedupe** on `escrow.processed_events` keyed `pl_id:tx_hash`, inserted on
  the SAME pgx transaction as the funded-write/release — at-least-once redelivery, exactly-once
  effect. A handler error rolls everything back and leaves the offset uncommitted.
- Sweeper: CAS updates (`UPDATE … WHERE state='WAITING'`) — concurrent confirm/funding/sweep
  cannot double-advance, and DISPUTED rows are never touched.

## Known MVP gaps (filed as follow-ups)

- The mirror payload carries no amount, so funding trusts the `pl_id` linkage; an
  amount/payee cross-check against the chain RPC is a tracked follow-up.
- An escrow created *after* its PayLink settled is never auto-funded (the funding event was
  consumed and ignored before the escrow existed). Acceptable for the MVP flow (escrow first,
  payment second).
- Dispute *resolution* (admin release/refund decision) is work22; this service only parks the
  escrow in DISPUTED and emits `escrow.disputed`.
