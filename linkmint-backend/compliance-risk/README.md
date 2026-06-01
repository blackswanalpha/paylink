# compliance-risk (work12)

Owns LinkMint's **KYC tier orchestration** and a deterministic **transaction risk-decision**
endpoint. Phase-1 MVP: KYC tiers `0..2` (none/basic/enhanced) via a pluggable provider with HMAC
callbacks, plus a pure, table-tested risk engine (velocity, amount-vs-tier ceiling, geo mismatch,
Kenya AML threshold) returning `allow | block | review`. It owns the `compliance` Postgres schema and
publishes `compliance.*` events. Python 3.12 / FastAPI, mirroring the `merchant-onboarding` (work10)
and `admin-backoffice` (work11) reference layout (config, error envelope, structlog + correlation id,
Redis idempotency, AES-GCM at-rest crypto, outbox, Alembic).

Non-custodial by construction — this service moves no funds. **Raw PII never persists, is logged,
returned, or emitted**: KYC-callback metadata is passed through an allowlist redactor before any
write, and `kyc_records.documents` stores only that redacted metadata; the provider reference is
AES-256-GCM ciphertext (the KMS stand-in).

## Endpoints (`/v1`, standard error envelope)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/v1/kyc/sessions` | JWT (self-or-admin) | start a KYC session → `{session_id, provider_url}` (idempotent) |
| POST | `/v1/kyc/callbacks/{provider}` | **HMAC** (not JWT) | apply a provider callback (verified over the raw body) → `{ok: true}` |
| GET | `/v1/compliance/status?user_id=` | JWT (self-or-admin) | `{user_id, kyc_tier, risk_score, flags[]}` |
| POST | `/v1/risk/evaluate` | **internal** (no JWT) | `{decision, score, reasons[]}` — consumed by paylink-service (Flow E) |
| GET | `/internal/healthz` · `/internal/readyz` · `/metrics` | none | ops |

Errors use the standard envelope `{"error":{code,message,details,trace_id}}`. State-mutating
endpoints honor `Idempotency-Key` (Redis, 24h). `POST /v1/kyc/sessions` returns `409 ALREADY_VERIFIED`
when the user already holds an equal/higher tier, `400 INVALID_TIER` for a tier outside `{1,2}`.
`POST /v1/kyc/callbacks/{provider}` returns `401 INVALID_SIGNATURE` on a bad/missing `X-Signature`
and `404 UNKNOWN_PROVIDER` for an unregistered provider. `GET /v1/compliance/status` returns
`404 COMPLIANCE_NOT_FOUND` when nothing is known about the user.

### `/v1/risk/evaluate` — the fixed contract (consumed by paylink-service)

```
POST /v1/risk/evaluate            (no JWT; trusted network + optional X-Internal-Token)
{ "user_id": "<uuid>", "action": "paylink.create",
  "amount": 200000, "currency": "KES",
  "geo": "NG", "registered_country": "KE", "context": "paylink.create:PLK..." }

200 { "decision": "allow" | "block" | "review",
      "score": 0.0,
      "reasons": [ { "code": "AML_THRESHOLD", "detail": "..." }, ... ] }
```

`InternalGate`: `/v1/risk/evaluate` takes no bearer token. If `COMPLIANCE_INTERNAL_SHARED_SECRET` is
set, a constant-time `X-Internal-Token` match is required (else `401`); if unset, the trusted network
is the only control (ADR-009 / mpesa precedent).

## Risk engine (pure, deterministic, table-tested)

`evaluate(RiskInputs, RiskConfig) -> {decision, score 0..1 (3dp), reasons[]}` applies hard rules
first, then weighted soft signals, then combines:

- **LOW_KYC** — tier-0 value action with `amount > 0` → hard **block**.
- **AML_THRESHOLD** (Kenya) — `cumulative_window + amount ≥ 150_000` without enhanced KYC →
  tier ≤ 1 **block**, tier 2 cleared.
- **AMOUNT_OVER_TIER_CEILING** — amount over the per-tier ceiling (tier1 = 50_000, tier2 = ∞) → **review**.
- Soft — `VELOCITY_24H` (≥50 → 0.8 + review / ≥20 → 0.4), `VELOCITY_1H` (≥10 → 0.3),
  `GEO_MISMATCH` (geo ≠ registered → 0.3).
- Combine — `score = min(1, soft + Σ hard weights)`; **block** if any hard-block or `score ≥ 0.8`;
  **review** if any hard-review or `score ≥ 0.5`; else **allow**.

All thresholds come from `COMPLIANCE_*` env (see `.env.example`). The service resolves the engine
inputs (tier from `kyc_records`; velocity counts + cumulative-amount window from `activity_events`),
persists a `risk_scores` row, raises flags + emits events per decision, commits, then publishes.
`evaluate` does NOT append `activity_events` (so a risk read never pollutes velocity); the
`payment.initiated` consumer feeds activity.

## Auth model — JWT **consumer** (verifier-only)

compliance-risk does **not** issue tokens. It verifies identity-service's **RS256** access tokens
(`COMPLIANCE_JWT_PUBLIC_KEY_PEM`, issuer `linkmint-identity`, audience `linkmint`) — RS256 only
(HS256/`none` rejected: alg-confusion guard). The user-facing KYC/status surface authorizes
**self-or-admin** from the token claims. The live JWKS fetch is a follow-up (the api-gateway
precedent).

## Events

Produced (`compliance.*`): `compliance.kyc.passed` (payload **exactly `{user_id, tier}`** —
identity-service's `KycConsumer` contract), `compliance.kyc.failed` (`{user_id}`),
`compliance.check.passed`, `compliance.check.failed`, `compliance.flag.raised`. Written
in-transaction to the `compliance.compliance_events` outbox, then published post-commit (no raw PII
in any payload). Consumed (seam): `payment.initiated` → activity ledger for velocity;
`paylink.requested` is the synchronous `/v1/risk/evaluate` seam (no-op in the consumer).

## Data model (`compliance` schema)

`kyc_records` (user tier + encrypted provider_ref + redacted documents), `risk_scores` (append-only
decision audit), `flags` (block/warn/info), `activity_events` (velocity/AML windows, composite index
on `(user_id, occurred_at)`), `compliance_events` (outbox). No cross-schema FKs — `user_id` is an
opaque identity.users UUID.

## Secrets (env/KMS only — never in code)

`COMPLIANCE_CALLBACK_SECRETS` (per-provider callback HMAC), `COMPLIANCE_INTERNAL_SHARED_SECRET`
(optional internal-token), `COMPLIANCE_PROVIDER_ENCRYPTION_KEY` (provider-ref-at-rest — the KMS
stand-in), and `COMPLIANCE_JWT_PUBLIC_KEY_PEM` (the verifier's public key) default to *unset*;
ephemeral dev material is generated at startup (zero-config local dev — though with an ephemeral
public key no real token verifies, by design). Production injects the real material via env/KMS.

## Run / test

```bash
pip install -e ".[dev]"
ruff check . && black --check . && mypy .          # lint + format + types
pytest                                             # unit + testcontainers integration, 80% gate
alembic upgrade head && uvicorn app.main:app --reload --port 8093   # run locally
```

Or via the stack: `docker compose up -d --wait` (boots Postgres + Redis + identity-service +
compliance-risk + paylink-service; a shared dev RSA keypair lets an identity token verify here).

## Deferred (follow-ups, not blocking)

Real Kafka/SQS event transport draining the `compliance_events` outbox (work15/16; identity's
`KycConsumer` contract already honored); a real KYC vendor (Jumio/Smile/Onfido) with per-vendor
callback parsing + real per-provider HMAC secrets (`providers/http.py` scaffolded); JWKS auto-fetch
of identity's public key; cross-service amount currency/minor-unit normalization
(paylink PLN minor-units ↔ compliance KES); Phase-2 sanctions/KYB/per-jurisdiction rules/ML anomaly
(`FlagKind.SANCTIONS` reserved, unused).
