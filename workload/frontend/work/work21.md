# work21 — Internationalization (i18n) & Localization

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 03
- **Flow:** [flow21](../flow/flow21.md)
- **Phase:** FE-2
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) (locale/currency in §1) / `fe14`

## Goal
Make the app translatable and locale-aware: externalized strings, locale-aware number/currency/date
formatting, and an RTL-ready layout — starting with English + a second locale to prove the pipeline.

## Why / context
LinkMint targets East Africa first then global (PRD). Hard-coded English + ad-hoc formatting blocks
that. Money is already integer minor units (`lib/money.ts`); this item formalizes locale formatting and
string externalization so future locales are config, not code.

## In scope
- An i18n library + message catalog; externalize UI strings (start with the built surfaces);
  a locale switcher; `Intl`-based number/currency/date formatting (extend `formatMinorUnits`).
- RTL-readiness (logical CSS properties, direction-aware components); a pseudo-locale for QA.
- One real second locale (e.g. Swahili) to prove the round-trip.

## Out of scope (do NOT do here)
- Translating every string for production (ongoing content work); server-driven locale negotiation depth.

## Invariants that apply
- **F.6** (RTL + translated content stays accessible), **F.7**.

## Reuse first
- `../../../linkmint-frontend/src/lib/money.ts` (`formatMinorUnits`, already `toLocaleString`-based) —
  extend to a locale-aware formatter; the `NEXT_PUBLIC_DEFAULT_CURRENCY`/locale env seam.

## Acceptance criteria
- [ ] UI strings on the built surfaces are externalized; a locale switcher changes language live.
- [ ] Numbers/currency/dates format per locale; a second locale renders correctly; RTL layout doesn't break.
- [ ] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": switch locale → strings + formats update; enable
the pseudo-locale to spot un-externalized strings; check an RTL pass.

## Notes / log
- Keep money in minor units internally; only formatting is localized. Pairs with work22 (a11y) for language attrs.
