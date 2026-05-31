# work27 — reconciliation-service (3-way: DB ↔ chain ↔ rails)

> **Seeded** — expand with `/work 27` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Go/chi · **Depends on:** 23 · **Flow:** [flow27](../flow/flow27.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.20

## Goal
Daily 3-way reconciliation across DB, chain, and rail settlement files — detect, classify, and
route discrepancies so money is provably accounted for.

## In scope
- `/v1/reconcile/run`, `/runs`, `/discrepancies`, `/discrepancies/{id}/resolve`.
- Per rail per day: join payments ↔ proofs ↔ chain events ↔ rail file on (pl_id, rail_tx_id, amount).
- Classify: missing_rail/chain/db, amount_mismatch, duplicate_proof; owns `reconcile` schema; publishes `reconcile.*`.
- Critical discrepancies page oncall; daily run completes < 30 min; ≥99.5% match target.

## Out of scope
- Streaming (within-hour) recon + ML root-cause (Phase 3).

## Invariants that apply
- Non-custodial; the on-chain proof hash is authoritative for chain side (A.7); read-only over sources.

## Reuse first
- Go/chi conventions (mirror work02); settlement file ingest (work23); the ledger (work16);
  chain events via the bus (work15).

## Acceptance criteria
- [ ] A daily run joins the three sources and classifies discrepancies correctly.
- [ ] Discrepancies queryable + resolvable; critical ones alert; match-rate measured.
- [ ] Tests ≥80% (incl. each discrepancy class); lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Go/chi)" + "Full stack": run recon over a
seeded day with an injected mismatch; confirm it's detected + classified.
