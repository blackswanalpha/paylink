# flow35 — fix: orchestrator rejects payable (PENDING) PayLinks (recipe · seeded)

**Work item:** [work35](../work/work35.md) · **Goal recap:** orchestrator `Initiate` must accept a
live, unsettled PayLink (`CREATED`/`PENDING`), not only `CREATED`.

## Pre-flight
- [x] Read [work35](../work/work35.md), work01/02 notes. Reproduce the 409 (create via SDK/gateway → initiate). Set `in-progress`.

## Steps
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Confirm the payable-state set vs `OffChainStatus` + chain PayLink FSM | **Explore** |
| 2 | Widen the `Initiate` guard to accept `CREATED`+`PENDING`; keep terminal rejects | **chain-engineer** / **service-builder** |
| 3 | Integration test: create (submit enabled) → initiate succeeds | **service-builder** |
| 4 | Review vs A.7 + FSM non-divergence | **invariant-auditor** + `/code-review` |
| 5 | Verify live: `docker compose --profile e2e` create→initiate→(settle) | `/verify` |

## Done
- [x] [work35](../work/work35.md) criteria met; mark `done` in [backlog.md](../backlog.md).
