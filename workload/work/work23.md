# work23 — settlement-service (aggregation, payouts, reconciliation files)

> **Seeded** — expand with `/work 23` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Go/chi · **Depends on:** 02, 10, 16 · **Flow:** [flow23](../flow/flow23.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.12

## Goal
Off-chain settlement lifecycle: aggregate verified PayLinks into merchant settlements, schedule
T+1 payouts, ingest rail settlement files, and produce statements.

## In scope
- `/v1/settlements`, `/v1/payouts` (+create), `/settlements/files/ingest` (mTLS).
- T+1 daily payout scheduling (cutoff per merchant tz); min-payout per currency.
- Owns `settlement` schema; consumes `chain.paylink.verified`, `chain.fee.collected`, `merchant.bank_account.verified`; publishes `settlement.*`, `payout.*`.
- Posts ledger entries (work16) for gross/fee/net.

## Out of scope
- Instant payouts, multi-currency, payout splitting (Phase 3).
- The 3-way reconciliation algorithm itself (work27 consumes ingested files).

## Invariants that apply
- **Non-custodial (A.1)** — payouts instruct the merchant's bank rail; LinkMint never holds the balance.
- Double-entry ledger (A.6) for every settlement line; settlement truth from chain.

## Reuse first
- Go/chi conventions (mirror work02); ledger helper (work16); fee semantics in `internal/fee`; event bus (work15).

## Acceptance criteria
- [ ] Verified PayLinks aggregate into a merchant settlement with gross/fee/net (balanced ledger).
- [ ] T+1 payout scheduled + executed via rail; rail file ingested + matched.
- [ ] Tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Go/chi)" + "Full stack".
