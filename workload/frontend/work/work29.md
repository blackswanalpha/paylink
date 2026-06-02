# work29 — Wallet & Staking + Token Send UI

> **Seeded** — expand with `/work 29` when picked up (await backend [work24](../../work/work24.md)/[work34](../../work/work34.md)).

- **Status:** seeded · **Owner:** service-builder · **Depends on:** 03,08 · backend **24,34** (wallet / token send) · **Flow:** [flow29](../flow/flow29.md)
- **Phase:** FE-3 · **Implements:** [frontendfeature.md §3.6](../../../frontendfeature.md) (Wallet & Staking — PLANNED)

## Goal
PLN wallet balance + staking positions + rewards, and the **non-custodial** token-send flow
(build → sign → broadcast) — the advanced/validator surface.

## In scope
- Wallet overview (PLN balance, staking positions, rewards); a token-send flow that builds + signs +
  broadcasts client-side (non-custodial — keys never leave the client per work34's model); staking actions.

## Out of scope
- The wallet/token-send backend (work24/34). Custodial key storage (forbidden — F.2).

## Invariants that apply
- **F.2 non-custodial** (the hard fence — signing is client-side, LinkMint never holds keys/funds), **F.1 SDK-only**, **F.5**, **F.6**, **F.7**.

## Acceptance criteria
- [ ] Wallet/staking/rewards render; token send builds→signs→broadcasts without LinkMint holding keys.
- [ ] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack" once backend work24/34 + their SDK resources exist.

## Notes / log
- Blocked on backend work24 (wallet) / work34 (token send). Non-custodial signing is the F.2 hard line.
