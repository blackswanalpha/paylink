# flow23 — settlement-service (execution recipe · seeded skeleton)

**Work item:** [work23](../work/work23.md) · **Goal recap:** aggregate → settlement → T+1 payout + file ingest, ledger-backed.

## Pre-flight
- [ ] Read [work23](../work/work23.md), [rules.md](../rules.md) (A.1, A.6). Confirm work02 + work10 + work16 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.12 (settlement, payout scheduling, file ingest) | **Explore** |
| 2 | Design aggregation + payout scheduler + ledger postings | **Plan** |
| 3 | Scaffold Go/chi skeleton (mirror work02) | `/scaffold-service` |
| 4 | Implement settlements + payouts + rail-file ingest | **service-builder** |
| 5 | Post balanced ledger entries (work16); tests ≥80% | **service-builder** |
| 6 | Review A.1 (no custody) + A.6 (balanced) | **invariant-auditor** + `/code-review` |
| 7 | Verify settlement → payout → file match | `/verify` |

## Done
- [ ] [work23](../work/work23.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
