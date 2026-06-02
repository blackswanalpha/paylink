# flow07 — Notifications & Toasts System (execution recipe)

**Work item:** [work07](../work/work07.md) · **Goal recap:** one governed toast taxonomy + a notification-center seam, coordinated with the error system.

## Pre-flight
- [ ] Read [work07](../work/work07.md), [frontendfeature.md §1](../../../frontendfeature.md), the Sonner config in `Provider.tsx`, and work04's `reportError` contract.
- [ ] Confirm work03 + work04 are usable.
- [ ] Set work07 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Inventory current `toast.*` usage + the work04 boundary | **Explore** | toast call sites + coordination rule |
| 2 | Design the `notify.*` API, themed toaster, promise toasts, notification-center store/UI | **Plan** | short design |
| 3 | Implement `notify.*` + themed toaster; migrate existing calls; build the bell/inbox (local store, PLANNED seam to work14) | **service-builder** | the system |
| 4 | Wrap an async mutation in a promise toast; coordinate error single-surfacing; tests | **service-builder** | used + passing |
| 5 | Review a11y + no double-surfacing | `/code-review` | clean diff |
| 6 | Trigger success/promise/error + open inbox + reduce-motion | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work07](../work/work07.md) met; **App** checklist complete; the work14-backed inbox parts marked PLANNED; status `done`.
