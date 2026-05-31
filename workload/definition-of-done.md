# Definition of Done

A work item is **done** only when every box for its change type is checked. "It compiles"
is not done. Match the checklist to what the item touches; an item can span types.

## Universal (every item)
- [ ] Stays within the work item's scope ([`scope.md`](scope.md)); no creep.
- [ ] No [`rules.md`](rules.md) invariant violated — run `/check-invariants`.
- [ ] `/code-review` run; findings fixed or consciously deferred to a new backlog item.
- [ ] Conventional commit(s) with scope; no secrets committed.
- [ ] The item's status flipped in [`backlog.md`](backlog.md); follow-ups filed.

## Go chain change (`paylink-chain/`)
- [ ] `cd paylink-chain && go build ./...` succeeds.
- [ ] `go test ./... -count=1` passes (unit + integration).
- [ ] `make lint` (`go vet`) and `make fmt` (`gofmt`) clean.
- [ ] New tx types follow the full recipe (constant → payload → executor case → events → tests).
- [ ] State/merkle-root expectations are deterministic and covered by tests.

## Backend service (`linkmint-backend/`) — Python/FastAPI or Go/chi (ADR-003)
- [ ] Builds/lints clean for its stack: Python → `ruff`/`black`/`mypy`; Go → `go build`/`go vet`/`gofmt`.
- [ ] Unit tests (mocks) + integration tests (testcontainers) pass; **≥80% coverage**.
- [ ] Config is 12-factor (env vars only); structured JSON logs with correlation IDs.
- [ ] Endpoints versioned `/v1/...`; errors use the standard envelope; `healthz`/`readyz`/`metrics` present.
- [ ] `Idempotency-Key` honored on state-mutating endpoints (Redis, 24h TTL).
- [ ] Domain events published/consumed by logical name over Kafka/SQS (ADR-004).
- [ ] DB changes are numbered migrations (one schema per service); `docker-compose.yml` updated.

## Adapter (`adapters/`)
- [ ] Implements receive-callback → normalize → sign → broadcast.
- [ ] Emits ONLY the rail-agnostic proof shape; no rail-specific leakage.
- [ ] Registered in the Payment Orchestrator config.
- [ ] Tests cover a captured/representative rail callback end-to-end.

## SDK (`sdks/`)
- [ ] Typed client for the relevant `/v1` endpoints; no `any`.
- [ ] Mirrors the error envelope; covers success + error paths in tests.
- [ ] Updated in the same change as the endpoints it consumes.

## App (`apps/`)
- [ ] Talks to the API only through the SDK (not raw fetch).
- [ ] Handles loading/error states and the standard error envelope.
- [ ] Verified live against the local stack ([`verification.md`](verification.md)).

## Infra / CI (`infra/`, `.github/`, `docker-compose.yml`)
- [ ] `docker-compose up` brings the in-scope stack healthy locally.
- [ ] CI runs lint + unit + integration on PR; green.
- [ ] No secrets in workflow files; uses repo/org secrets or env injection.

## Verified, not just claimed
- [ ] The relevant section of [`verification.md`](verification.md) was actually run, and
      the result (pass/fail + output) is reported honestly.
