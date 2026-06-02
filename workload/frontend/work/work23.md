# work23 — Command Palette & Global Search (⌘K)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 02
- **Flow:** [flow23](../flow/flow23.md)
- **Phase:** FE-2
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §1 (navigation / power-user)

## Goal
A ⌘K command palette for fast navigation and actions (go to a surface, create a PayLink, search an
entity by id), giving the app the keyboard-driven feel power users expect of enterprise tools.

## Why / context
Enterprise SaaS (Linear/Stripe-grade, the design north star) lives on ⌘K. It also unifies navigation
and quick actions in one accessible surface, complementing the sidebar.

## In scope
- A ⌘K / Ctrl-K command palette (modal) with fuzzy command list: navigate to any surface, "Create
  PayLink", "Find by id" (PayLink/payment via `client.*.get`), recent items, theme toggle.
- Keyboard-first (arrow/enter/esc), grouped results, empty/loading states (work06), reduced-motion (work05).
- Optional: global search field in the topbar that opens the palette.

## Out of scope (do NOT do here)
- Full server-side search index (uses existing get/list + the admin search in work16). AI/NL commands.

## Invariants that apply
- **F.6** (fully keyboard-operable, focus trap, announced), **F.1 SDK-only** (lookups via SDK), **F.7**.

## Reuse first
- The nav model `nav.ts` (work02) for navigation commands; `Modal` + `useReducedMotion` (work03/05);
  `client.{paylinks,payments}.get` for id lookups; the admin search (work16) where applicable.

## Acceptance criteria
- [ ] ⌘K opens a palette; arrow/enter/esc work; commands navigate + run quick actions.
- [ ] "Find by id" resolves a PayLink/payment via the SDK; recent items + theme toggle present.
- [ ] Fully keyboard-operable + focus-trapped; reduced-motion respected.
- [ ] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": ⌘K → navigate, create, find-by-id; keyboard-only;
toggle reduce-motion.

## Notes / log
- Builds on the nav model from work02; reuses lookups rather than a new search backend.
