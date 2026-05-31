# work09 — identity-service (auth, users, orgs, API keys)

> **Seeded** — expand with `/work 09` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 15, 16, 17 · **Flow:** [flow09](../flow/flow09.md)
- **Phase:** 1 / MVP · **Spec:** backendfeatures.md §2.2

## Goal
The identity foundation: user/org/role CRUD, registration + login with JWT issuance, OAuth 2.0,
MFA, scoped API keys, and session management — consumed by the api-gateway for auth.

## In scope
- `/v1/auth/*` (register, login, refresh, logout, oauth start/callback, mfa enroll/verify/disable).
- `/v1/users/me`, `/v1/users/me/api-keys`, `/v1/organizations`, `/v1/organizations/{id}/members`, `/v1/sessions`.
- JWT RS256 (60-min access + refresh rotation); TOTP MFA (Phase 1); RBAC roles (owner/admin/developer/operator/viewer; payer).
- Owns the `identity` Postgres schema; publishes `identity.*` events; consumes `compliance.kyc.*`.

## Out of scope
- WebAuthn MFA, SMS-OTP fallback (Phase 2).
- The gateway's routing/rate-limiting (work05).
- KYC verification itself (work12 consumes the result).

## Invariants that apply
- Secrets/keys via env/KMS only ([rules.md](../rules.md) B); non-custodial (no funds).
- JWT signing key never in code; password hashing argon2id.

## Reuse first
- The Python/FastAPI reference layout from work01; the idempotency framework (work17);
  the event bus (work15); the standard error envelope.

## Acceptance criteria
- [ ] Register → login → refresh → authenticated `/v1/users/me` round-trip works with RS256 JWT.
- [ ] API key issuance with scopes; revocation; session listing + per-session revoke.
- [ ] TOTP enroll + verify; RBAC enforced on org endpoints.
- [ ] `identity.*` events published; tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
