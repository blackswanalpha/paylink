# work20 — escrow-manager (conditional release/refund)

> **Done** (2026-06-12) — `linkmint-backend/escrow-manager/` (port 8098, `escrow` schema), cover 94.0%
> (every pkg ≥80, incl. postgres integration), invariants PASS (8/8), live compose smoke green.

- **Status:** done · **Owner:** service-builder · **Stack:** Go/chi · **Depends on:** 01, 03 · **Flow:** [flow20](../flow/flow20.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.7

## Goal
Conditional PayLinks: hold release until a condition is met (delivery confirmation, time-lock, or
N-of-M approval), then release or refund, with the WAITING→CONDITIONS_MET→RELEASED|REFUNDED|DISPUTED machine.

## In scope
- `/v1/escrows` (create with conditions + timeout + refund_to), `/confirm`, `/dispute`, `GET`.
- Condition types: delivery_confirmation, time_lock (auto at release_at), multi_party_approval.
- Owns `escrow` schema; consumes `chain.paylink.verified` (evaluate conditions); publishes `escrow.*`.

## Out of scope
- Manual dispute resolution UI (admin/Phase 2 admin mutations).
- Holding funds — **escrow coordinates state/conditions, never custody (A.1).**

## Invariants that apply
- **Non-custodial (A.1)** — the protocol never holds the escrowed funds; release/refund is a
  state/instruction outcome, settled via the rail/chain, not a LinkMint-held balance.
- Settlement truth from chain; anti-replay.

## Reuse first
- Go/chi conventions (mirror work02); PayLink FSM in `paylink-chain/internal/fsm`; event bus (work15).

## Acceptance criteria
- [x] Create escrow → condition met (confirm / time-lock / N-of-M) → RELEASED; timeout → REFUNDED.
- [x] Dispute path → DISPUTED (awaits resolution); no custody anywhere.
- [x] Tests ≥80%; lint/build clean.
- [x] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Go/chi)" + "Full stack".

## Notes / log
- **Done 2026-06-12.** Mirrors the payment-orchestrator layout (ports/adapters + `internal/httpx`)
  with proof-validator's per-package cover Makefile. FSM copied from `paylink-chain/internal/fsm`
  (WAITING→CONDITIONS_MET→RELEASED|REFUNDED|DISPUTED; **funded is a column flag set only by the
  `chain.paylink.verified` consumer** — no API path can mark an escrow funded; ConditionsMet+Release
  apply together in one DB tx). The repo's **first Go bus consumer** (eventbus-go, group
  `escrow-manager`, topic `chain`) and the **first wired `DbDedupe`** (work17): dedupe row on
  `escrow.processed_events` commits atomically with the funded-write. Sweeper ticker handles
  time-lock release + timeout refund via CAS (`WHERE state='WAITING'`); DISPUTED blocks both.
  `escrow.created/released/refunded/disputed` published after commit as **instructions, never
  transfers (A.1)** — no balance columns, wallet, signer, or chain client in the module.
- Integrated: compose (8098, kafka mode), Kong `escrows` route (jwt+key-auth verbatim from payments;
  post-function gate + entrypoint/Makefile/test-compose wiring), prometheus target, CI job. Gateway
  suite 33 passed (3 new escrow auth-matrix tests); `kong config parse` clean.
- Live smoke: create 201 + idempotent replay; unfunded confirm stays WAITING; real
  `chain.paylink.verified` (rpk) → funded + RELEASED with tx_hash; short-timeout → sweeper REFUNDED
  `funded:false`; all three escrow metrics live.
- Invariant audit 8/8 PASS. Hardened post-audit: `GET /v1/escrows/{id}` is view-scoped
  (participants + multi-party approvers; outsiders get the missing-id 404 — no existence leak).
- Known MVP gaps (DESIGN.md + backlog): publish-after-commit is at-most-once (a crash in the
  commit→publish window drops the released/refunded instruction — covered by the open work15 Go
  transactional-outbox follow-up); mirror payload carries no amount → funding trusts the `pl_id`
  linkage (cross-check vs chain RPC is a follow-up); escrow created after its PayLink already
  settled never auto-funds; dispute resolution (the CONDITIONS_MET→Refund seam) is work22.
