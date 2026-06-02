# LinkMint Frontend Workload — building the premium UI, screen by screen

This folder is the **frontend process layer**: the execution-level breakdown of the premium,
enterprise web app whose contract lives in [`../../frontendfeature.md`](../../frontendfeature.md). It
mirrors the backend [`workload/`](../README.md) operating system one level down — same loop, same
`work ↔ flow` pairing, same Definition of Done — but scoped to `linkmint-frontend/`.

It does not restate facts. The **source of truth** stays:
- [`../../frontendfeature.md`](../../frontendfeature.md) — the frontend contract: invariants (F.1–F.8),
  architecture, the **Ivory Premium** design system (§2), per-surface §3.x specs, the SDK↔UI gap (§4),
  and the `feNN` backlog (§6) these items execute.
- [`../../backendfeatures.md`](../../backendfeatures.md) — the API each feature screen consumes.
- [`../../prd.md`](../../prd.md), [`../../system.md`](../../system.md) — product + protocol context.

## The loop (same as the backend)

```
  pick a frontend work item →  work/workNN.md   (WHAT: goal, scope, done-criteria)
  read its paired flow       →  flow/flowNN.md   (HOW: ordered steps, agent/skill per step)
  execute with named agent   →  service-builder (web), Explore, Plan, invariant-auditor
  verify against DoD         →  ../definition-of-done.md (App) + frontendfeature.md §7
  update the tracker         →  backlog.md
```

Each `workNN.md` is paired 1:1 with a `flowNN.md`. **Start at [`backlog.md`](backlog.md).**

## What's already built (last session)

The foundation is real, not planned — `work01` (Ivory Premium theme), `work02` (app shell + nav), and
`work18` (the flagship Merchant Dashboard) are **done** in `linkmint-frontend/src/**`; `work03`
(component library) is **in-progress** (primitives shipped, enterprise Modal/Drawer/Tabs/Form/DataTable
remain). Everything else is `todo`. The matrix in [`backlog.md`](backlog.md) is honest about this.

## Numbering & relation to the root workload

These items are numbered `work01…work30` **inside `workload/frontend/`** — namespaced by this folder,
distinct from the root `workload/work/` backend items. Each row cross-references its
`frontendfeature.md` `feNN`/§3.x surface and the **backend** `workNN` it consumes (column "BE"). The
root [`../backlog.md`](../backlog.md) carries a pointer to this tree.

## Shared process docs (reused, not duplicated)

| Concern | Doc |
|---|---|
| Protocol + frontend invariants | [`../rules.md`](../rules.md) + `frontendfeature.md` "Invariants" (F.1–F.8) |
| How code is implemented (TS/web) | [`../standard.md`](../standard.md) "TypeScript (SDK + web app only)" |
| Phase fences | [`../scope.md`](../scope.md) |
| When an item is done | [`../definition-of-done.md`](../definition-of-done.md) "App" + `frontendfeature.md §7` |
| How to verify | [`../verification.md`](../verification.md) "App" + "Full stack" |
| New item scaffolding | [`../templates/`](../templates/) (`work`/`flow`/`prompt`) |

## Driving it with Claude

- `/work <nn>` — load `workNN.md` + `flowNN.md` and execute the flow (point it at this subtree).
- `/new-work <title>` — scaffold the next pair from `../templates/`; add a row to `backlog.md`.
- `/check-invariants` — audit the diff against `../rules.md` + the frontend invariants.
- Subagent: **service-builder** (it owns the TS web app + JS SDK). Review: **invariant-auditor** + `/code-review`.

## Ground rules in one breath

1. **SDK-only** — every API call goes through `@linkmint/sdk` (F.1); never raw `fetch`.
2. **Non-custodial UI** (F.2) is the outermost fence — the UI shows instructions, never holds funds or captures PINs/PANs.
3. **Reuse first** — the built primitives (`src/components/ui/*`, `shell/*`, `theme/system.ts`) and `lib/*` before writing new code.
4. **Phase-honest** (F.7) — a PLANNED surface is marked, never faked.
5. Nothing ships until it passes the **App** DoD + `frontendfeature.md §7`. Update `backlog.md` on status change.
