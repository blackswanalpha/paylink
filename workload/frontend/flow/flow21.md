# flow21 — Internationalization (i18n) & Localization (execution recipe)

**Work item:** [work21](../work/work21.md) · **Goal recap:** externalized strings + locale-aware formatting + RTL-readiness.

## Pre-flight
- [ ] Read [work21](../work/work21.md), [frontendfeature.md §1](../../../frontendfeature.md), `lib/money.ts`.
- [ ] Confirm work03 is usable.
- [ ] Set work21 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Inventory hard-coded strings + formatting call sites | **Explore** | i18n gap list |
| 2 | Choose the i18n lib + catalog structure + locale switcher + RTL approach | **Plan** | short design + decision |
| 3 | Wire i18n + externalize built-surface strings + locale-aware formatters | **service-builder** | i18n pipeline |
| 4 | Add a second locale + pseudo-locale; tests | **service-builder** | passing |
| 5 | Review RTL/a11y | `/code-review` | clean diff |
| 6 | Switch locale + pseudo-locale + RTL pass | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work21](../work/work21.md) met; **App** checklist complete; status `done`.
