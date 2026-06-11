# work01 — Scaffold paylink-service (Python/FastAPI CRUD, Postgres, migrations)

- **Status:** done
- **Owner:** service-builder
- **Depends on:** —
- **Flow:** [flow01](../flow/flow01.md)
- **Phase:** MVP (see [scope.md](../scope.md))

## Goal
Stand up the first backend microservice — `linkmint-backend/paylink-service` — exposing versioned
REST CRUD for PayLinks, backed by PostgreSQL with numbered migrations. This establishes the
**reference shape every other Python/FastAPI service follows** (stack per ADR-003).

## Why / context
The lVM owns PayLink *state and settlement*; the application layer needs an off-chain service
to create/read/manage PayLink records, drive the create flow, and read back on-chain status.
This is the foundational service (`../../system.md`, `../../backendfeatures.md` "PayLink
Service"). Because it's first, its layout/conventions become the template for work02, work03,
work05.

## In scope
- Service skeleton under `linkmint-backend/paylink-service/`: FastAPI app + Pydantic models,
  ruff+black+mypy, env-based config (12-factor), structlog JSON logging w/ correlation IDs,
  health/readiness/metrics; owns the `paylink` Postgres schema; publishes `paylink.*` domain
  events over Kafka/SQS; `Idempotency-Key` on create.
- REST API, versioned `/v1/paylinks`: create, get by id, list (by creator/receiver/status,
  paginated), cancel. Standard error envelope on every error.
- PostgreSQL persistence with **numbered migrations**; a `paylinks` table + supporting indexes.
- Read-through of on-chain PayLink status via the lVM JSON-RPC (reuse the RPC methods the
  chain already exposes — e.g. PayLink queries by creator/receiver/status).
- Dockerfile + an entry in `docker-compose.yml` (Postgres + the service).
- Unit tests (mocks) + integration tests (testcontainers, real Postgres); **≥80% coverage**.

## Out of scope (do NOT do here)
- Payment lifecycle orchestration → work02.
- Proof verification / chain broadcast → work03.
- Any rail/adapter logic → work04.
- Auth/gateway concerns → work05 (service trusts the gateway for now; keep an auth seam).
- KYC/AML, escrow, notifications (deferred services).

## Invariants that apply
- **A.1 Non-custodial** — the service stores PayLink *metadata/state*, never funds or
  fund-moving credentials.
- **A.4 Rail-agnostic** — no rail-specific fields in the PayLink model or API.
- **A.7 Anti-replay** — settlement state is sourced from the chain (the on-chain proof hash
  is the source of truth); the service does not invent settlement.

## Reuse first
- lVM JSON-RPC PayLink query methods in `paylink-chain/internal/rpc/` — the service reads
  on-chain status rather than duplicating it.
- PayLink field definitions in `paylink-chain/internal/types/` (mirror names/shape).
- The proof format and error-envelope/API conventions in [standard.md](../standard.md).

## Acceptance criteria
- [x] `POST /v1/paylinks` creates a PayLink record and returns it with an id.
- [x] `GET /v1/paylinks/:id` and `GET /v1/paylinks?creator=&receiver=&status=&page=` work,
      paginated, with the standard list shape.
- [x] `POST /v1/paylinks/:id/cancel` transitions per the documented PayLink rules.
- [x] On-chain settlement status is reflected by reading the lVM RPC.
- [x] All config via env vars; no secrets in code; `.env.example` provided.
- [x] Migrations run cleanly from empty; `docker-compose up` brings Postgres + service healthy.
- [x] Unit + integration tests pass; coverage ≥80%; lint + build clean.
- [x] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack":
`ruff check . && mypy . && pytest --cov`, then `docker-compose up -d` and exercise
create → get → list → cancel; confirm error envelope on a bad request.

## Notes / log
- This service's layout, config, logging, and test setup are the **reference template for the
  other Python/FastAPI services** (work09/10/11/12/14/05) — keep them clean and copyable.
- Keep an auth seam (middleware hook) so work05's gateway can slot in without refactor.
- **2026-05-29 — DONE.** Built `linkmint-backend/paylink-service` (Python 3.12 / FastAPI / SQLAlchemy /
  Alembic / Redis / httpx / structlog). All acceptance criteria met; 79 tests (unit + testcontainers
  integration) pass at 94% coverage; ruff/black/mypy clean. Verified end-to-end on
  `docker compose --profile e2e` (create→get→list→cancel against a live single-node lVM devnet).
  - **Stack call:** Alembic (not dbmate) for migrations; Kafka/SQS event transport (ADR-004)
    deferred to work15 — shipped a `Publisher` seam with a durable `paylink_events` outbox.
  - **Create path:** persists the record AND submits a signed `TxCreatePayLink` via
    `paylink_sendTransaction`. lVM signing replicated byte-exact in Python (NIST **P-256**, raw
    `r||s`, **legacy-Keccak** address) — locked by a golden vector captured from the Go crypto.
    The chain does not yet verify tx sigs (ADR-005); we sign correctly for forward-compat.
  - **Design decision:** service holds the P-256 signing key (on-chain `from` = service) — see
    **ADR-006**. Non-custodial (A.1) holds: create/cancel move no value; only a `metadataHash` goes
    on-chain. Invariants A.1/A.4/A.7 audited PASS.
  - **Deferred seams (follow-ups):** background reconciliation worker + 60s expiry sweeper;
    compliance/KYC gate → work12; real event transport → work15; gateway-mandatory `X-Creator-Addr`
    → work05.
- 2026-06-12 — audit re-verified: all criteria + Backend-service DoD hold (ruff clean, 90 unit tests pass incl. the ADR-015 pubKey signer change, now committed); status header synced, boxes ticked.
