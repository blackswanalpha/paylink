# flow08 — docker-compose + CI (execution recipe · seeded skeleton)

**Work item:** [work08](../work/work08.md) · **Goal recap:** one-command local stack + green CI.
**Incremental** — extend per service as 01–07 land.

## Pre-flight
- [ ] Read [work08](../work/work08.md), [rules.md](../rules.md) (secrets). Set `in-progress` when actively extending.

## Steps (skeleton — refine on start)

| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Inventory each component's Dockerfile + run/test commands | **Explore** |
| 2 | Design the compose topology (node, Postgres, Redis, services) + CI matrix | **Plan** |
| 3 | Author/extend `docker-compose.yml` with healthchecks | **service-builder** |
| 4 | Author `.github/workflows/` jobs: chain (`go build/test`), TS lint/build/test | **service-builder** |
| 5 | Ensure no secrets in YAML; use env injection / repo secrets | **invariant-auditor** + `/security-review` |
| 6 | Verify: `docker-compose up` healthy + full flow; CI green on a test PR | `/verify` |

## Done
- [ ] [work08](../work/work08.md) criteria met; Infra/CI DoD complete; mark `done` in [backlog.md](../backlog.md).
