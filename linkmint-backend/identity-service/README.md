# identity-service (work09)

The identity foundation for LinkMint: system of record for **users, organizations, roles, API keys,
OAuth identities, and MFA**, and the issuer of the **RS256 JWTs** the api-gateway verifies. Every
other service treats identity as opaque IDs. Python 3.12 / FastAPI, mirroring the `paylink-service`
reference layout (config, error envelope, structlog + correlation id, Redis idempotency, Alembic).

Non-custodial by construction — this service moves no funds.

## Endpoints (`/v1`)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/v1/auth/register` | none | email/phone + password |
| POST | `/v1/auth/login` | none | access + refresh JWT (TOTP if enrolled) |
| POST | `/v1/auth/refresh` | refresh token | rotate access + refresh (single-use, reuse-detected) |
| POST | `/v1/auth/logout` | JWT | revoke a refresh-token session |
| POST | `/v1/auth/oauth/{provider}/start` · `/callback` | none | Google/Apple/GitHub (stub + seam) |
| POST | `/v1/auth/mfa/enroll` · `/verify` · `/disable` | JWT | TOTP (Phase 1) |
| GET | `/v1/auth/.well-known/jwks.json` | none | RS256 public key set |
| GET, PATCH | `/v1/users/me` | JWT | profile |
| POST, GET, DELETE | `/v1/users/me/api-keys` | JWT | scoped API keys (full key shown once) |
| POST | `/v1/organizations` | JWT | create org (caller becomes `owner`) |
| POST, GET, DELETE | `/v1/organizations/{id}/members` | JWT (admin) | invite/list/remove members + roles |
| GET, DELETE | `/v1/sessions`, `/v1/sessions/{id}` | JWT | list active sessions / per-session revoke |
| GET | `/internal/healthz` · `/internal/readyz` · `/metrics` | none | ops |

RBAC roles (org): `owner` > `admin` > `developer` > `operator` > `viewer`; plus user-level `payer`.
Errors use the standard envelope `{"error":{code,message,details,trace_id}}`. State-mutating
endpoints honor `Idempotency-Key` (Redis, 24h).

## Auth model

- **Access token:** RS256 JWT, 60-min, claims `sub`/`iss`/`aud`/`exp`/`jti`/`sid`/`roles`/`kyc_tier`.
  The service verifies its **own** tokens (`get_principal`) — the `/v1/users/me` round-trip is
  self-contained, no gateway required.
- **Refresh token:** opaque, SHA-256-hashed at rest, **single-use rotation**; replaying a rotated
  token revokes the whole session family and emits `identity.auth.failed`.
- **JWKS:** the public key is published at `/v1/auth/.well-known/jwks.json`. The gateway registers
  it as an **additive** RS256 consumer (`GATEWAY_JWT_RS256_*`) alongside its HS256 dev path — see the
  gateway README. The full RS256 cutover + routing identity endpoints through the gateway is a
  work05 follow-up. (`user_id`↔`creator_addr` mapping for downstream `X-Creator-Addr` is also a
  follow-up — identity tokens carry `sub`=UUID only.)

## Secrets (env/KMS only — never in code)

`IDENTITY_JWT_PRIVATE_KEY_PEM` (RS256 signing key), `IDENTITY_MFA_ENCRYPTION_KEY` (TOTP secret
encryption — the KMS stand-in), and the OAuth client secrets. All default to *unset*, in which case
ephemeral dev material is generated at startup (zero-config local dev). Passwords and API keys are
hashed with **argon2id**; MFA secrets are AES-GCM encrypted at rest.

## Run / test

```bash
pip install -e ".[dev]"
ruff check . && black --check . && mypy .          # lint + format + types
pytest                                             # unit + testcontainers integration, 80% gate
alembic upgrade head && uvicorn app.main:app --reload --port 8090   # run locally
```

Or via the stack: `docker compose up -d --wait` (boots Postgres + Redis + identity-service on :8090).

## Deferred (follow-ups, not blocking)

Real Google/Apple/GitHub OAuth verification (needs creds; `IDENTITY_OAUTH_FAKE=true` locally);
WebAuthn + SMS-OTP MFA (Phase 2); `jti` access-token denylist; MFA-key rotation job; OAuth `state`
persistence/CSRF validation; gateway calling identity to validate partner API keys; real Kafka/SQS
event transport + the `compliance.kyc.*` consumer wiring (work15/16).
