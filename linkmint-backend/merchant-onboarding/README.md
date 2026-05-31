# merchant-onboarding (work10)

Owns the **merchant lifecycle** for LinkMint — distinct from a personal user account: business
verification, document upload, bank-account linking + verification, contract acceptance, and
fee-tier assignment, governed by the state machine
`DRAFT → PENDING_VERIFICATION → ACTIVE | REJECTED | SUSPENDED`. It owns the `merchant` Postgres
schema, publishes `merchant.*` events, and (later) consumes `compliance.kyb.*` / `admin.override.*`.
Python 3.12 / FastAPI, mirroring the `identity-service` (work09) reference layout (config, error
envelope, structlog + correlation id, Redis idempotency, AES-GCM at-rest crypto, outbox, Alembic).

Non-custodial by construction — this service moves no funds. Bank-account references are stored as
**AES-256-GCM ciphertext** (the KMS stand-in); the plaintext account details are never persisted,
logged, returned, or placed in an event payload.

## Endpoints (`/v1`, JWT-authed)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/v1/merchants/onboard` | JWT | onboard a merchant for an org → `PENDING_VERIFICATION` |
| GET | `/v1/merchants/{id}` | JWT | full record (bank accounts show **status only**, never the ref) |
| POST | `/v1/merchants/{id}/documents` | JWT | multipart upload (cert of incorporation, tax id, …) |
| POST | `/v1/merchants/{id}/bank-accounts` | JWT | link a bank account → `PENDING_VERIFY` |
| POST | `/v1/merchants/{id}/bank-accounts/{bid}/verify` | JWT | mark a bank account `VERIFIED` |
| GET, POST | `/v1/merchants/{id}/contracts` | JWT | list / accept a contract version |
| GET, PATCH | `/v1/merchants/{id}/fee-tier` | JWT (admin) | read / change the fee tier |
| POST | `/internal/merchants/{id}/decision` | none (internal) | manual-review / consumer entry → `decide()` |
| GET | `/internal/healthz` · `/internal/readyz` · `/metrics` | none | ops |

Errors use the standard envelope `{"error":{code,message,details,trace_id}}`. State-mutating
endpoints honor `Idempotency-Key` (Redis, 24h).

## Auth model — JWT **consumer** (verifier-only)

merchant-onboarding does **not** issue tokens. It verifies identity-service's **RS256** access
tokens (`MERCHANT_JWT_PUBLIC_KEY_PEM`, issuer `linkmint-identity`, audience `linkmint`) — RS256 only
(HS256/`none` are rejected: alg-confusion guard). docker-compose pins a fixed dev RSA keypair across
identity-service + merchant-onboarding so a token minted by identity verifies here end-to-end; the
live JWKS fetch is a follow-up (the api-gateway precedent).

### RBAC from claims (deliberate divergence from identity-service)

identity-service is the system of record for memberships, so it reloads fresh org roles from its
`memberships` table for every RBAC check. merchant-onboarding owns **no** memberships table (one
schema per service; cross-schema FKs are disallowed), so it authorizes from the **token claims** —
the RS256 signature is the trust anchor. `require_org_member` raises `ORG_NOT_FOUND` (404, no
existence leak) for non-members; `require_admin` (`owner`/`admin`) raises `FORBIDDEN`.

## State machine

`DRAFT → PENDING_VERIFICATION → {ACTIVE, REJECTED, SUSPENDED}`; `ACTIVE → SUSPENDED`;
`SUSPENDED → {ACTIVE, REJECTED}`; `REJECTED` is terminal. `onboard` creates a merchant **directly**
at `PENDING_VERIFICATION` (the API contract returns it). `decide(approve|reinstate)` to `ACTIVE`
requires ≥1 VERIFIED bank account + ≥1 accepted contract (env-gated:
`MERCHANT_REQUIRE_VERIFIED_BANK_FOR_ACTIVE`, `MERCHANT_REQUIRE_CONTRACT_FOR_ACTIVE`).

## Events

Produced (`merchant.*`): `merchant.onboarded`, `merchant.verified`, `merchant.rejected`,
`merchant.suspended`, `merchant.bank_account.added`, `merchant.bank_account.verified`,
`merchant.contract.accepted`, `merchant.fee_tier.changed`. Written in-transaction to the
`merchant.merchant_events` outbox, then published post-commit (no plaintext bank details in any
payload). Consumed (seam): `compliance.kyb.passed`/`failed`, `admin.override.suspend`/`reinstate` —
all funnel through one guarded `MerchantsService.decide()`.

## Secrets (env/KMS only — never in code)

`MERCHANT_BANK_ENCRYPTION_KEY` (bank-ref encryption — the KMS stand-in) and
`MERCHANT_JWT_PUBLIC_KEY_PEM` (the verifier's public key) default to *unset*; ephemeral dev material
is generated at startup (zero-config local dev — though with an ephemeral public key no real token
verifies, by design). Production injects the real material via env/KMS.

## Run / test

```bash
pip install -e ".[dev]"
ruff check . && black --check . && mypy .          # lint + format + types
pytest                                             # unit + testcontainers integration, 80% gate
alembic upgrade head && uvicorn app.main:app --reload --port 8091   # run locally
```

Or via the stack: `docker compose up -d --wait` (boots Postgres + Redis + identity-service +
merchant-onboarding; a shared dev RSA keypair lets an identity token verify here).

## Deferred (follow-ups, not blocking)

Live JWKS fetch / RS256 cutover at the gateway; the real S3 object store (`MERCHANT_OBJECT_STORE_MODE=s3`,
not exercised locally); the KYB verification engine (work12) + companies-registry lookups; real
micro-deposit / MPesa name-match bank verification (currently a Phase-1 manual stub); real Kafka/SQS
event transport + the `compliance.kyb.*` / `admin.override.*` consumer wiring (work15/16); bank-key
rotation job; self-serve onboarding (Phase 3).
