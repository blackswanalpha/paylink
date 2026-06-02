# flow10 — Account & Security (execution recipe)

**Work item:** [work10](../work/work10.md) · **Goal recap:** tabbed `/account` — profile, sessions, API keys, orgs & members.

## Pre-flight
- [ ] Read [work10](../work/work10.md), [frontendfeature.md §3.2](../../../frontendfeature.md), backend [work09](../../work/work09.md).
- [ ] Confirm work08, work09 are usable.
- [ ] Set work10 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map `client.{users,sessions,organizations}.*` + the api-key one-time-secret rule | **Explore** | data + rules |
| 2 | Design the tabbed account layout + destructive-confirm + optimistic flows | **Plan** | short design |
| 3 | Implement Profile/Security/API-keys/Orgs/Notifications(PLANNED) tabs | **service-builder** | `/account` |
| 4 | Wire tables/modals/optimistic/errors/toasts; tests | **service-builder** | passing |
| 5 | Review secret-handling + a11y | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Drive issue-key/revoke-session/org-invite against live stack | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work10](../work/work10.md) met; **App** checklist complete; API-keys component shared with work17; status `done`.
