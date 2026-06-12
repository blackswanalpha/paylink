# work16 — double-entry ledger (shared `ledger` schema)

> **Seeded** — expand with `/work 16` when picked up.

- **Status:** done · **Owner:** service-builder · **Stack:** shared Postgres schema + Python/Go libs · **Depends on:** 15 · **Flow:** [flow16](../flow/flow16.md)
- **Phase:** 1 / MVP (cross-cutting) · **Spec:** backendfeatures.md §"Data Consistency & Ledger"

## Goal
The append-only double-entry ledger: the `ledger.ledger_entries` table plus posting helpers
shared (read/write) across services, giving a full audit trail for reconciliation.

## In scope
- `ledger.ledger_entries` (entry_group UUID, account, direction DR|CR, amount, currency, pl_id, note, created_at).
- Posting helper (Python + Go) that writes balanced DR/CR pairs in one transaction.
- Append-only enforcement: corrections are new reversing entries, never edits/deletes.
- Read APIs/queries for reconciliation (work27) and reporting (work26).

## Out of scope
- Per-service business logic that posts entries (each service calls the helper).
- Schema extraction to its own DB (Phase 3).

## Invariants that apply
- **Double-entry (rules A.6): append-only; DR total == CR total per entry_group.** Non-custodial.

## Reuse first
- The fee split semantics in `paylink-chain/internal/fee` for fee-related postings.
- The event bus (work15) to react to `chain.fee.*`, `payment.confirmed`, etc.

## Acceptance criteria
- [x] Posting helper writes balanced DR/CR atomically; unbalanced posts rejected.
- [x] Append-only enforced (no UPDATE/DELETE path); corrections via reversing entries.
- [x] Read queries support reconciliation/reporting; tests ≥80%.
- [x] Passes the relevant [definition-of-done.md](../definition-of-done.md) checklist(s).

## Verification
[verification.md](../verification.md) → "Backend service": post a balanced entry group, attempt an
unbalanced one (rejected), attempt an edit (rejected); confirm reversing-entry correction.

## Notes / log
- Shipped as the byte-identical sibling libs `linkmint-backend/ledger-go` + `ledger-python` over the
  shared append-only `ledger` schema: DB trigger raises P0001 on UPDATE/DELETE, `Post()` validates
  DR==CR before a single INSERT, `Reverse()` posts correcting entries, read APIs (Balance,
  IsBalanced, EntriesBy*) serve work26/27. One-shot `ledger-migrate`; no service posts yet (per-service
  posting is out of scope here; settlement-service is the intended first writer).
- 2026-06-12 — audit: header was stale (`todo` despite the libs + CI having shipped); flipped to done.
  Suites fresh-green: ledger-go 84.9%, ledger-python 100%; boxes ticked.
