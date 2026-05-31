# flow18 — observability (execution recipe · seeded skeleton)

**Work item:** [work18](../work/work18.md) · **Goal recap:** OTel tracing + Prometheus + structured logs across services.

## Pre-flight
- [ ] Read [work18](../work/work18.md), [rules.md](../rules.md). No deps. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec observability section + the lVM metrics package | **Explore** |
| 2 | Design shared tracing/logging init + correlation-id propagation | **Plan** |
| 3 | Implement Python (structlog+OTel) + Go (slog+OTel) init libs | **service-builder** |
| 4 | Add Prometheus + Tempo/Jaeger to docker-compose; scrape node + services | **service-builder** |
| 5 | Wire metrics + trace propagation through HTTP + event bus | **service-builder** |
| 6 | Review no-secrets/PII in telemetry | **invariant-auditor** + `/code-review` |
| 7 | Verify one trace end-to-end + metrics scraped | `/verify` |

## Done
- [ ] [work18](../work/work18.md) criteria met; Infra/CI DoD complete; mark `done` in [backlog.md](../backlog.md).
