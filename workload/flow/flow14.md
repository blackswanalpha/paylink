# flow14 — notification-service (execution recipe · seeded skeleton)

**Work item:** [work14](../work/work14.md) · **Goal recap:** SMS/email delivery with retries + templates.

## Pre-flight
- [ ] Read [work14](../work/work14.md), [rules.md](../rules.md). Confirm work15 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.18 (channels, retry, templates, events) | **Explore** |
| 2 | Design event→template→channel pipeline + retry/backoff | **Plan** |
| 3 | Scaffold Python/FastAPI skeleton (mirror work01) + Celery/Redis | `/scaffold-service` |
| 4 | Implement consumers + SMS/email senders + template registry | **service-builder** |
| 5 | Tests (delivery, retry/backoff, template render); ≥80% | **service-builder** |
| 6 | Review secrets handling + PII minimization | **invariant-auditor** + `/code-review` |
| 7 | Verify event→delivery on the stack (sandbox providers) | `/verify` |

## Done
- [x] [work14](../work/work14.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
