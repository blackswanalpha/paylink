# work24 — Component Tests + Storybook + Visual Regression

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 03
- **Flow:** [flow24](../flow/flow24.md)
- **Phase:** FE-2
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §7 (frontend DoD — coverage)

## Goal
A testing + visual-documentation harness: component/unit tests (Vitest + Testing Library), a Storybook
(or equivalent) catalog of the component library, and a visual-regression gate in CI — so the premium
UI is documented and protected from regressions.

## Why / context
`frontendfeature.md §7` targets component test coverage; the app currently has no test files (work08/CI
noted this). A component catalog also serves as living design-system docs and the surface for the work22
axe gate.

## In scope
- Vitest + React Testing Library setup; tests for the component library (work03) + key hooks
  (`usePayLinks`, settlement poll) + critical flows; a meaningful coverage target toward §7.
- **Storybook** (or Ladle) cataloging every `ui/*` component with states (light/dark, loading/empty/error).
- **Visual-regression** snapshots in CI (e.g. Storybook test-runner / Playwright screenshots) + a
  frontend test job wired into `.github/workflows/ci.yml` (the work08 follow-up).

## Out of scope (do NOT do here)
- E2E of the whole stack (covered by the gated compose smoke). Net-new components.

## Invariants that apply
- **F.6** (the catalog hosts the work22 axe checks), **F.7**.

## Reuse first
- The existing Vitest config in `../../../linkmint-frontend/` (`vitest` is already a devDependency); the
  built `ui/*` components; the CI pattern in `../../../.github/workflows/ci.yml` (add a frontend test job).

## Acceptance criteria
- [ ] Vitest + Testing Library run component + hook tests; coverage tracked toward the §7 target.
- [ ] Storybook catalogs the `ui/*` library with light/dark + state variants.
- [ ] A visual-regression check + a frontend test job run in CI and are green.
- [ ] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Infra/CI": `npm test` green with coverage;
Storybook builds + renders; the CI frontend job passes on a PR.

## Notes / log
- Closes the work08 deferred "frontend test job once the web app has tests". Hosts the work22 axe gate.
