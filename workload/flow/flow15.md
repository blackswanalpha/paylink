# flow15 — event bus + domain event catalog (execution recipe · seeded skeleton)

**Work item:** [work15](../work/work15.md) · **Goal recap:** Kafka/SQS transport + event catalog + client libs + chain mirror.

## Pre-flight
- [ ] Read [work15](../work/work15.md), [rules.md](../rules.md), ADR-004 in [decisions.md](../decisions.md). No deps. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Extract the full event taxonomy from backendfeatures.md; map the lVM datastream events | **Explore** |
| 2 | Choose Kafka vs SQS for local; design envelope + client lib API | **Plan** |
| 3 | Stand up the transport in docker-compose | **service-builder** |
| 4 | Implement Python + Go publish/consume libs (correlation id, at-least-once) | **service-builder** |
| 5 | Implement chain-event-mirror (WS → `chain.*`) | **service-builder** |
| 6 | Tests for libs; review idempotency + no-secrets | **invariant-auditor** + `/code-review` |
| 7 | Verify cross-service publish/consume + chain mirror | `/verify` |

## Done
- [ ] [work15](../work/work15.md) criteria met; Infra/CI DoD complete; mark `done` in [backlog.md](../backlog.md).
