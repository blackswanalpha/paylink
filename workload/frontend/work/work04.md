# work04 — Error & Feedback System

- **Status:** done
- **Owner:** service-builder
- **Depends on:** 03
- **Flow:** [flow04](../flow/flow04.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §1 + invariant **F.5** (error-envelope-aware)

## Goal
A single, app-wide system that turns any failure — API envelope, transport, render crash — into a
calm, consistent, actionable surface. No raw stack traces, no swallowed errors, every API error
carries its `trace_id` for support.

## Why / context
Errors are where premium apps feel cheap or trustworthy. The SDK already throws a **typed** hierarchy
and `lib/errors.ts:toDisplayError` normalizes it to a `DisplayError`; this item turns that into a
**system**: one place that decides inline-vs-toast, retry, re-auth, and status-specific UX, used by
every screen so no feature reinvents error handling. (`frontendfeature.md` F.5.)

## In scope
- **React `ErrorBoundary`** wrapping the app + per-route segments (Next `error.tsx`), rendering a
  branded fallback with a "reload / go home" action and a copyable error id.
- A central **error presenter**: given a `DisplayError`, choose inline (`ErrorBanner`) vs toast vs
  full-page; expose a `useErrorHandler()` hook + a `reportError(err)` helper feature code calls.
- **Status-specific handling** mapped from the envelope: `401` → session-expired re-auth prompt
  (hook into work09); `402 KYC_REQUIRED` → KYC gate CTA (work15); `403` → forbidden panel; `404` →
  not-found route page; `409` → contextual conflict copy; `429` → rate-limit with `Retry-After`
  countdown; `5xx`/transport → "try again" + the stale-service-worker hint already in `errors.ts`.
- **`trace_id`/`request_id` surfacing** with copy, plus the standard 404 and 500 route pages
  (`app/not-found.tsx`, `app/error.tsx`, `app/global-error.tsx`).
- A retry primitive (idempotent reads only) + an offline/online banner.

## Out of scope (do NOT do here)
- The toast transport/notification center → work07. The actual login screen → work09. KYC screen → work15.

## Invariants that apply
- **F.5 error-envelope-aware** (this item *is* F.5's implementation); **F.1 SDK-only** (errors come
  from the SDK hierarchy, not bespoke fetch); **F.6** (error surfaces are reachable + announced via `aria-live`).

## Reuse first
- `../../../linkmint-frontend/src/lib/errors.ts` (`toDisplayError`, `DisplayError`, `isAbortError`,
  the transport/service-worker copy) and `src/components/ErrorBanner.tsx` — extend, don't replace.
- The SDK error classes (`LinkMintApiError` subclasses, `RateLimitError.retryAfter`,
  `PaymentRequiredError`) from `@linkmint/sdk`.

## Acceptance criteria
- [x] An `ErrorBoundary` + Next `error.tsx`/`not-found.tsx`/`global-error.tsx` render branded fallbacks.
- [x] `useErrorHandler()`/`reportError()` route a `DisplayError` to inline vs toast consistently.
- [x] 401/402/403/404/409/429/5xx/transport each have a distinct, tested presentation; `trace_id` is copyable.
- [x] Errors are announced to assistive tech (`aria-live`); retry only fires on idempotent reads.
- [x] No `any`; `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": force each class (kill the gateway →
transport; bad token → 401; tier-0 over-threshold create → 402; unknown id → 404; hammer an endpoint →
429) and confirm the mapped UX + `trace_id`. Throw in a child to prove the boundary catches it.

## Notes / log
- This is the **system-UX exemplar** — feature items must route errors through it, never hand-roll.
- **Done.** Pure `classifyError` decision table in `src/lib/errors.ts` (+ `newErrorId`); imperative
  `reportError(err, opts)` (`src/lib/reportError.ts`) → toast (Sonner) / overlay store / inline; hooks
  `useErrorHandler` + `useRetry` (idempotent-reads-only, 429 `Retry-After` countdown). Components:
  extended `ErrorBanner` (copyable trace via CopyField, retry+cooldown, `aria-live`, CTA slot),
  `ErrorBoundary` (class) + `ErrorFallback`, `GlobalErrorOverlays` (401 re-auth + 402 KYC `alertdialog`
  seams), `KycGate` (inline 402), `OfflineBanner`. Route files `app/not-found.tsx` + `app/error.tsx` +
  `app/global-error.tsx` (self-contained, inline brand colors) + `app/dashboard/error.tsx` (per-segment).
  Mounted in `Provider`. 33 new Vitest specs (52 total) green; typecheck/lint/build green.
- **Decisions (with user):** existing call sites migrated to route through `reportError`
  (`useCreatePayLink`/`useInitiatePayment`/`useSettlementStatus`/`usePayLinks` + dashboard retry); 402 is
  **configurable** — default global modal, opt-in inline `KycGate` (wired on the live create-flow 402).
- **Seams (not stubs):** toast transport = existing Sonner (center → work07); "Sign in again" → work09;
  "Verify identity" → work15. `/kitchen-sink` has an "Error & feedback system" panel to drive each class.
