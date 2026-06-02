# work16 — Admin Console (search / drill-down / audit)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 08, 09 · backend [work11](../../work/work11.md), [work13](../../work/work13.md)
- **Flow:** [flow16](../flow/flow16.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.4 (Admin Console) / `fe07`

## Goal
The internal ops console: an MFA-gated unified search across users/merchants/PayLinks/payments, entity
drill-down panels, and an audit-log viewer with on-chain chain-integrity verification.

## Why / context
Ops/support staff need a read surface over the platform. admin-backoffice (backend work11, read-only)
+ audit-log (work13) provide it; this is their premium, access-controlled UI, with graceful
degradation when an upstream is down.

## In scope
- `/admin` (MFA + admin-role gated — refuse non-MFA tokens, coordinate with work09): a unified search
  bar → typed results across users/merchants/PayLinks/payments; a **degraded** banner when an upstream is down.
- Entity **drill-down** panels (user / merchant / paylink / payment) via the admin endpoints.
- **Audit-log viewer**: filterable entries (actor/resource/time) + a "Verify chain integrity" action
  showing `{ok, broken_at?}` from work13.
- Mutations (suspend/force-refund/override) shown as **PLANNED** (backend work11 Phase-2), per F.7.

## Out of scope (do NOT do here)
- The mutations themselves (PLANNED backend). New admin APIs. Merchant-facing onboarding → work14.

## Invariants that apply
- **F.4 edge-auth** + admin/MFA gating (console refuses non-MFA/non-admin tokens), **F.1 SDK-only**,
  **F.5** (degraded/partial results honest), **F.6**, **F.7** (mutations marked PLANNED).

## Reuse first
- `client.admin.*` + `client.auditLog.*` (work08); `DataTable`/`Tabs` (work03); `StatusPill`/
  `AddressChip` (built); the degraded-result pattern from backend work11; errors (work04).

## Acceptance criteria
- [ ] `/admin` is gated on admin role + MFA; non-MFA tokens are refused.
- [ ] Unified search returns results across entity types and shows a degraded banner if an upstream is down.
- [ ] Entity drill-downs render; the audit viewer filters + verifies chain integrity (ok/broken).
- [ ] Mutations marked PLANNED; `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": with an MFA admin token, search +
drill-down + audit verify against the live stack; with a non-MFA token, confirm the console refuses.

## Notes / log
- Reuses the audit chain-verify from backend work13. Mutations await backend work11 Phase 2.
