# work07 — Notifications & Toasts System

- **Status:** done
- **Owner:** service-builder
- **Depends on:** 03, 04
- **Flow:** [flow07](../flow/flow07.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §1 (toasts) + §3.3 notification surface seam

## Goal
A single notification system: a consistent **toast** taxonomy (success/info/warning/error/promise)
and an in-app **notification center** seam, so transient feedback and persistent alerts are uniform
and never collide with the error system.

## Why / context
The app already uses Sonner ad-hoc (e.g. copy confirmations). A premium enterprise app needs a
governed feedback layer: one styled toaster, typed helpers, promise/loading toasts for async actions,
and a place for persistent notifications (settlement alerts, KYC outcomes) — with a clear boundary to
work04 (errors decide *whether* a failure is a toast; this item owns *how* toasts look/behave).

## In scope
- A themed Sonner config (palette, position, duration, close affordance, reduced-motion-aware slide)
  and a typed `notify.{success,info,warning,error,promise}` wrapper used app-wide.
- **Promise/loading toasts** for async mutations (create PayLink, revoke key) with success/error resolution.
- An in-app **notification center** UI (bell in the topbar → panel/inbox) with read/unread state,
  backed by a local store now and the **work14 notification-service** (PLANNED) later — clearly marked
  PLANNED where it would need that backend (F.7).
- Toast↔error coordination: work04's `reportError` decides toast-vs-inline; this item renders the toast.

## Out of scope (do NOT do here)
- Error decisioning/mapping → work04. Notification **preferences** page → work10 (Account) tab.
  Real push/SMS/email delivery → backend work14.

## Invariants that apply
- **F.6** (toasts announced via `aria-live`, dismissible, not the only signal); **F.7** (the inbox is
  marked PLANNED until work14 backs it); **F.5** (no duplicate error surfacing — coordinate with work04).

## Reuse first
- The Sonner `Toaster` already configured in `../../../linkmint-frontend/src/components/ui/Provider.tsx`;
  the existing `toast.*` calls in `AddressChip.tsx`/hooks (migrate them to `notify.*`).

## Acceptance criteria
- [x] A typed `notify.*` wrapper + themed toaster replaces ad-hoc `toast.*` calls.
- [x] Promise toasts wrap ≥1 async mutation (loading→success/error).
- [x] A notification-center bell + panel renders (local store), with the work14-backed parts marked PLANNED.
- [x] Toasts are `aria-live`, dismissible, reduced-motion-aware; no double-surfacing with work04.
- [x] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": trigger a success (copy), a promise toast (create
PayLink), and a coordinated error (one surface only); open the notification center; toggle reduce-motion.

## Notes / log
- Third leg of the system-UX trio (work04 errors, work06 loading/empty, work07 notifications).
- **Done (landed in `36e0e48`, status synced 2026-06-12 audit).** `notify.*` (`src/lib/notify.ts`) is
  the sole sonner wrapper (success/info/warning/error/loading); themed Toaster in `Provider.tsx`;
  `reportError`'s toast path renders through `notify.error` (F.5, one surface only); create-PayLink
  uses `notify.loading` (not `toast.promise`). The notification center **exceeded** AC3: instead of a
  local store it shipped FULL-STACK — `components/notifications/{NotificationBell,NotificationCenter,
  NotificationItem,NotificationLiveRegion}.tsx` + `hooks/useNotifications.ts` over the real
  notification-service inbox API (`client.notifications`, scoped by `creator_addr`/X-Creator-Addr, with
  paylink-service emitting events). No PLANNED markers needed — the work14 backend already backs it.
- **2026-06-12 — live e2e pass** (docker compose --profile e2e): paylink create with the event enabled
  landed in the creator-scoped inbox; with `paylink.created` disabled via preferences the inbox write
  was suppressed (see work10 preferences). FE typecheck/lint/139 tests/build green.
