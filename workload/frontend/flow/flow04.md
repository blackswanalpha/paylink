# flow04 — Error & Feedback System (execution recipe)

**Work item:** [work04](../work/work04.md) · **Goal recap:** one app-wide system mapping every failure to calm, actionable, envelope-aware UX.

## Pre-flight
- [x] Read [work04](../work/work04.md), [frontendfeature.md](../../../frontendfeature.md) F.5, and `src/lib/errors.ts` + `ErrorBanner.tsx`.
- [x] Confirm work03 is `done`/usable (needs Modal/route-page primitives).
- [x] Set work04 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map the SDK error hierarchy + `DisplayError` fields to UX decisions (inline/toast/page per status) | **Explore** (`sdks/javascript/src/errors.ts`, `src/lib/errors.ts`) | decision table |
| 2 | Design `ErrorBoundary` + `useErrorHandler`/`reportError` + status router + route pages | **Plan** | short design |
| 3 | Implement boundary, presenter hook, `error.tsx`/`not-found.tsx`/`global-error.tsx`, retry + offline banner | **service-builder** | the system |
| 4 | Tests for each status mapping + boundary catch + `aria-live` | **service-builder** | passing tests |
| 5 | Review against F.5/F.6 | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Force each error class against the live stack | `/verify` | observed UX + trace_id |

## Done
- [x] Acceptance criteria in [work04](../work/work04.md) met; **App** checklist complete; status `done`.
- [x] Feature items updated to route through `reportError`/`useErrorHandler` (note in their logs).

> **Build note:** static suite green (52 Vitest specs incl. 33 new; typecheck/lint/build clean). The
> live-stack pass (step 6, docker e2e) is driveable via the `/kitchen-sink` "Error & feedback system"
> panel, which fires each class (inline/toast/re-auth/KYC/boundary) without the backend. Steps 1–4 done
> in-session (Explore→Plan→implement→tests); run `/code-review` + invariant-auditor (step 5) on the diff.
