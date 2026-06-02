# work22 — Accessibility Audit & Hardening (WCAG 2.1 AA)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 03–18 (the built surfaces)
- **Flow:** [flow22](../flow/flow22.md)
- **Phase:** FE-2
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §2.7 + invariant **F.6**

## Goal
A full WCAG 2.1 AA pass across the app — keyboard operability, focus management, ARIA semantics,
contrast, reduced-motion — plus an automated `axe` gate in CI so accessibility doesn't regress.

## Why / context
F.6 mandates AA, and components were built with a11y in mind, but a dedicated audit + automation
catches what per-component work misses (focus traps in modals/drawers, live-region announcements,
landmark structure, tab order across the shell).

## In scope
- Audit each surface: keyboard-only operability, visible focus, logical tab order, focus trap +
  restore in Modal/Drawer, `aria-live` for toasts/errors/loading, landmark/heading structure, form
  label/error association, contrast in both themes.
- Fix findings; add an **axe** automated check (jest-axe in component tests + an axe pass in CI).
- A short accessibility statement / a11y notes doc.

## Out of scope (do NOT do here)
- Net-new features. Screen-reader certification beyond AA self-audit.

## Invariants that apply
- **F.6 Accessibility** (this item *is* the F.6 enforcement), **F.7**.

## Reuse first
- The a11y primitives already in components (focus ring token, `aria-current`, `aria-busy`, `aria-live`
  seams in work04/06/07); Chakra v3's built-in a11y; `jest-axe`/`@axe-core` for automation; the work24 test harness.

## Acceptance criteria
- [ ] Every built surface is fully keyboard-operable with visible focus + correct tab order; modals trap+restore focus.
- [ ] Errors/toasts/loading announce via `aria-live`; landmarks + heading order correct; AA contrast in both themes.
- [ ] An `axe` check runs in CI and is green; findings fixed or filed.
- [ ] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": keyboard-only walkthrough of each surface; run
axe on each route (0 serious/critical); verify reduced-motion + contrast.

## Notes / log
- Best run after the feature surfaces (03–18) exist; pairs with work24 (tests/CI) for the axe gate.
