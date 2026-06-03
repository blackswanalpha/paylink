# work10 — Account & Security (profile / sessions / API keys / orgs)

- **Status:** done
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
  warning → list → revoke), **Organizations** (create, list with names, members list, invite/remove with role),
  **Notifications** (real per-channel + per-event preferences — backend built in this work item).
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
- [x] Profile edit, session revoke, API-key issue(once)/list/revoke, org create + member add/remove all work via the SDK.
- [x] The one-time API-key secret is shown with a copy + warning and is not re-fetchable/persisted.
- [x] Destructive actions confirm; tables are keyboard-accessible; envelope errors surfaced.
- [x] `typecheck`/`lint`/`build` green (SDK + frontend); backend `pytest`/`ruff`/`mypy` green. Live-stack **App**/**Full stack** walkthrough still pending (needs docker-compose up).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": issue an API key (copy-once),
revoke a session, create an org + invite a member, edit profile — all against the live stack.

## Notes / log
- Shares the API-keys component with work17 (Developer Portal).
- Notifications tab is now LIVE (no longer a PLANNED placeholder). Built the backend end-to-end:
  notification-service gained a `notify.notification_preferences` table (migration `0003`), a
  `Preferences` value object + GET/PUT `/v1/notifications/preferences`, and enforcement in
  `NotificationService.intake()` (a disabled channel/event suppresses the inbox write + SMS/email).
  Preferences are scoped by `recipient_addr` (X-Creator-Addr), so the tab uses the HS256/dashboard
  client — the `/account` page now mints a dev token and also feeds the Topbar bell. SDK gained
  `notifications.getPreferences/updatePreferences` + types.
- Closed the org-name gap: identity-service gained `GET /v1/organizations` (list-my-orgs with names);
  SDK gained `organizations.list()`; `useOrganizations` now loads real names (survive refresh), with
  the roles-derived list as a silent fallback.
- Left uncommitted for review (per request). Verified: notify 95 / identity 97 pytest, SDK 127, FE 139
  tests, plus typecheck/lint/build across SDK + FE and ruff/mypy on both services.
