# work30 — Realtime (WebSocket datastream)

> **Seeded** — expand with `/work 30` when picked up (await the chain datastream client path).

- **Status:** seeded · **Owner:** service-builder · **Depends on:** 11,12,13 · chain `datastream` (WS) · **Flow:** [flow30](../flow/flow30.md)
- **Phase:** FE-2 · **Implements:** [frontendfeature.md §5](../../../frontendfeature.md) (Realtime) / `fe12`

## Goal
Replace settlement **polling** with a WebSocket subscription to the chain `datastream` so PayLink/payment
status updates push instantly — with polling as the graceful fallback.

## In scope
- A WS client subscribing to `paylink.verified/failed/cancelled`; wire it into the settlement views
  (work13), the payments timeline (work12), and the PayLinks list (work11) so statuses update live.
- Reconnect/backoff; fall back to the existing poll (`useSettlementStatus`) when WS is unavailable. No UI contract change (same status → same `StatusPill`).

## Out of scope
- Chain-side datastream changes. A general pub/sub framework.

## Invariants that apply
- **F.1 SDK-only** (subscription via an SDK/client seam, not ad-hoc), **F.5**, **F.7** (fallback honest).

## Acceptance criteria
- [ ] Statuses update live via WS where wired; polling remains the fallback on disconnect.
- [ ] Reconnect/backoff handled; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": settle a PayLink and observe an
instant push update (no poll delay); kill the WS → confirm poll fallback still settles.

## Notes / log
- Upgrades work11/12/13 freshness. Needs an SDK/client datastream seam; coordinate with the chain WS endpoint.
