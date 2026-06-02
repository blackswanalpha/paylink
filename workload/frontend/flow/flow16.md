# flow16 — Admin Console (execution recipe)

**Work item:** [work16](../work/work16.md) · **Goal recap:** MFA-gated unified search + entity drill-downs + audit viewer with chain verify.

## Pre-flight
- [ ] Read [work16](../work/work16.md), [frontendfeature.md §3.4](../../../frontendfeature.md), backend [work11](../../work/work11.md)/[13](../../work/work13.md).
- [ ] Confirm work08, work09 are usable (admin SDK + MFA session).
- [ ] Set work16 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map `client.admin.*` + `client.auditLog.*` + the MFA/role gate + degraded semantics | **Explore** | data + gating |
| 2 | Design the gated console, search, drill-downs, audit viewer | **Plan** | short design |
| 3 | Implement the MFA gate, search, drill-down panels, audit viewer + verify | **service-builder** | admin console |
| 4 | Degraded-state handling + tests | **service-builder** | passing |
| 5 | Review F.4/F.7 access control | **invariant-auditor** + `/code-review` | clean diff |
| 6 | MFA-admin search/drill/audit + non-MFA refusal against live stack | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work16](../work/work16.md) met; **App** checklist complete; mutations filed PLANNED; status `done`.
