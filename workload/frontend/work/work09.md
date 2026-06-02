# work09 — Auth (login / register / forgot / MFA)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 03, 04, 08 · backend [work09](../../work/work09.md)
- **Flow:** [flow09](../flow/flow09.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.2 (Auth)

## Goal
The authentication surface — register, login, forgot-password, and the MFA (TOTP) challenge + enrollment
flows — built on the expanded SDK (work08), replacing the demo's server-minted dev JWT with a real
identity session.

## Why / context
Every authenticated surface (account, dashboard, admin, dev portal) needs a real login. identity-service
(backend work09) provides register/login/refresh/MFA/JWKS; this item is its premium UI front door.

## In scope
- `/login`, `/register`, `/forgot-password` routes; an MFA **challenge** step (when login returns an
  MFA requirement) and an MFA **enroll/verify** flow (TOTP secret + otpauth QR via `QRBlock`).
- Session handling: store the access token + transparent refresh; **reuse-detection → forced re-login**
  (coordinate with work04's 401 handling); a route-guard wrapper for protected pages.
- Premium auth layout (Ivory Premium split/centered card), inline validation via `FormField`, envelope
  errors via work04, success/redirect via work07 toasts.

## Out of scope (do NOT do here)
- Profile/sessions/API-keys/orgs management → work10. OAuth provider buttons beyond what the SDK stub
  exposes (mark PLANNED). Replacing the dev-JWT minting for *demo* routes (keep `/` working).

## Invariants that apply
- **F.1 SDK-only** (auth via `client.auth.*`), **F.4 edge-auth** (token is the only credential),
  **F.5** (envelope errors), **F.6** (labelled fields, focus order, error `aria`), **F.2** (no funds).

## Reuse first
- The expanded `client.auth.*` (work08); `FormField`/`QRBlock` (work03); `useErrorHandler` (work04);
  `notify.*` (work07); the dev-JWT mint pattern in `../../../linkmint-frontend/src/lib/jwt.ts` (as the
  fallback for demo routes only).

## Acceptance criteria
- [ ] Register → login → authenticated session works through the SDK; protected routes guard on it.
- [ ] MFA challenge appears when required; TOTP enroll shows the otpauth QR and verifies.
- [ ] Refresh is transparent; a detected refresh-reuse forces re-login (401 path via work04).
- [ ] Validation + envelope errors surfaced; no `any`; `typecheck`/`lint`/`build` green.
- [ ] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": against the live stack,
register→login→`/account`; enable MFA, log out, log in → MFA challenge; force a refresh-reuse → re-login.

## Notes / log
- Depends on work08 (SDK). The dev-JWT mint stays only for the demo wizard at `/`.
