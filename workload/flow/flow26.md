# flow26 — reporting-analytics (execution recipe · seeded skeleton)

**Work item:** [work26](../work/work26.md) · **Goal recap:** OLAP reports + exports + regulatory drafts.

## Pre-flight
- [ ] Read [work26](../work/work26.md), [rules.md](../rules.md). Confirm work15 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.19 (reports, exports, regulatory, storage) | **Explore** |
| 2 | Design event→ClickHouse pipeline + materialized views + export staging | **Plan** |
| 3 | Scaffold Python/FastAPI skeleton (mirror work01) + ClickHouse in compose | `/scaffold-service` |
| 4 | Implement ingestion + reports + exports (S3) + CTR draft | **service-builder** |
| 5 | Tests (report correctness, export size/time); ≥80% | **service-builder** |
| 6 | Review read-only + PII access controls | **invariant-auditor** + `/security-review` |
| 7 | Verify a report + export end-to-end | `/verify` |

## Done
- [ ] [work26](../work/work26.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
