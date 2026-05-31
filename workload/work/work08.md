# work08 — docker-compose + CI (full-stack local)

> **Seeded** — expand with `/work 08` when picked up. **Incremental:** extend this as each
> service in 01–07 comes online rather than waiting for all of them.

- **Status:** todo · **Owner:** service-builder · **Depends on:** 01–07 (incremental) · **Flow:** [flow08](../flow/flow08.md)
- **Phase:** MVP (see [scope.md](../scope.md))

## Goal
Provide a one-command local stack (`docker-compose up`) running the lVM node + all in-scope
services, and CI (`.github/workflows/`) that lints, builds, and tests every component on PR.

## Why / context
Makes the system runnable and keeps it green as it grows (`../../CLAUDE.md` Build/Test/Run,
"Adding a new microservice" → add to docker-compose + CI). This is the connective tissue.

## In scope
- `docker-compose.yml`: paylinkd (lVM node), Postgres, Redis, paylink-service,
  payment-orchestrator, proof-validator, mpesa adapter, api-gateway — healthy together.
- GitHub Actions: per-component lint + unit + integration on PR; chain job runs
  `go build ./... && go test ./... -count=1`.
- A short "run the stack locally" doc snippet.

## Out of scope
- Terraform/Helm/K8s, multi-AZ, production deploy (deferred).
- Monitoring dashboards (Prometheus client exists on the node; full dashboards later).
- Release/canary pipelines.

## Invariants that apply
- Secrets handling ([rules.md](../rules.md) Part B) — **no secrets in compose files or CI
  YAML**; use env injection / repo secrets.

## Reuse first
- The Makefile targets in `paylink-chain/` for the chain CI job.
- Each service's Dockerfile + docker-compose entry added in its own work item.
- The commands already in [verification.md](../verification.md) for CI steps.

## Acceptance criteria
- [ ] `docker-compose up -d` brings the in-scope stack healthy; the end-to-end flow works.
- [ ] CI runs lint + unit + integration for each component on PR and is green.
- [ ] No secrets committed in compose or workflow files.
- [ ] Passes the Infra/CI checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Full stack": `docker-compose up -d`, run the
create→pay→settle flow, `docker-compose down`; confirm CI green on a test PR.

## Notes / log
- Treat as a living item — each of work01–07 should add/extend its compose + CI entry as it
  lands, so this never becomes a big-bang at the end.
