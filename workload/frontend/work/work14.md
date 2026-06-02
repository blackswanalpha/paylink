# work14 — Merchant Onboarding (KYB stepper)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 03, 08 · backend [work10](../../work/work10.md)
- **Flow:** [flow14](../flow/flow14.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.3 (Onboarding) / `fe09`

## Goal
A guided KYB onboarding wizard that walks a merchant from draft to active: business details → document
upload → bank-account add + verify → contract acceptance → fee-tier — with a clear status of where
they are in the `DRAFT → PENDING_VERIFICATION → ACTIVE` lifecycle.

## Why / context
A merchant can't receive settlements until onboarded. merchant-onboarding (backend work10) exposes the
full KYB API; this is its premium multi-step UI, with the activation preconditions surfaced (≥1
verified bank + ≥1 accepted contract).

## In scope
- `/dashboard/onboarding`: a `Stepper` (work03) with steps — **Business** (name/registration/country/
  type), **Documents** (multipart upload + status), **Bank accounts** (add → `PENDING_VERIFY` → verify
  → `VERIFIED`; details never shown back), **Contracts** (view/accept), **Fee tier** (view).
- Lifecycle banner (`StatusPill` for merchant + bank-account states); resume-where-you-left-off;
  activation-precondition hints.

## Out of scope (do NOT do here)
- Admin approval of merchants → work16. Fee-tier admin override (admin-only). Real document storage internals (backend).

## Invariants that apply
- **F.1 SDK-only**, **F.2 non-custodial** (bank details are submitted to the API, never displayed back;
  no funds), **F.5**, **F.6** (stepper a11y, upload labelling), **F.7** (verification-pending states honest).

## Reuse first
- `client.merchants.*` (work08); `Stepper`/`FormField`/`DataTable`/`Modal` (work03); `StatusPill`
  (built — merchant + bank-account states already mapped in §2.6); errors (work04); toasts (work07).

## Acceptance criteria
- [ ] The stepper drives business → documents → bank → contracts → fee tier via the SDK; progress persists.
- [ ] Bank account goes PENDING_VERIFY → VERIFIED; account details are never echoed back (F.2).
- [ ] Merchant lifecycle + activation preconditions are shown honestly; envelope errors surfaced.
- [ ] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": onboard a merchant end-to-end
against the live stack (business→doc upload→bank add+verify→contract accept) and confirm the lifecycle advances.

## Notes / log
- Reuses the work11 create-modal/stepper patterns. Bank-detail confidentiality is a F.2 hard line.
