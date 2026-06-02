# flow15 — Compliance & KYC (execution recipe)

**Work item:** [work15](../work/work15.md) · **Goal recap:** KYC status + start/escalate flow + the in-context 402 gate experience.

## Pre-flight
- [ ] Read [work15](../work/work15.md), [frontendfeature.md §3.2–3.3](../../../frontendfeature.md), backend [work12](../../work/work12.md), the work04 402 handling.
- [ ] Confirm work08, work09 are usable.
- [ ] Set work15 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map `client.compliance.*` + the 402 envelope `reasons` shape | **Explore** | data + gate contract |
| 2 | Design the status panel, KYC-session flow, and 402 gate CTA | **Plan** | short design |
| 3 | Implement status panel + start/escalate KYC + the 402 gate component | **service-builder** | compliance surface |
| 4 | Wire into work11's create path + tests | **service-builder** | passing |
| 5 | Review F.5/F.7 | `/code-review` | clean diff |
| 6 | Tier-0 blocked→verify→allowed against the live stack | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work15](../work/work15.md) met; **App** checklist complete; status `done`.
