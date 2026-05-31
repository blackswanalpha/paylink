# work11 — admin-backoffice (read-only console; mutations in Phase 2)

> **Seeded** — expand with `/work 11` when picked up.

- **Status:** done · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 09 · **Flow:** [flow11](../flow/flow11.md)
- **Phase:** 1 / MVP (read-only) · **Spec:** backendfeatures.md §2.18 (the header's "§2.4" was a typo — admin-backoffice is §2.18)

## Goal
Internal ops console for support/finance/compliance/engineering. **Phase 1: read-only**
(unified search + entity views). Mutating actions (suspend, force-refund, resolve, feature flags)
land in Phase 2 with dual-approval. All privileged actions auditable.

## In scope (Phase 1)
- `/v1/admin/search`, `/v1/admin/{users,merchants,paylinks,payments}/{id}` read views.
- Admin JWT role + MFA required; default-deny authorization scopes.
- Every access/action emits to the audit-log (work13).

## Out of scope (Phase 2+)
- Mutations: suspend/restore, force-refund, dispute/flag resolve, feature flags, announcements.
- Dual-approval gating (Phase 2).

## Invariants that apply
- Non-custodial; least-privilege RBAC; all privileged actions auditable ([rules.md](../rules.md) B).

## Reuse first
- work09 auth/RBAC + MFA; read APIs of paylink/payment/merchant services; audit-log (work13).

## Acceptance criteria
- [x] Unified search + entity read views behind admin JWT + MFA.
- [x] Default-deny scopes; every view logged to audit-log (LogAuditSink → work13 drop-in).
- [x] Tests ≥80%; lint/build clean. (95.5% / 50 tests; ruff/black/mypy clean.)
- [x] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
