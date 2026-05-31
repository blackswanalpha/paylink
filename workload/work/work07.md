# work07 — web app (React: create → pay via MPesa → settle)

> **Seeded** — expand with `/work 07` when picked up.

- **Status:** done · **Owner:** service-builder · **Depends on:** 06 · **Flow:** [flow07](../flow/flow07.md)
- **Phase:** 2 / Beta (see [scope.md](../scope.md))

## Goal
Build `apps/web` — a minimal React app demonstrating the end-to-end flow: create a PayLink,
pay it via MPesa, and watch it settle. The MVP's user-facing proof of concept.

## Why / context
Ties the whole stack together for a human: the "create PayLink, pay via MPesa, settle
on-chain" goal from the MVP milestone (`../../system.md`, `../../CLAUDE.md`).

## In scope
- React app calling the API **only through the JS SDK** (work06), never raw fetch.
- Create-PayLink form → display pay instructions → live settlement status.
- Handle loading/error states and the standard error envelope.
- TS strict, no `any`; basic, clean UI (not a design exercise).

## Out of scope
- Card/crypto payment UI (MPesa only this phase).
- Mobile app / CLI (deferred).
- Auth/account management beyond what the gateway requires for the demo flow.
- Production build/hosting concerns.

## Invariants that apply
- **A.1 Non-custodial** (UI never collects/holds funds — it shows MPesa pay instructions;
  payment happens in MPesa), **A.4 Rail-agnostic** in the shared parts; TS standards.

## Reuse first
- The JS SDK (work06) for all API calls.
- The error envelope shape for consistent error display.

## Acceptance criteria
- [x] Create-PayLink form works via the SDK; shows a created PayLink.
- [x] Displays MPesa pay instructions; reflects live settlement status.
- [x] Loading + error states handled; error envelope surfaced to the user.
- [x] No raw fetch (SDK only); strict TS, no `any`.
- [x] Passes the App checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "App" + "Full stack": `npm run dev` against the local
stack; drive create → pay (MPesa sandbox) → settle; use `/run` to launch/screenshot.

## Notes / log
- Keep it minimal — it's the end-to-end demonstration, not the product UI.
- 2026-05-30 — **done**. Built at **`linkmint-frontend/`** (per owner — not `apps/web`; App DoD still
  applies at this path). Per owner the stack is **Next 16 / React 19 + Chakra UI v3 + Zustand + Sonner +
  Feather icons** (a deliberate deviation from the seeded "minimal/plain-CSS" intent). API access is
  **only via `@linkmint/sdk`** (no raw fetch); a Next server component mints a dev HS256 JWT
  (secret server-side); `next.config` rewrites `/v1/*` to the gateway (same-origin). Live-verified
  against `docker compose --profile e2e`: create → 201 PENDING → M-PESA charge + Daraja-stub callback →
  proxy poll flipped **PENDING → VERIFIED**. work35 handled gracefully (labeled note, not fixed). See
  `linkmint-frontend/README.md`.
