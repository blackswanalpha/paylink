# work03 — Core Component Library

- **Status:** done
- **Owner:** service-builder
- **Depends on:** 01, 02
- **Flow:** [flow03](../flow/flow03.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §2.5 (component-library spec)

## Goal
The reusable, token-driven component kit every screen composes from — so feature items (work09–18)
assemble pages instead of restyling primitives. Premium, accessible, consistent.

## Why / context
An enterprise UI is only as consistent as its components. `frontendfeature.md §2.5` lists the kit;
the primitives shipped with the foundation, but the **interactive enterprise patterns**
(Modal/Drawer/Tabs/Form-Field/DataTable) still need first-class, themed wrappers with a11y baked in.

## In scope
- **Shipped primitives (done):** `Panel`, `PageHeader`, `MetricCard`, `Sparkline`, `EmptyState`,
  `Skeleton` (+ MetricCard/TableRows variants), `StatusPill` (token-driven), `AmountDisplay`,
  `AddressChip`. Keep these as the baseline.
- **To build (this item):** `Modal`/`Dialog`, `Drawer`, `Tabs`, a `FormField` wrapper (label/help/
  error wired to validation), `DataTable` (sortable header, hairline rows, cursor "Load more",
  row-card collapse seam for work20), `CopyField`, `QRBlock`, `Stepper`/`Wizard` (extract from the
  work07 flow), `Button` variant coverage (solid/outline/ghost/gold), `Avatar`, `Tooltip`, `Menu`.
- A barrel/export surface and usage docs in code comments.

## Out of scope (do NOT do here)
- Storybook + visual-regression → work24. Motion specifics → work05. Feature pages → work09–18.

## Invariants that apply
- **F.6 Accessibility** — every interactive component keyboard-operable, labelled, focus-ringed;
  **F.5** — error-state styling consumes the `DisplayError` shape (no bespoke error UI).

## Reuse first
- The shipped primitives in `../../../linkmint-frontend/src/components/ui/*`; Chakra v3 compound
  components (`Dialog`, `Drawer`, `Tabs`, `Table`, `Menu`) as the unstyled base to wrap with tokens;
  `lib/money.ts` (AmountDisplay), the `KeyValueRow` copy idiom (CopyField/AddressChip).

## Acceptance criteria
- [x] Modal/Drawer/Tabs/FormField/DataTable/CopyField/QRBlock/Stepper shipped, token-styled, a11y-clean.
- [x] A demo/kitchen-sink route or test renders each in light theme without overflow/contrast issues.
- [x] No `any`; `typecheck`/`lint`/`build` green.
- [x] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": typecheck/lint/build; render the kit and tab through it (keyboard + focus).

## Notes / log
- **2026-06-01 — in-progress.** Foundation primitives shipped last session
  (`src/components/ui/{Panel,PageHeader,MetricCard,Sparkline,EmptyState,Skeleton,StatusPill,AmountDisplay,AddressChip}.tsx`).
  Remaining: the interactive enterprise components above. Several feature items (11/13/14) depend on
  Modal/Drawer/Stepper, so prioritize those.
- **2026-06-01 — done.** Shipped all 12 interactive components wrapping Chakra v3 compounds, token-styled +
  a11y-clean: `Button`(+`GoldButton`), `Modal`, `Drawer`, `Tabs`, `Stepper`, `FormField`, `Tooltip`, `Menu`,
  `Avatar`, `CopyField`, `QRBlock`, `DataTable` — plus the barrel `src/components/ui/index.ts` (kit + folded-in
  primitives). Notable decisions: `GoldButton` uses the semantic `gold.*` tokens via style props because the
  `champagne` ramp stops at 600 and `colorPalette="champagne"` would reference missing 700–950 (extending the
  ramp filed below as a follow-up); `QRBlock`/`CopyField` need no new deps (`QrCode.*` is native, CopyField
  reuses the AddressChip clipboard idiom); `DataTable` is generic, client-side-sortable (`aria-sort`), with a
  caller-driven cursor "Load more" and a `hideBelow`/`cardLabel` seam for the work20 card layout (seam only).
  Verification: net-new Vitest harness (`vitest.config.ts`, `vitest.setup.ts` with jsdom shims, `src/test/renderWithTheme.tsx`)
  + a 19-case a11y smoke test (`src/components/ui/__tests__/kit.smoke.test.tsx`); dev-only `/kitchen-sink` gallery
  (`src/app/kitchen-sink/page.tsx` → `src/components/dev/KitchenSink.tsx`, 404s in prod) verified HTTP 200.
  `typecheck` / `lint` / `test` (19/19) / `build` all green. **Unblocks work11/13/14 (Modal/Drawer/Stepper) and
  work07/14 (Stepper).**
- **Follow-ups filed:** (1) extend the `champagne` ramp to a full 50–950 palette so `colorPalette="champagne"`
  works and `GoldButton` can drop its style-prop overrides. Storybook + visual-regression remains work24; the
  responsive row-card layout remains work20.
- **2026-06-02 — kit adopted into product pages.** Wired the kit into the live surfaces: `/dashboard`
  (`MerchantDashboard.tsx`) "Recent PayLinks" now renders `<DataTable>` (sortable Amount/Created), importing
  primitives from the `@/components/ui` barrel; the `/` wizard `CreatePayLinkForm.tsx` rebuilt on `<FormField>`
  (same validation) and `PayLinkDemo.tsx` gained a `<Stepper>` (create→pay→settlement). typecheck/lint/test
  (19/19)/build green; `/dashboard` + `/` render HTTP 200 with no error overlay.
