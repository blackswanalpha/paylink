# work15 — Compliance & KYC

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 08, 09 · backend [work12](../../work/work12.md)
- **Flow:** [flow15](../flow/flow15.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.2–§3.3 (KYC status / risk)

## Goal
Surface a user's compliance standing — KYC tier (0/1/2), risk flags — and the flow to start/escalate a
KYC session, plus the in-context **402 KYC gate** prompt that appears when an action is blocked.

## Why / context
compliance-risk (backend work12) gates above-threshold actions (Flow E → `402 KYC_REQUIRED`). The UI
must explain *why* an action was blocked and offer the path to verify, and show current standing — a
trust + conversion concern.

## In scope
- A KYC status panel (in `/account` and/or a dedicated route): current tier, risk flags, what each tier unlocks.
- **Start/escalate KYC**: `client.compliance` kyc-session → redirect to the provider URL → return + reflect updated tier.
- The **402 gate** experience: when work04 catches `PAYMENT_REQUIRED` (e.g. from create-PayLink), render
  a clear "verify to continue" CTA that deep-links into this flow, showing the compliance `reasons`.
- Tier-upgrade prompts where limits are near.

## Out of scope (do NOT do here)
- The risk engine / decisions (backend). Sanctions/KYB (Phase 2 backend). Admin compliance tooling → work16.

## Invariants that apply
- **F.1 SDK-only**, **F.5** (render the 402 envelope `reasons`), **F.6**, **F.7** (pending-review states honest), **F.2**.

## Reuse first
- `client.compliance.*` (work08); the work04 `402 PaymentRequiredError` handling; `StatusPill` (KYC tier
  mapping in §2.6); `notify.*` (work07); the create-PayLink 402 path from work11.

## Acceptance criteria
- [ ] KYC status (tier + flags + what-each-tier-unlocks) renders via the SDK.
- [ ] Start/escalate KYC launches a session and reflects the updated tier on return.
- [ ] A blocked action (402) shows the compliance `reasons` + a "verify to continue" CTA into this flow.
- [ ] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": as a tier-0 user, attempt an
over-threshold create → 402 with reasons → start KYC → (stub) reach tier 2 → the same action now allowed.

## Notes / log
- Tightly coupled to work04 (402 handling) and work11 (the create path that triggers the gate).
