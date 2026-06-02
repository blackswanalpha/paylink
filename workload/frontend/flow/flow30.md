# flow30 — Realtime (WebSocket datastream) (execution recipe)

> **Seeded** — skeleton recipe; expand when the chain datastream client seam is ready.

**Work item:** [work30](../work/work30.md) · **Goal recap:** push PayLink/payment status via WS, polling as fallback.

## Pre-flight
- [ ] Read [work30](../work/work30.md), [frontendfeature.md §5](../../../frontendfeature.md), the chain `datastream` WS + `useSettlementStatus` poll.
- [ ] Confirm work11/12/13 exist (the surfaces to upgrade).
- [ ] Set work30 → `in-progress`.

## Steps
| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map the chain WS events + the existing poll pattern | **Explore** | event + fallback map |
| 2 | Design the WS client seam + reconnect/backoff + fallback | **Plan** | short design |
| 3 | Implement the subscription + wire into work11/12/13 | **service-builder** | live updates |
| 4 | Reconnect/fallback + tests | **service-builder** | passing |
| 5 | Review F.1/F.7 | `/code-review` | clean diff |
| 6 | Settle a PayLink (instant push) + WS-down fallback | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work30](../work/work30.md) met; **App** checklist complete; status `done`.
