# work26 — reporting-analytics (reports, exports, regulatory filings)

> **Seeded** — expand with `/work 26` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python/FastAPI (+ ClickHouse/DuckDB) · **Depends on:** 15 · **Flow:** [flow26](../flow/flow26.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.19

## Goal
Aggregate operational events into reportable form: merchant transaction/revenue reports, exports
(CSV/JSON/PDF), and regulatory filing drafts (SAR/CTR).

## In scope
- `/v1/reports/{transactions,revenue}`, `/v1/exports` (+status), `/v1/regulatory/{sar/draft,ctr}`.
- Events → ClickHouse (OLAP); materialized views; exports staged in S3 with 24h pre-signed URLs.
- Owns `reports` schema; broad read-only consumer of `chain.*`, `payment.*`, `paylink.*`, `refund.*`, `settlement.*`, `fee.*`.

## Out of scope
- Self-serve dashboards + data-warehouse query API + per-jurisdiction SAR templates (Phase 3).

## Invariants that apply
- Non-custodial; read-only (operational Postgres remains system of record); PII access controls + audit.

## Reuse first
- The event bus (work15) as the ingestion source; the ledger (work16) for revenue/fee figures; S3 from infra.

## Acceptance criteria
- [ ] Transactions + revenue reports generated; export (≤100k rows) completes within 60s to S3.
- [ ] CTR draftable; reports are read-only over OLAP, not the operational DB.
- [ ] Tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
