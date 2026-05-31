# work13 — audit-log-service (tamper-evident hash chain)

> **Seeded** — expand with `/work 13` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Go/chi · **Depends on:** 15, 16 · **Flow:** [flow13](../flow/flow13.md)
- **Phase:** 1 / MVP (linear chain) · **Spec:** backendfeatures.md §2.17

## Goal
Append-only, tamper-evident log of every privileged action — the system of record for
"who did what when". Phase 1: linear hash chain. On-chain anchoring is Phase 2.

## In scope (Phase 1)
- `POST /v1/audit-log` (internal mTLS), `GET /v1/audit-log`, `GET /{entry_id}`, `GET /verify`.
- Hash chain: `entry_hash = SHA256(prev_hash || canonical_json(entry))`; `/verify` detects breaks.
- Consumes `audit.intake` (every service emits privileged actions); owns `audit` schema.

## Out of scope (Phase 2+)
- Nightly on-chain anchoring (TxAuditAnchor) — Phase 2.
- Cold-archive to S3 (>90d) — Phase 3.

## Invariants that apply
- Append-only (no edits/deletes); non-custodial; mTLS for intake; deterministic canonical JSON.

## Reuse first
- The Go/chi conventions (mirror work02); SHA256 hashing conventions; event bus (work15) for `audit.intake`.

## Acceptance criteria
- [ ] Entries appended with linked hash chain; `/verify` returns ok or the break point.
- [ ] Query by actor/resource/time; intake via mTLS only.
- [ ] Tests (chain integrity, tamper detection); ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Go/chi)": append entries, tamper one in a
test fixture, confirm `/verify` reports the break.
