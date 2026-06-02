# flow13 — Public Resolve & Pay (execution recipe)

**Work item:** [work13](../work/work13.md) · **Goal recap:** the public `/pay/[plId]` resolve→pay→settle→receipt experience, non-custodial.

## Pre-flight
- [ ] Read [work13](../work/work13.md), [frontendfeature.md §3.1](../../../frontendfeature.md), backend [work01](../../work/work01.md)/[02](../../work/work02.md)/[04](../../work/work04.md), the work07 pay components.
- [ ] Confirm work03, work04, work05 are usable.
- [ ] Set work13 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map resolve + initiate + settlement-poll + the work07 components to reuse | **Explore** | reuse map |
| 2 | Design the public route, method picker, settlement + receipt, edge states | **Plan** | short design |
| 3 | Implement `/pay/[plId]` (resolve, method picker, instructions, live settle, receipt) reusing work07 parts | **service-builder** | the pay page |
| 4 | Branded not-found/expired/already-settled; motion; tests | **service-builder** | passing |
| 5 | Review **F.2 non-custodial** (no PIN/PAN) + F.3 | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Pay a PayLink end-to-end (MPesa stub) → receipt | `/verify` | observed VERIFIED |

## Done
- [ ] Acceptance criteria in [work13](../work/work13.md) met; **App** checklist complete; non-custodial audited PASS; status `done`.
