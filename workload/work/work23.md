# work23 — settlement-service (aggregation, payouts, reconciliation files)

- **Status:** done · **Owner:** service-builder · **Stack:** Go/chi · **Depends on:** 02, 10, 16 · **Flow:** [flow23](../flow/flow23.md)
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
- [x] Verified PayLinks aggregate into a merchant settlement with gross/fee/net (balanced ledger).
- [x] T+1 payout scheduled + executed via rail; rail file ingested + matched.
- [x] Tests ≥80%; lint/build clean.
- [x] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Go/chi)" + "Full stack".

## Outcome (done 2026-06-13)
Built `linkmint-backend/settlement-service/` (Go/chi, port 8101, owns `settlement` schema), mirroring
the work20 escrow template. Aggregates `chain.paylink.verified` (enriched with payee+amount) +
`chain.fee.collected` into per-merchant settlements; balanced double-entry ledger postings (A.6) via
ledger-go on the caller tx; T+1 payout scheduler (cutoff per tz, min-payout per currency);
internal/mTLS rail-file ingest (JSON/CSV) that matches lines → payouts PAID; consumes `merchant.*`
(projections) + `refund.clawback.requested` (negative offset, enriched with paylink_id in work22);
publishes `settlement.*`/`payout.*` (payout.* aliased to the settlement topic). Non-custodial (A.1):
payouts are instructions only. Wired into docker-compose, Kong (X-Creator-Addr-injected routes), and
CI; `make cover` 81.3%. See the service `DESIGN.md` for the documented seams.
