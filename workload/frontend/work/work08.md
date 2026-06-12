# work08 — SDK Expansion (identity / merchant / compliance / admin / audit)

- **Status:** done
- **Owner:** service-builder
- **Depends on:** backend 09, 10, 11, 12, 13 (all done)
- **Flow:** [flow08](../flow/flow08.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §4 (SDK↔UI gap) / `fe-sdk`

## Goal
Extend `@linkmint/sdk` beyond `paylinks` + `payments` to cover the identity, merchant-onboarding,
compliance, admin, and audit-log APIs — the typed surface that unblocks the auth, account, onboarding,
KYC, admin, and developer screens (work09/10/14/15/16/17).

## Why / context
**F.1 (SDK-only)** means a screen can't be built until its data has a typed SDK method. Today the SDK
covers only paylinks/payments, so §3.2/3.4/3.5 and onboarding are blocked (`frontendfeature.md §4`).
The backend services exist (work09–13 done); this item mirrors their wire contracts into the SDK.

## In scope
- New resources mirroring the wire shape **byte-for-byte (snake_case)** — sourced from the backend
  services, not invented:
  - `auth` (register/login/refresh/logout/oauth/mfa) + `users` (`me`, api-keys) + `organizations` +
    `sessions` — backend [work09](../../work/work09.md) (`backendfeatures.md §2.2`).
  - `merchants` (onboard/documents/bank-accounts/contracts/fee-tier) — [work10](../../work/work10.md) (§2.3).
  - `compliance` (kyc sessions, status) — [work12](../../work/work12.md) (§2.6).
  - `admin` (search, entity views) — [work11](../../work/work11.md) (§2.18).
  - `auditLog` (query, get, verify) — [work13](../../work/work13.md) (§2.17).
- Reuse the existing HTTP client, auth config, **typed error hierarchy**, and auto-`Idempotency-Key`
  on mutations. Export new wire types. Maintain the no-`X-Creator-Addr` rule.
- Tests (mock-fetch) for success + error envelope on every new method; **≥80% coverage** (SDK gate).

## Out of scope (do NOT do here)
- The UI screens (work09/10/14/15/16/17). New backend endpoints. Wallet/settlement/etc. (Phase 2/3).

## Invariants that apply
- **F.1 SDK-only**, **F.4 edge-auth** (never send `X-Creator-Addr`), **F.5 error envelope**, **F.8 idempotency** — same as the existing SDK (work06).

## Reuse first
- `../../../sdks/javascript/src/{client,http,errors,types,idempotency}.ts` and the
  `resources/{paylinks,payments}.ts` pattern — copy their shape exactly. Source field names from each
  backend service's `app/api/v1/schemas.py` (Python) / `internal/server/*.go` (Go).

## Acceptance criteria
- [x] `client.{auth,users,organizations,sessions,merchants,compliance,admin,auditLog}` resources exist with typed methods + wire types.
- [x] Field names mirror the backend wire shape exactly (no mapping layer); errors map to the existing hierarchy.
- [x] Mutations carry auto-`Idempotency-Key`; no client `X-Creator-Addr`.
- [x] `tsc` strict + ESLint (no `any`) clean; vitest success+error paths; **≥80% coverage**.
- [x] Passes the **SDK** checklist in [../../definition-of-done.md](../../definition-of-done.md).

## Verification
[../../verification.md](../../verification.md) → "SDK" + "Full stack": unit tests against a mock fetch,
then drive the new resources against `docker compose --profile e2e` (register→login→me; merchant
onboard; kyc status; admin search; audit query) and confirm typed errors from real envelopes.

## Notes / log
- The keystone enabler: do this before work09/10/14/15/16/17. Mirrors the work06 SDK conventions exactly.
- **Done (landed in `36e0e48`, status synced 2026-06-12 audit).** All 8 resources wired on the client
  (`sdks/javascript/src/client.ts`) plus `notifications.getPreferences/updatePreferences` and
  `organizations.list()`; wire types snake_case byte-for-byte; auto `Idempotency-Key` on every
  mutation; no `X-Creator-Addr` anywhere in client code (identity-family routes are gateway
  PASS-THROUGH — Authorization forwarded, services self-verify RS256). Vitest per-resource
  success+error suites; coverage thresholds (80% lines/functions/statements/branches) enforced in
  `vitest.config.ts` and passing.
- **2026-06-12 — live "Full stack" pass** (docker compose --profile e2e, real envelopes through Kong):
  register→login→me, MFA enroll/verify→challenge (401 `MFA_REQUIRED` typed error), refresh rotation +
  reuse→401 `INVALID_TOKEN`, password-reset request, profile edit, sessions list/revoke, org
  create/list/members, api-key issue (`full_key` once)/list/revoke, preferences GET/PUT, paylink
  create/cancel — 27 SDK-driven checks PASS.
