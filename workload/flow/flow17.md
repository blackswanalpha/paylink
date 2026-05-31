# flow17 — idempotency framework (execution recipe · seeded skeleton)

**Work item:** [work17](../work/work17.md) · **Goal recap:** shared Idempotency-Key middleware + consumer dedupe.

## Pre-flight
- [ ] Read [work17](../work/work17.md), [rules.md](../rules.md) (A.7). Confirm work15 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec idempotency section + proof_hash semantics (work03) | **Explore** |
| 2 | Design middleware (Redis key scheme, TTL, response caching) + consumer dedupe | **Plan** |
| 3 | Implement Python + Go middleware + helpers | **service-builder** |
| 4 | Adopt in one Python + one Go service | **service-builder** |
| 5 | Tests (replayed request, redelivered event); ≥80% | **service-builder** |
| 6 | Review A.7 alignment (defers to on-chain truth) | **invariant-auditor** + `/code-review` |
| 7 | Verify single-effect on replay | `/verify` |

## Done
- [ ] [work17](../work/work17.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
