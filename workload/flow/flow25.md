# flow25 — fraud-detection-service (execution recipe · seeded skeleton)

**Work item:** [work25](../work/work25.md) · **Goal recap:** hybrid rules+ML scoring with a real block path.

## Pre-flight
- [ ] Read [work25](../work/work25.md), [rules.md](../rules.md). Confirm work02 + work12 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.16 (rules, features, decision, feedback) | **Explore** |
| 2 | Design rules + ML scoring + block integration with orchestrator | **Plan** |
| 3 | Scaffold Python/FastAPI skeleton (mirror work01) | `/scaffold-service` |
| 4 | Implement rules engine + XGBoost scoring + feedback | **service-builder** |
| 5 | Wire block path into work02; tests ≥80%; check p95<80ms | **service-builder** |
| 6 | Review PII + block-path enforcement | **invariant-auditor** + `/security-review` |
| 7 | Verify a block prevents rail initiation | `/verify` |

## Done
- [ ] [work25](../work/work25.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
