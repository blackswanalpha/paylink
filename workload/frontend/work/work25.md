# work25 — Settlements UI

> **Seeded** — expand with `/work 25` when picked up (await backend [work23](../../work/work23.md)).

- **Status:** seeded · **Owner:** service-builder · **Depends on:** 03,08 · backend **23** (settlement-service) · **Flow:** [flow25](../flow/flow25.md)
- **Phase:** FE-2 · **Implements:** [frontendfeature.md §3.3](../../../frontendfeature.md) (Settlements — PLANNED)

## Goal
A merchant **Settlements** surface: payout batches, per-batch detail, and reconciliation status — over
the settlement-service API.

## In scope
- `/dashboard/settlements`: batch list (period, amount, status), batch detail (included payments), payout status.
- Reuse `DataTable`/`StatusPill`/`AmountDisplay`; SDK resource added under work08 when the backend lands.

## Out of scope
- New backend APIs. Reconciliation internals (backend work27).

## Invariants that apply
- **F.1 SDK-only**, **F.2 non-custodial**, **F.5**, **F.6**, **F.7** (until backend 23 ships, the nav entry is marked PLANNED).

## Acceptance criteria
- [ ] Settlement batches + detail render via the SDK; reconciliation status shown.
- [ ] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack" once backend work23 + its SDK resource exist.

## Notes / log
- Blocked on backend work23 (settlement-service). Expand this stub with `/work 25` then.
