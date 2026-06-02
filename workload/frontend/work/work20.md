# work20 — Responsive & Mobile

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 02, 03
- **Flow:** [flow20](../flow/flow20.md)
- **Phase:** FE-2
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §2.7 (responsive rules)

## Goal
Make every surface excellent on phones and tablets: a mobile **drawer** sidebar, tables that collapse
to cards, touch-friendly targets, and consistent breakpoints — completing the responsive seams left in
the shell and component library.

## Why / context
The payer flow especially is mobile-first (PRD), and merchants use phones. The shell hid the sidebar on
mobile (work02) and the DataTable left a card-collapse seam (work03); this item finishes responsive
behavior across the app.

## In scope
- A mobile **Drawer** sidebar (hamburger in the topbar) replacing the desktop rail < md.
- `DataTable` → stacked **card** layout < sm; forms/wizards reflow; modals become full-screen sheets on mobile.
- Touch target sizing (≥44px), safe-area insets, sticky action bars; verify the payer pay page (work13)
  on a phone viewport.
- A documented breakpoint scale + responsive helpers.

## Out of scope (do NOT do here)
- A native app (Flutter) — separate track. Offline/PWA depth beyond basics.

## Invariants that apply
- **F.6** (touch a11y, focus order on collapsed layouts), **F.7**.

## Reuse first
- The shell's existing responsive `display` props (work02); the DataTable card-collapse seam (work03);
  Chakra responsive token syntax; the existing `{ base, md }` patterns already in the codebase.

## Acceptance criteria
- [ ] The sidebar becomes a drawer < md; nav is reachable on mobile.
- [ ] DataTables collapse to cards < sm; modals become sheets; forms reflow without overflow.
- [ ] Touch targets ≥44px; the payer pay page is fully usable on a 360px viewport.
- [ ] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": exercise dashboard/paylinks/pay at 360/768/1280px;
open the mobile drawer; confirm no horizontal scroll or clipped controls.

## Notes / log
- Finishes the responsive seams from work02 (drawer) and work03 (table→card).
