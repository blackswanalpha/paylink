# work20 — escrow-manager (conditional release/refund)

> **Seeded** — expand with `/work 20` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Go/chi · **Depends on:** 01, 03 · **Flow:** [flow20](../flow/flow20.md)
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
- [ ] Create escrow → condition met (confirm / time-lock / N-of-M) → RELEASED; timeout → REFUNDED.
- [ ] Dispute path → DISPUTED (awaits resolution); no custody anywhere.
- [ ] Tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Go/chi)" + "Full stack".
