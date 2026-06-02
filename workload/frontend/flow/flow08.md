# flow08 — SDK Expansion (execution recipe)

**Work item:** [work08](../work/work08.md) · **Goal recap:** typed SDK resources for identity/merchant/compliance/admin/audit — the enabler for the account/admin/onboarding screens.

## Pre-flight
- [ ] Read [work08](../work/work08.md), the existing `sdks/javascript/src/*`, and backend [work09](../../work/work09.md)/[10](../../work/work10.md)/[11](../../work/work11.md)/[12](../../work/work12.md)/[13](../../work/work13.md).
- [ ] Confirm backend 09–13 are `done` in [../../backlog.md](../../backlog.md).
- [ ] Set work08 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Inventory each backend's `/v1` request/response shapes (schemas.py / *.go) | **Explore** (`linkmint-backend/*`) | field-exact contract list |
| 2 | Design the resource classes + wire types mirroring the paylinks/payments pattern | **Plan** | API sketch |
| 3 | Implement `resources/{auth,users,organizations,sessions,merchants,compliance,admin,auditLog}.ts` + types + client wiring | **service-builder** | new SDK surface |
| 4 | Tests (mock fetch) success + error envelope per method; ≥80% coverage | **service-builder** | passing + coverage |
| 5 | Contract-fidelity + error/idempotency review | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Drive new resources against the live stack | `/verify` | observed real responses |

## Done
- [ ] Acceptance criteria in [work08](../work/work08.md) met; **SDK** checklist complete; status `done`; unblocks 09/10/14/15/16/17.
