# flow09 — Auth (execution recipe)

**Work item:** [work09](../work/work09.md) · **Goal recap:** register/login/forgot + MFA challenge & enroll on a real identity session.

## Pre-flight
- [ ] Read [work09](../work/work09.md), [frontendfeature.md §3.2](../../../frontendfeature.md), backend [work09](../../work/work09.md) (auth endpoints).
- [ ] Confirm work03, work04, work08 are usable.
- [ ] Set work09 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map `client.auth.*` + the MFA/refresh/reuse semantics | **Explore** | auth flow map |
| 2 | Design the auth routes, session store, route guard, MFA steps | **Plan** | short design |
| 3 | Implement `/login`/`/register`/`/forgot` + MFA challenge/enroll + session store + guard | **service-builder** | auth surface |
| 4 | Wire validation (FormField), errors (work04), toasts (work07); tests | **service-builder** | passing |
| 5 | Review F.1/F.4/F.5/F.6 | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Drive register→login→MFA→reuse against the live stack | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work09](../work/work09.md) met; **App** checklist complete; status `done`.
