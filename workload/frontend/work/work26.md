# work26 — Analytics & Reporting UI

> **Seeded** — expand with `/work 26` when picked up (await backend [work26](../../work/work26.md)).

- **Status:** seeded · **Owner:** service-builder · **Depends on:** 03,08 · backend **26** (reporting-analytics) · **Flow:** [flow26](../flow/flow26.md)
- **Phase:** FE-2 · **Implements:** [frontendfeature.md §3.3](../../../frontendfeature.md) (Analytics — PLANNED)

## Goal
A merchant **Analytics** surface: revenue series, conversion, rail mix, exportable reports — the richer
analytics the dashboard overview (work18) marks PLANNED.

## In scope
- `/dashboard/analytics`: time-series charts (revenue, volume), conversion + rail-mix breakdowns, date-range, CSV/PDF export.
- A charting library decision (recorded in [../../decisions.md](../../decisions.md)); upgrades the overview's client-derived stats to real reporting data.

## Out of scope
- The reporting backend/ClickHouse (backend work26). Custom report builder (later).

## Invariants that apply
- **F.1 SDK-only**, **F.5**, **F.6** (charts have accessible table fallbacks), **F.7**.

## Acceptance criteria
- [ ] Revenue/volume/conversion/rail-mix render from the reporting API; date-range + export work.
- [ ] Charts have accessible fallbacks; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack" once backend work26 + its SDK resource exist.

## Notes / log
- Blocked on backend work26. Replaces the work18 overview's client-side aggregates with real reporting.
