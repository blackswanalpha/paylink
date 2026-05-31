# Backlog — master tracker & coverage matrix

Every work item, its paired flow, stack, the `backendfeatures.md` service it maps to, phase,
dependencies, and status. **This is the entry point for execution and the coverage matrix
against `backendfeatures.md`.** Pick an item whose phase is current and whose dependencies are
`done`, then run `/work <nn>`.

Status: `todo` · `in-progress` · `blocked` · `done`. Stack per ADR-003. Transport per ADR-004.

> **Scope:** covers the full application layer in `backendfeatures.md` (all 20 services +
> cross-cutting infra), phase-tagged ([scope.md](scope.md)). **Chain hardening
> (`blockchainfeature.md`) is NOT in this backlog** — see ADR-005 in [decisions.md](decisions.md);
> ⚠️ its P0 consensus gaps block the Phase-2 multi-validator milestone and should be tracked in a
> parallel chain backlog.

## Phase 1 — MVP

| # | Work / Flow | Service (backendfeatures.md) | Stack | Depends on | Status |
|---|-------------|------------------------------|-------|------------|--------|
| 15 | [work15](work/work15.md) / [flow15](flow/flow15.md) | Event bus + domain event catalog | Kafka/SQS | — | todo |
| 16 | [work16](work/work16.md) / [flow16](flow/flow16.md) | Double-entry ledger (shared `ledger` schema) | Python/Go lib | 15 | todo |
| 17 | [work17](work/work17.md) / [flow17](flow/flow17.md) | Idempotency framework (shared) | lib + Redis | 15 | todo |
| 18 | [work18](work/work18.md) / [flow18](flow/flow18.md) | Observability (OTel tracing + Prometheus + logs) | infra | — | todo |
| 09 | [work09](work/work09.md) / [flow09](flow/flow09.md) | 2.2 Identity service | Python/FastAPI | 15,16,17 | todo |
| 01 | [work01](work/work01.md) / [flow01](flow/flow01.md) | 2.5 PayLink service | Python/FastAPI | 15,16 | done |
| 10 | [work10](work/work10.md) / [flow10](flow/flow10.md) | 2.3 Merchant onboarding | Python/FastAPI | 09 | todo |
| 11 | [work11](work/work11.md) / [flow11](flow/flow11.md) | 2.4 Admin backoffice (read-only) | Python/FastAPI | 09 | todo |
| 05 | [work05](work/work05.md) / [flow05](flow/flow05.md) | 2.1 API gateway | Kong (ADR-008) | 09,01 | done |
| 02 | [work02](work/work02.md) / [flow02](flow/flow02.md) | 2.10 Payment orchestrator | Go/chi | 01,04,14 | done |
| 03 | [work03](work/work03.md) / [flow03](flow/flow03.md) | 2.11 Proof validator | Go/chi | 02 | done |
| 04 | [work04](work/work04.md) / [flow04](flow/flow04.md) | 2.14 MPesa adapter | Go core + Node rail | 03 | done |
| 12 | [work12](work/work12.md) / [flow12](flow/flow12.md) | 2.15 Compliance-risk (basic KYC) | Python/FastAPI | 09 | todo |
| 13 | [work13](work/work13.md) / [flow13](flow/flow13.md) | 2.17 Audit-log service | Go/chi | 15,16 | todo |
| 14 | [work14](work/work14.md) / [flow14](flow/flow14.md) | 2.18 Notification (SMS/email) | Python/FastAPI | 15 | todo |
| 08 | [work08](work/work08.md) / [flow08](flow/flow08.md) | docker-compose + CI (incremental) | infra | 01–14 | done |
| 35 | [work35](work/work35.md) / [flow35](flow/flow35.md) | fix: orchestrator rejects payable (PENDING) PayLinks | Go/chi | 01,02 | todo |

## Phase 2 — Beta

| # | Work / Flow | Service (backendfeatures.md) | Stack | Depends on | Status |
|---|-------------|------------------------------|-------|------------|--------|
| 19 | [work19](work/work19.md) / [flow19](flow/flow19.md) | 2.6 Invoice (invoices) | Python/FastAPI | 01 | todo |
| 20 | [work20](work/work20.md) / [flow20](flow/flow20.md) | 2.7 Escrow manager | Go/chi | 01,03 | todo |
| 21 | [work21](work/work21.md) / [flow21](flow/flow21.md) | 2.8 Fee-pricing service | Python/FastAPI | 10 | todo |
| 22 | [work22](work/work22.md) / [flow22](flow/flow22.md) | 2.9 Refund-dispute service | Python/FastAPI | 02,23 | todo |
| 23 | [work23](work/work23.md) / [flow23](flow/flow23.md) | 2.12 Settlement service | Go/chi | 02,10,16 | todo |
| 24 | [work24](work/work24.md) / [flow24](flow/flow24.md) | 2.13 Wallet service | Go/chi | (chain RPC) | todo |
| 34 | [work34](work/work34.md) / [flow34](flow/flow34.md) | 2.13 Token send & payment submission (build→sign→broadcast) | Go/chi + TS | 24, 06 | todo |
| 25 | [work25](work/work25.md) / [flow25](flow/flow25.md) | 2.16 Fraud-detection service | Python/FastAPI | 02,12 | todo |
| 26 | [work26](work/work26.md) / [flow26](flow/flow26.md) | 2.19 Reporting-analytics | Python/FastAPI + ClickHouse | 15 | todo |
| 27 | [work27](work/work27.md) / [flow27](flow/flow27.md) | 2.20 Reconciliation service | Go/chi | 23 | todo |
| 28 | [work28](work/work28.md) / [flow28](flow/flow28.md) | 2.14 Card adapter (Stripe) | Go | 03 | todo |
| 29 | [work29](work/work29.md) / [flow29](flow/flow29.md) | 2.14 Crypto adapter | Go | 03 | todo |
| 06 | [work06](work/work06.md) / [flow06](flow/flow06.md) | JS/TS SDK | TypeScript | 05 | done |
| 07 | [work07](work/work07.md) / [flow07](flow/flow07.md) | Web app (React) | TypeScript | 06 | done |

## Phase 3 — Mainnet

| # | Work / Flow | Service (backendfeatures.md) | Stack | Depends on | Status |
|---|-------------|------------------------------|-------|------------|--------|
| 30 | [work30](work/work30.md) / [flow30](flow/flow30.md) | 2.14 Bank adapter (Plaid/GoCardless/TrueLayer) | Go | 03 | todo |
| 31 | [work31](work/work31.md) / [flow31](flow/flow31.md) | 2.6 Subscriptions (extends 19) | Python/FastAPI | 19 | todo |
| 32 | [work32](work/work32.md) / [flow32](flow/flow32.md) | SDK suite (Python/Go/Java/Flutter) | multi | 06 | todo |
| 33 | [work33](work/work33.md) / [flow33](flow/flow33.md) | Dashboards (merchant/admin/mobile) | TS/Flutter | 06,11 | todo |

## Coverage vs backendfeatures.md (20 services)

All 20 spec services are represented above: 2.1 api-gateway(05), 2.2 identity(09),
2.3 merchant-onboarding(10), 2.4 admin-backoffice(11), 2.5 paylink(01), 2.6 invoice(19)+subs(31),
2.7 escrow(20), 2.8 fee-pricing(21), 2.9 refund-dispute(22), 2.10 orchestrator(02),
2.11 proof-validator(03), 2.12 settlement(23), 2.13 wallet(24), 2.14 adapters(04/28/29/30),
2.15 compliance(12), 2.16 fraud(25), 2.17 audit-log(13), 2.18 notification(14),
2.19 reporting(26), 2.20 reconciliation(27). Cross-cutting: event bus(15), ledger(16),
idempotency(17), observability(18). Clients/infra: SDK(06/32), web/dashboards(07/33),
docker-compose+CI(08).

## Detail level (honest status of this scaffold)

Pairs **01–02 are written in full detail**; **03–08 and 09–33 are seeded** with goal, scope
fences, acceptance criteria, stack, and a flow skeleton — ready to expand with `/work <nn>`.
Expanding a seeded item into a full spec is the **first step** of working it; the skeleton is
not the finished spec. Numbers are stable IDs; execution order is the phase + dependency columns
above, not the numeric order.

## Adding work

`/new-work <title>` scaffolds the next pair from [templates/](templates/), or copy the templates
manually, then add a row in the right phase table. Discovered side-work becomes a new row — it
never expands the active item ([scope.md](scope.md)).

## Changelog
- 2026-05-28 — Seeded MVP slice (items 01–08).
- 2026-05-28 — Expanded to full `backendfeatures.md` coverage (all 20 services + cross-cutting,
  items 09–33), phase-tagged; stack set per ADR-003; chain hardening kept out per ADR-005.
- 2026-05-28 — Added work34 (token send & payment submission: build→sign→broadcast,
  non-custodial) — closes the payer-side send-token gap; Phase 2, next to wallet/crypto.
- 2026-05-29 — work01 → in-progress: scaffolding `linkmint-backend/paylink-service` (Python/FastAPI).
  Started ahead of deps 15/16 with an event-publisher seam (no ledger coupling); create path
  submits on-chain `TxCreatePayLink` via `paylink_sendTransaction`; migrations via Alembic.
- 2026-05-29 — work01 → **done**. `linkmint-backend/paylink-service` shipped: /v1/paylinks create/get/
  list(cursor)/cancel, error envelope, Idempotency-Key (Redis 24h), structlog+correlation id,
  health/readyz/metrics, Alembic `paylink` schema, P-256 tx signing matched byte-exact to the lVM
  (golden-vector test), on-chain read-through reconciliation. 79 tests pass (unit + testcontainers
  integration), 94% cov; ruff/black/mypy clean. Verified live via `docker compose --profile e2e`
  (create→get→list→cancel against a real node). Recorded **ADR-006** (service-held signing key).
  Deferred as marked seams (future backlog): background reconciliation worker + 60s expiry sweeper
  (paylink-service); real event transport → work15; compliance/KYC gate → work12; auth gateway
  (mandatory `X-Creator-Addr`) → work05.
- 2026-05-29 — work02 → in-progress: building `linkmint-backend/payment-orchestrator` (Go/chi) — the
  **first Go/chi service** and reference template for work03/13/20/23/24/27. Note: the backlog dep
  column lists `01,04,14`, but work04→work03→work02 is a build cycle (the adapter/proof-validator are
  built *after* the orchestrator they integrate with) and the work02 contract itself says
  "Depends on: 01". So work01 (done) is the only hard build dependency; 04/14 are forward integration
  points, satisfied later. Orchestrator speaks JSON over the chain WS `/ws` datastream + JSON-RPC
  (`paylink_getPayLink`) only — it does NOT import `paylink-chain/internal/*` (byte-exact wire format
  is work03/04's concern), so the Go-`internal` import barrier doesn't apply here.
- 2026-05-29 — work02 → **done**. `linkmint-backend/payment-orchestrator` shipped (Go 1.25 / chi /
  pgx / go-redis / nhooyr WS). `/v1/payments` initiate + status; lifecycle FSM is a strict projection
  of the on-chain PayLink FSM (`AWAITING_PAYMENT→SETTLED|CANCELLED|FAILED`, no divergent states);
  WS datastream subscriber advances lifecycle on settle/cancel/fail; read-path reconcile vs
  `paylink_getPayLink` keeps GET consistent + closes reconnect gaps. Anti-replay (A.7) at three
  layers: FSM terminal guard + `applied_chain_events (paylink_id,seq)` dedupe + `Idempotency-Key`
  (Redis 24h). Error envelope, slog+correlation id, healthz/readyz/metrics, embedded numbered
  migrations (`payment` schema). **88.0% combined coverage** (unit + testcontainers integration —
  `make cover`); go build/vet/gofmt clean. Invariant audit PASS on all 8 + FSM-divergence; code-review
  fixes applied (idempotency SETNX race + cancel-safe release; WS keepalive ping + backoff; non-zero
  exit on listen failure). **Verified live**: real Postgres+Redis + a mock chain/paylink role-server —
  initiate→AWAITING, emit `paylink.verified`→SETTLED, duplicate event = single advance
  (`payment_transitions_total{AWAITING_PAYMENT→SETTLED}=1`), idempotent replay + 409 conflict. Added
  docker-compose `payment-orchestrator` service (compose config validates). **Established the Go/chi
  reference template** (the `internal/httpx` + ports/adapters layout) for work03/13/20/23/24/27.
  Deferred (consistent with project seams): domain-event transport (LogPublisher seam) → work15;
  double-entry settlement ledger → work16/23; auth gateway → work05; proof verification/broadcast →
  work03; rail callbacks → work04. Repo is not yet a git repo (rules.md) — no commit made (would need
  `git init`, ask first).
- 2026-05-29 — work03 → in-progress: building `linkmint-backend/proof-validator` (Go/chi) — verify a
  signed rail-agnostic proof and broadcast a `TxSubmitValidation` settlement tx to the lVM RPC.
  Two spec realities resolved with the user: (1) Go's `internal/` rule blocks importing
  `paylink-chain/internal/*` cross-module → expose an **additive** public `paylink-chain/pkg/lvm`
  (byte-exact wire/crypto re-export) and import via a `replace` directive (single source of truth,
  honoring "never re-derive"); (2) single-validator settlement can't reach quorum on the default
  devnet (RequiredValidations=3, zero seeded validators) → **self-contained** devnet wiring: a
  `requiredValidations:1` genesis + a devnet-flagged auto-stake on startup (no chain consensus change).
  work03 is the first item to settle a PayLink on-chain end-to-end; it also locks the proof-signature
  contract (canonical bytes + P-256) that work04's adapter must reproduce.
- 2026-05-29 — work03 → **done**. `linkmint-backend/proof-validator` shipped (Go 1.25 / chi / pgx /
  go-redis). `POST /v1/proofs` verifies the rail-agnostic proof shape (A.4) + its P-256 signature
  against trusted adapter keys, defers to the on-chain `paylink_isProofUsed` for anti-replay (A.7),
  cross-checks the on-chain PayLink (status/amount/expiry), then builds/signs/broadcasts a
  `TxSubmitValidation` settlement tx and returns **202** (chain quorum decides finality, A.3); errors
  use the standard envelope; `GET /v1/proofs/{hash}` reconciles status against the chain. Non-custodial
  (A.1) — moves no funds. **Byte-exact wire reuse, not re-derived:** added an additive public
  `paylink-chain/pkg/lvm` (type aliases + crypto wrappers + `ProofHash`/`BuildSubmitValidationTx`/
  `SignTx`) and imported it via a `replace` directive (resolves the Go `internal/` cross-module
  barrier the spec assumed away). **Single-validator settlement** made to work via a self-contained
  devnet: `genesis.devnet.json` (`requiredValidations:1`, validator pre-funded) + a devnet-flagged
  auto-stake on startup (idempotent, waits-until-active before serving; `validator_active` readiness
  gate). slog+correlation id, healthz/readyz/metrics, embedded numbered migration (`proof_validator`
  schema), Idempotency-Key (Redis 24h). **84.5% combined coverage** (unit + testcontainers
  integration); `go build/vet/gofmt` clean for both the chain (full `go test ./...` green incl. the new
  pkg) and the service. **Invariant audit PASS on all 8.** `/code-review` run: applied fixes
  (duplicate-metric mislabel; resultForExisting error masking; `Get` reconciles `received` rows;
  panic-safe nonce-commit; dropped the unused cross-check receiver field + documented the deferred
  rail-id↔chain-address binding); the flagged "nonce concurrency" item was verified a non-issue (the
  mempool nonce-orders per sender and the producer batches pending per block). **Verified live** twice
  via `docker compose --profile e2e` (fresh volume): valid proof → 202 → PayLink settles VERIFIED +
  proof marked used; replay → `already_settled` (no re-broadcast); tampered amount → 401, PayLink stays
  CREATED. Added a repo-root `.dockerignore` (proof-validator image builds from repo root for the
  `replace`); published the chain RPC port for the e2e. Established the **lVM client SDK** (`pkg/lvm`)
  + proof contract (`DESIGN.md`) for work04 (adapters) and future Go services (23/24/27).
  Deferred (filed as follow-ups, not blocking): (a) chain should return a distinct JSON-RPC "not found"
  error code so the client need not match on the message substring; (b) `pg_advisory_xact_lock` around
  migrations if >1 replica ever cold-starts together; (c) a background reconciler for `received` rows
  whose post-send bookkeeping write failed; (d) receiver identity-mapping (rail-id ↔ chain-address) to
  bind the proof receiver to the on-chain receiver. Repo is not a git repo (rules.md) — no commit made.
- 2026-05-29 — work04 → **done**. MPesa adapter shipped as a **hybrid** (**ADR-007**, on the user's
  request to use Node.js for the Daraja rail SDK): a **Go/chi core** (`adapters/mpesa/`) and a
  **Node.js Daraja rail service** (`adapters/mpesa/daraja-service/`, plain Node, built-ins only). The
  core normalizes a rail-neutral callback → the proof `{pl_id, rail:"mpesa", tx_id, amount, timestamp,
  sender, receiver, proof_signature}`, **signs it byte-exact via `paylink-chain/pkg/lvm`** (reused via
  `replace`, no re-derivation; canonical-bytes golden test vs work03), and broadcasts to the
  proof-validator (`POST /v1/proofs`, 202/already_settled). The Node rail SDK owns Daraja OAuth + STK
  push + raw STK-callback parsing (the only place MPesa wire shapes exist) and forwards rail-neutral
  fields to the core over a token-authed internal hop. `POST /v1/charges` (Idempotency-Key) starts an
  STK push and stores the `CheckoutRequestID → PayLink` correlation (Redis); `POST /v1/callbacks/mpesa`
  runs receive→normalize→sign→broadcast. **A.1**: STK collects to the *receiver's* shortcode (`PartyB`)
  — no LinkMint collection account anywhere; a wrong-amount payment is not proved. **A.4**: only the
  8-field proof crosses the core boundary (no-leak test). **A.7**: deterministic `Idempotency-Key`
  (`mpesa:<tx_id>`) + the validator/chain are the single dedupe authority. Orchestrator registration is
  **config-only** (`PAYMENT_ADAPTER_MPESA_URL`, logged at boot; not called — rail stays opaque, work02
  untouched). Go core **75% cover**, all unit + a server sign→verify e2e test pass; **Node 13 tests**
  pass; chain + orchestrator still green; `go build/vet/gofmt` clean. Added compose services
  `mpesa-adapter` + `mpesa-daraja` (`--profile e2e`) and un-ignored `adapters/mpesa` in `.dockerignore`.
  **Verified live** via `docker compose --profile e2e` (DARAJA_STUB=true, no Safaricom creds): charge →
  Daraja callback → Node → core → validator → PayLink **VERIFIED** on-chain + proof used. Recorded
  **ADR-007** (hybrid Go core + Node rail; config-only registration; Redis correlation). Deferred
  (follow-ups): per-merchant Daraja creds/shortcodes; Safaricom IP allowlist + split tokens; update the
  stale `/scaffold-adapter` skill (TS layout) to Go+Node per ADR-007. Repo is not a git repo — no commit.
- 2026-05-29 — work05 → in-progress: building `linkmint-backend/api-gateway`. **Owner chose Kong over
  the custom-FastAPI option** (both were allowed by the work05 spec) → recorded **ADR-008** (amends
  ADR-003's Python/FastAPI row for api-gateway). Started ahead of dep work09 (identity-service) via a
  config-only JWT seam (HS256 dev secret now; RS256 registered-key for work09), as the work05 spec's
  "Out of scope" explicitly permits a dev stub. Per the owner, `X-Creator-Addr` is enforced
  gateway-side only; the paired paylink-service `PAYLINK_REQUIRE_CREATOR_ADDR` flag is deferred.
- 2026-05-30 — work05 → **done**. `linkmint-backend/api-gateway` shipped: **Kong 3.7 OSS, DB-less
  declarative** (`kong/kong.yml.tmpl` rendered from env via `envsubst` at start — 12-factor; rendered
  `kong.yml` git-ignored, secrets env-only). Routes `/v1/paylinks*`→paylink-service and
  `/v1/payments*`→payment-orchestrator (`strip_path:false`); unknown paths → **404** in the standard
  envelope (catch-all route + `request-termination`). Auth passes on **either** a JWT (`jwt` plugin,
  HS256 dev / RS256 work09 seam, `iss`-bound credential, `exp` verified) **or** a partner API key
  (`key-auth`, **header-only**), via the shared-`anonymous`-fallback pattern; a single global
  serverless `post-function` (sandboxed, `cjson` allow-listed) then hard-**401**s the anonymous
  consumer, **injects `X-Creator-Addr`** from the verified JWT claim / key-auth consumer `custom_id`
  while **stripping any client-supplied `X-Creator-Addr`/`X-Partner-Id`** and the credentials before
  the upstream hop (ADR-006 anti-spoofing, edge-authoritative), and **normalizes every ≥400 response
  to the LinkMint envelope** `{"error":{code,message,details,trace_id}}` — passing an upstream's own
  envelope through unchanged. `correlation-id` (`X-Request-Id` reuse/generate/echo/propagate),
  Redis-backed `rate-limiting` (per-consumer → **429 + Retry-After**), `prometheus` metrics on the
  status listener (compose-net only; **admin API bound to 127.0.0.1, not exposed**). **Verified:**
  `kong config parse` clean; an isolated acceptance matrix (gateway + Redis + two echo upstreams)
  **19/19 green** (routing to each upstream; 401/404/429/502·504 envelopes; JWT valid/invalid/
  expired/bad-sig; API-key valid/invalid; **query-string creds rejected**; X-Creator-Addr inject +
  spoof-strip; credential hygiene; correlation echo+generate; rate-limit + Retry-After); and **live
  on the default-profile real stack** (`docker compose up -d --wait` all healthy incl. api-gateway →
  401 envelope without a token, routed to the real paylink-service `{"items":[]}` 200, orchestrator
  reached with its envelope passed through, `Via: kong/3.7.1`, correlation id propagated). Closes
  against the **Infra/CI + Universal DoD** (config gateway: compose-healthy + integration matrix; the
  per-language ≥80%-coverage line is N/A — no app code). **Invariant audit PASS** (A.1 non-custodial —
  reverse proxy, moves no funds; secrets env-only; admin not exposed). **`/security-review` of the
  auth surface**: `alg:none`/RS↔HS confusion, anonymous-bypass, and X-Creator-Addr spoofing all
  verified-mitigated; **fixed two Medium credential-leak paths found in review** (JWT via `?jwt=` and
  API key via `?X-API-Key=` — now header-only + Kong `hide_credentials`) and dropped the host-publish
  of the `:8100` metrics port. Recorded **ADR-008**; annotated `standard.md`. Deferred (follow-ups,
  not blocking): paylink-service `PAYLINK_REQUIRE_CREATOR_ADDR` enforcement + payment-orchestrator
  binding to the injected header (ADR-006/008 — services reachable directly still bypass; close with
  network policy + the flag); dynamic JWKS/OIDC at work09 (and **remove the HS256 dev credential at
  the RS256 cutover** to avoid alg-confusion); a real rotatable partner-key store (replaces the single
  declarative credential); full `/v1` OpenAPI aggregation; sliding-window/token-bucket + per-route
  rate limits (document the `fault_tolerant` fail-open); `request-size-limiting` + envelope-buffer cap;
  `nbf`/`aud` + token-lifetime caps. Repo is not a git repo — no commit.
- 2026-05-30 — work06 → in-progress: building `sdks/javascript` (TypeScript JS/TS SDK). Typed `/v1`
  client for PayLinks (create/get/list/cancel) + payments (initiate/status), error-envelope-aware,
  bearer-JWT/API-key auth pass-through to the gateway. Inventorying the exact request/response shapes
  from paylink-service + payment-orchestrator + the Kong gateway (no OpenAPI spec exists yet).
- 2026-05-30 — work06 → **done**. `sdks/javascript` shipped as `@linkmint/sdk` — a **strict-TS, zero-
  runtime-dependency** typed client over the `/v1` gateway. Surfaces: `paylinks.create/get/list/cancel`
  + `payments.initiate/get` (status), all 6 endpoints; DTO types **mirror the wire shape byte-for-byte
  (snake_case)** sourced directly from `paylink-service/app/api/v1/schemas.py` + `payment-orchestrator/
  internal/server/payments.go` (no mapping layer = no mapping-bug surface; **no OpenAPI spec existed** so
  types were hand-mirrored from source — full `/v1` OpenAPI aggregation remains a work05 follow-up).
  **Auth pass-through** via a discriminated `AuthConfig` → `Authorization: Bearer <jwt>` or `X-API-Key`;
  the SDK **never sends `X-Creator-Addr`** (gateway injects it — ADR-006/008). **Error envelope →
  typed errors:** `{error:{code,message,details,trace_id}}` + status + `X-Request-Id` mapped to a
  `LinkMintApiError` hierarchy (status-keyed subclasses `BadRequest/Unauthorized/PaymentRequired/
  Forbidden/NotFound/Conflict/RateLimit(retryAfter)/ServerError`), with `LinkMintConnectionError`/
  `LinkMintTimeoutError` for transport; `.code` is a typed-but-open union (known codes autocomplete,
  future codes still accepted). **Auto `Idempotency-Key`** (UUID) on every mutating call — required by
  the orchestrator, honored by paylink-service — overridable per call; the server/chain remain the
  dedupe authority (A.7 not weakened). Native `fetch` (injectable), 30s timeout + caller-abort
  disambiguation, cursor pagination, `Date`→ISO expiry. **Rail-agnostic (A.4):** `PayLink` carries no
  rail field; only the opaque `PaymentRail` routing label appears (on payment init). Build: `tsup` →
  ESM + CJS + `.d.ts`. **Verified:** `tsc --noEmit` strict clean; `eslint` (no-explicit-any=error) +
  `prettier` clean; **69 vitest tests pass, 98.66% stmts / 97.31% branch coverage** (≥80% gate) against
  a faithful mock-`fetch` server (real `Response` objects); built ESM **and** CJS artifacts smoke-tested
  (create→read→settle e2e + 404→NotFoundError envelope mapping). **Invariant audit PASS** (A.1 non-
  custodial / A.4 rail-agnostic / A.7 anti-replay; A.2/3/5/6/8 N/A for a client; no secrets, no `any`,
  no client `X-Creator-Addr`). **`/code-review` (high)** run: contract-fidelity finder found **0
  mismatches** vs both services; fixed one finding — non-serializable request bodies now throw a typed
  `LinkMintError` instead of a raw `TypeError` (fetch not called) + test added; auth-provider errors
  left to propagate unwrapped **by design** (caller's own error, preserves cause); cleanup findings
  consciously declined (explicit body construction + named error subclasses are deliberate). Added
  README.md + DESIGN.md. Unblocks **work07 (web app)** and **work32 (SDK suite)**. Deferred (follow-ups,
  not blocking): SDK lint/test in CI → **work08**; consume the gateway's `/v1` OpenAPI once aggregated
  (work05 follow-up) to generate/validate types; large-`amount` precision (JSON `number`, safe to
  ~9e15 minor units) → revisit with bigint/string if mainnet needs it. Repo is not a git repo
  (rules.md) — no commit made.
- 2026-05-30 — work06 **live e2e verification** against `docker compose --profile e2e` (all 9 services
  healthy). curl gateway-contract matrix + the built `@linkmint/sdk` driven against the real gateway
  (:8088): create/get/list/cancel, typed error mapping (NotFound/BadRequest/Conflict from live
  envelopes), idempotency replay + conflict, X-Creator-Addr spoof-strip, X-Request-Id echo, JWT +
  partner-API-key auth. **Full create→read→settle proven through the SDK**: SDK-created PayLink →
  MPesa adapter `/v1/charges` + stubbed Daraja STK callback → proof signed/broadcast → proof-validator
  settles on-chain (chain=VERIFIED) → SDK `paylinks.get` reads **VERIFIED** (verified_at set). No SDK
  defects found. **Discovered side-work → filed as work35**: payment-orchestrator `Initiate` accepts
  only `CREATED`, but paylink-service returns `PENDING` once the on-chain submit succeeds (compose
  default), so `POST /v1/payments` 409s `PAYLINK_NOT_PAYABLE` for any normally-created PayLink — a
  work01↔work02 integration gap (the SDK correctly surfaced it as a typed `ConflictError`; settlement
  via the adapter path is unaffected).
- 2026-05-30 — work07 → **done**. Frontend shipped at **`linkmint-frontend/`** (per the owner — not
  `apps/web`; the App DoD still applies at this path). **Next 16 / React 19, TypeScript strict (no
  `any`)**, calling the API **only through `@linkmint/sdk`** (work06) — no raw fetch. Stack per owner
  request: **Chakra UI v3** (UI), **Zustand** (client + wizard store), **Sonner** (toasts), **Feather
  icons** — a deliberate deviation from the seeded "minimal/plain-CSS" intent. 3-step wizard: create →
  M-PESA pay instructions → live settlement (polls `paylinks.get`, the on-chain source of truth).
  **Auth:** a Next **server component** mints a short-lived **HS256 dev JWT** (`node:crypto`; the secret
  stays server-side, only the token reaches the browser) matching the gateway's dev config; the SDK
  sends it as a bearer token (gateway injects `X-Creator-Addr` from the `creator_addr` claim). **CORS:**
  `next.config` `rewrites()` proxies `/v1/*` → gateway (`:8088`) so the SDK stays same-origin;
  `turbopack.root` is set to the repo root so the sibling `@linkmint/sdk` (`file:` dep) resolves.
  **A.1 non-custodial** (UI collects/holds no funds — shows M-PESA instructions only) and **A.4
  rail-agnostic** (no rail fields on core types; only the opaque `mpesa` routing label) preserved.
  **work35** handled gracefully — `payments.initiate` 409 `PAYLINK_NOT_PAYABLE` is shown as a neutral
  labeled note and the PayLink poll remains the settlement source of truth (not fixed here). Errors map
  the standard envelope to typed UI (`ErrorBanner` + Sonner toasts). **Verified:** `tsc --noEmit` strict
  + `eslint` (no-`any`) + `prettier` clean; `next build` green. **Live against `docker compose --profile
  e2e`**: server-minted JWT accepted by the gateway, no-token → 401 envelope, **create → 201 PENDING**,
  drove the M-PESA charge + Daraja-stub callback, and the Next-proxy poll flipped **PENDING → VERIFIED**
  (votes=1, `verified_at` set) — the full create→pay→settle proven through the SDK/proxy. Repo is not a
  git repo (rules.md) — no commit made.
- 2026-05-31 — work08 → **done**. The connective tissue: **CI** (`.github/workflows/ci.yml`) + a root
  **`README.md`** (run-the-stack-locally) + **`.gitignore`**. `docker-compose.yml` was already complete
  from work01–07, so this item added the missing CI and docs. **CI** is one workflow, one job per
  component, each **reusing the component's existing make/npm/pytest target** (no duplicated commands):
  `chain` (`go vet` + `go build ./... && go test ./... -count=1`), `payment-orchestrator`/`proof-validator`
  (`make lint build cover`), `mpesa-adapter` (`make lint build cover`), `mpesa-daraja` (`npm install &&
  node --test`), `paylink-service` (`ruff`/`black`/`mypy` + full `pytest` → testcontainers + 80% gate),
  `api-gateway` (`make validate` = `kong config parse`), `sdk` (`npm ci` + typecheck/lint/test/build),
  `frontend` (builds the SDK first — `@linkmint/sdk` is a `file:` dep it imports runtime values from —
  then typecheck/lint/`next build`; **no test step**, it has no test files), and a **gated** `e2e-smoke`
  (`docker compose --profile e2e up --wait` + `make -C adapters/mpesa e2e`, only on `main` / an `e2e`
  label). Triggers on PR + push to main/develop; `concurrency` cancel-in-progress; `permissions:
  contents: read`. Lint + unit + **testcontainer integration** run on every PR; the heavy full-stack
  smoke is gated. Gotchas handled: **covdata-safe** per-package `cover` for the Go ≥1.25.7 chain-
  replacers (proof-validator/mpesa-adapter — see the cover Makefiles), **full-repo checkout** for the two
  modules that `replace ../../paylink-chain`, SDK-before-frontend ordering, and the no-lockfile daraja
  service (`npm install`, no cache). **No secrets** — zero `${{ secrets.* }}`; `DARAJA_STUB=true` needs no
  Safaricom creds; the inline devnet keys live only in `docker-compose.yml` as documented local-only
  fixtures (chain `lint` left as `go vet`-only to match the chain's own Makefile, since this env's local
  `go` is 1.21.5 and its older gofmt false-flags the 1.25-formatted tree). **Verified:** dockerized
  `actionlint` clean; `docker compose config -q` for **both** default and `--profile e2e`; chain `go vet`
  + `go build ./...` + `go test ./... -count=1` green (incl. the 20.5s integration package) under the
  auto-fetched 1.25.7 toolchain; secret-scan of the new files clean. **Live**: full `docker compose
  --profile e2e` stack — **all 9 services healthy** — and the create→pay→settle smoke
  (`make -C adapters/mpesa e2e`, `TestE2E_MpesaPaymentSettles`) **PASS**. The only thing not verifiable
  locally is the actual green PR run, which requires GitHub. Closes against the **Infra/CI + Universal
  DoD** (per-language ≥80% line N/A for infra/docs). **First git commit:** initialized the repo and
  pushed to `github.com/blackswanalpha/paylink` — commits attributed to the owner (no bot/Co-Authored-By
  trailer), per the owner's request. **Scope:** work08's defined in-scope stack (chain + Postgres/Redis +
  paylink-service/payment-orchestrator/proof-validator/mpesa-adapter/api-gateway) is complete; as
  09–14 land they extend compose + CI per the living-item convention. Deferred (follow-ups, not blocking):
  path-filtered / required-check tuning + GitHub branch-protection wiring (needs the repo live on GitHub);
  running the e2e smoke on every PR (currently gated); a frontend test job once the web app has tests.
