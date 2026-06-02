# flow14 — Merchant Onboarding (execution recipe)

**Work item:** [work14](../work/work14.md) · **Goal recap:** a KYB stepper from draft to active (business→docs→bank→contracts→fee tier).

## Pre-flight
- [ ] Read [work14](../work/work14.md), [frontendfeature.md §3.3](../../../frontendfeature.md), backend [work10](../../work/work10.md) (merchant endpoints + states).
- [ ] Confirm work03, work08 are usable.
- [ ] Set work14 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map `client.merchants.*` + the merchant/bank state machines | **Explore** | data + states |
| 2 | Design the stepper, per-step forms, upload, verify flow, lifecycle banner | **Plan** | short design |
| 3 | Implement the 5-step wizard with progress persistence | **service-builder** | onboarding surface |
| 4 | Wire upload/verify/accept; F.2 bank-detail confidentiality; tests | **service-builder** | passing |
| 5 | Review F.2/F.5 + stepper a11y | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Onboard a merchant end-to-end against the live stack | `/verify` | observed ACTIVE |

## Done
- [ ] Acceptance criteria in [work14](../work/work14.md) met; **App** checklist complete; status `done`.
