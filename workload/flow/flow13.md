# flow13 — audit-log-service (execution recipe · seeded skeleton)

**Work item:** [work13](../work/work13.md) · **Goal recap:** append-only tamper-evident hash chain.

## Pre-flight
- [ ] Read [work13](../work/work13.md), [rules.md](../rules.md). Confirm work15/16 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.17 (hash chain, endpoints, intake) | **Explore** |
| 2 | Design hash-chain + canonical JSON + verify | **Plan** |
| 3 | Scaffold Go/chi skeleton (mirror work02) | `/scaffold-service` |
| 4 | Implement append + chain linking + query + verify | **service-builder** |
| 5 | Wire `audit.intake` consumer (mTLS) | **service-builder** |
| 6 | Tests (integrity + tamper detection); review append-only | **invariant-auditor** + `/code-review` |
| 7 | Verify chain + tamper detection | `/verify` |

## Done
- [x] [work13](../work/work13.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
