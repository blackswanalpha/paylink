# flow01 — Scaffold paylink-service (execution recipe)

**Work item:** [work01](../work/work01.md) · **Goal recap:** first TS microservice — versioned
PayLink CRUD over Postgres, reading on-chain status from the lVM RPC. Becomes the service template.

## Pre-flight
- [ ] Read [work01](../work/work01.md), [rules.md](../rules.md), [standard.md](../standard.md).
- [ ] No dependencies — clear to start.
- [ ] Set work01 → `in-progress` in [backlog.md](../backlog.md).

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map the lVM RPC PayLink query methods and the `types` PayLink shape to mirror | **Explore** (`paylink-chain/internal/rpc`, `internal/types`) | list of RPC methods + fields to reuse |
| 2 | Design the service: routes, DB schema, config, on-chain read seam, auth seam | **Plan** | short design doc |
| 3 | Scaffold the Python/FastAPI skeleton (Pydantic, ruff/black/mypy, env config, structlog, health/readiness/metrics, Dockerfile, docker-compose entry, pytest setup) | `/scaffold-service` (skill) | `linkmint-backend/paylink-service/` skeleton |
| 4 | Implement migrations + `paylinks` table + repository layer | **service-builder** | migration files + data layer |
| 5 | Implement `/v1/paylinks` create/get/list/cancel with the error envelope | **service-builder** | endpoints |
| 6 | Implement on-chain status read via lVM JSON-RPC client | **service-builder** | RPC client + status mapping |
| 7 | Unit tests (mocks) + integration tests (testcontainers Postgres); reach ≥80% | **service-builder** | passing tests + coverage |
| 8 | Review against invariants (A.1/A.4/A.7) and quality | **invariant-auditor** + `/code-review` | clean diff |
| 9 | Verify end-to-end on the local stack | `/verify` (+ `/run`) | observed create→get→list→cancel |

## Done
- [ ] All acceptance criteria in [work01](../work/work01.md) met.
- [ ] Backend-service checklist in [definition-of-done.md](../definition-of-done.md) complete.
- [ ] Note the chosen layout as the service template; set work01 → `done` in [backlog.md](../backlog.md).
