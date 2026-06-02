# work10 — Account & Security (profile / sessions / API keys / orgs)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 08, 09 · backend [work09](../../work/work09.md)
- **Flow:** [flow10](../flow/flow10.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.2 (Account)

## Goal
The self-serve account area: profile, active sessions, API keys, organizations & members, plus a
notification-preferences tab — every identity-service capability surfaced in a premium, tabbed page.

## Why / context
Enterprise users manage their own security. identity-service exposes profile/sessions/api-keys/orgs;
this turns them into a polished `/account` surface (the developer portal reuses the API-keys part).

## In scope
- `/account` with tabs: **Profile** (email/phone edit), **Security** (sessions list + revoke; MFA
  enable/disable linking work09), **API keys** (issue → secret shown **once** with a strong "copy now"
  warning → list → revoke), **Organizations** (create, members list, invite/remove with role),
  **Notifications** (preferences — PLANNED seam to backend work14, marked per F.7).
- DataTable (work03) for sessions/keys/members; confirm-destructive Modal for revoke/remove; optimistic
  updates (work06) with rollback.

## Out of scope (do NOT do here)
- Login/MFA-challenge itself → work09. Real notification delivery → backend work14. Billing (none in scope).

## Invariants that apply
- **F.1 SDK-only**, **F.5** (envelope errors), **F.6** (tab a11y, focus, destructive-action confirms),
  **F.8** (idempotent issue/revoke). The API-key secret is shown once and **never persisted** in app state.

## Reuse first
- `client.{users,sessions,organizations}.*` (work08); `DataTable`/`Modal`/`Tabs`/`CopyField` (work03);
  optimistic helper (work06); `notify.*` (work07).

## Acceptance criteria
- [ ] Profile edit, session revoke, API-key issue(once)/list/revoke, org create + member add/remove all work via the SDK.
- [ ] The one-time API-key secret is shown with a copy + warning and is not re-fetchable/persisted.
- [ ] Destructive actions confirm; tables are keyboard-accessible; envelope errors surfaced.
- [ ] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": issue an API key (copy-once),
revoke a session, create an org + invite a member, edit profile — all against the live stack.

## Notes / log
- Shares the API-keys component with work17 (Developer Portal). Notifications tab is a PLANNED seam to work14.
