# flow12 — Payments (execution recipe)

**Work item:** [work12](../work/work12.md) · **Goal recap:** payments list + detail timeline, rail-agnostic, cross-linked to PayLinks.

## Pre-flight
- [ ] Read [work12](../work/work12.md), [frontendfeature.md §3.3](../../../frontendfeature.md), backend [work02](../../work/work02.md).
- [ ] Confirm work03 is usable; check `payments.list` exists in the SDK (else file under work08).
- [ ] Set work12 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map `client.payments.*` + the lifecycle FSM | **Explore** | data + states |
| 2 | Design the list + timeline detail + live-poll | **Plan** | short design |
| 3 | Implement list + detail timeline + poll | **service-builder** | payments surface |
| 4 | Tests + rail-agnostic check | **service-builder** | passing |
| 5 | Review F.3 + a11y | `/code-review` | clean diff |
| 6 | Drive a payment to SETTLED; watch the timeline | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work12](../work/work12.md) met; **App** checklist complete; status `done`.
