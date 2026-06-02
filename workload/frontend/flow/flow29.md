# flow29 — Wallet & Staking + Token Send UI (execution recipe)

> **Seeded** — skeleton recipe; expand when backend [work24](../../work/work24.md)/[work34](../../work/work34.md) land.

**Work item:** [work29](../work/work29.md) · **Goal recap:** PLN wallet/staking/rewards + non-custodial token send.

## Pre-flight
- [ ] Read [work29](../work/work29.md); confirm backend work24/34 + their SDK resources (work08) are `done`.
- [ ] Set work29 → `in-progress`.

## Steps
| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map the wallet/staking API + the build→sign→broadcast model (work34) | **Explore** | data + signing model |
| 2 | Design wallet overview + send flow (client-side signing) | **Plan** | short design |
| 3 | Implement wallet/staking/rewards + token send | **service-builder** | screens |
| 4 | Tests + F.2 non-custodial audit | **invariant-auditor** | passing + PASS |
| 5 | Review + verify | `/code-review` + `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work29](../work/work29.md) met; **App** checklist complete; non-custodial audited; status `done`.
