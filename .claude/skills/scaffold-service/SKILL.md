---
name: scaffold-service
description: Scaffold a new backend microservice under linkmint-backend/ in the correct stack — Python/FastAPI or Go/chi per ADR-003 / workload/standard.md. Use when starting a new service (e.g. identity, paylink-service, payment-orchestrator, settlement-service). Produces the directory layout, config, structured logging, /v1 routing with the error envelope, health/readiness/metrics, idempotency hook, migrations, Dockerfile, docker-compose entry, and test setup.
---

# Scaffold a LinkMint backend microservice

Creates a new service under `linkmint-backend/<name>/` conforming to `workload/standard.md`. **First
pick the stack** from the table in `workload/standard.md` (ADR-003): most services are
**Python/FastAPI**; the hot-path ones (payment-orchestrator, proof-validator, escrow,
settlement, wallet, audit-log, reconciliation) are **Go/chi**. Match the first service of that
stack as the reference template once it exists.

## Inputs to confirm first
- Service name (`linkmint-backend/<name>`), its stack, the `/v1` resource(s) it owns, its Postgres
  schema, which domain events it publishes/consumes, and whether it needs Redis/Kafka/SQS.
- Which work item it belongs to (for scope + acceptance criteria + phase).

## Standards baked in (both stacks)
- 12-factor env-only config; structured JSON logs + correlation id; graceful shutdown.
- `/internal/healthz`, `/internal/readyz`, `/metrics`.
- `/v1` versioned routes; standard error envelope `{"error":{"code","message","details","trace_id"}}`.
- `Idempotency-Key` hook on state-mutating routes (Redis, 24h TTL).
- One Postgres schema; numbered migrations; no cross-schema FKs (opaque id refs).
- Domain events by logical name over Kafka/SQS (ADR-004).
- An **auth seam** (middleware hook) for the api-gateway to fill.

## Python/FastAPI layout
```
linkmint-backend/<name>/
  app/
    main.py            # FastAPI app, lifespan, health/readiness/metrics
    config.py          # Pydantic BaseSettings (env)
    logging.py         # structlog JSON + correlation id
    api/v1/            # routers (one per resource)
    errors.py          # error-envelope exception handlers
    domain/            # business logic (no rail-specific code)
    db/                # SQLAlchemy models + session
    migrations/        # Alembic numbered migrations
    events/            # publish/consume domain events
  tests/               # pytest unit + integration (testcontainers)
  pyproject.toml       # ruff + black + mypy config
  Dockerfile
  .env.example
  README.md
```

## Go/chi layout
```
linkmint-backend/<name>/
  cmd/<name>/main.go   # bootstrap, chi router, health/readiness/metrics, shutdown
  internal/
    config/            # env config
    httpx/             # router, error envelope, middleware (idempotency, correlation id)
    domain/            # business logic
    store/             # Postgres + numbered SQL migrations
    events/            # Kafka/SQS publish/consume
  test/                # table-driven unit + integration
  go.mod
  Dockerfile
  .env.example
  README.md
```

## Wiring & invariants
- Add a service block (with healthcheck + deps) to root `docker-compose.yml`.
- **Non-custodial** (no funds), **rail-agnostic** (no rail fields in models/APIs), **settlement
  truth from chain** (read lVM JSON-RPC). Go services on the chain boundary import
  `paylink-chain/internal/types` + `internal/crypto` for byte-exact wire format. Secrets via
  env/KMS only; `.env` in `.gitignore`.

## Done
Builds, lints, smoke tests pass; `docker-compose up` brings it healthy. Hand business logic to
the **service-builder** agent. Close against the relevant checklist in
`workload/definition-of-done.md`.
