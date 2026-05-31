# work10 — merchant-onboarding (verification, bank linking, contracts)

> **Seeded** — expand with `/work 10` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 09 · **Flow:** [flow10](../flow/flow10.md)
- **Phase:** 1 / MVP (Kenya manual) · **Spec:** backendfeatures.md §2.3

## Goal
Onboard merchants: business verification, document upload, bank-account linking + verification,
contract acceptance, and fee-tier assignment, with the DRAFT→PENDING_VERIFICATION→ACTIVE state machine.

## In scope
- `/v1/merchants/onboard`, `/documents`, `/bank-accounts(+/verify)`, `/contracts`, `/fee-tier`, `GET /merchants/{id}`.
- State machine DRAFT → PENDING_VERIFICATION → ACTIVE | REJECTED | SUSPENDED.
- Owns the `merchant` schema (KMS-encrypted bank refs); publishes `merchant.*`; consumes `compliance.kyb.*`, `admin.override.*`.
- Phase 1: Kenya, manual verification path.

## Out of scope
- Self-serve onboarding for low-risk merchants (Phase 3).
- Fee computation (work21 fee-pricing consumes the tier).
- KYB verification engine (work12 / Phase 2 sanctions).

## Invariants that apply
- Non-custodial; bank account references encrypted (KMS), never plaintext; secrets via env/KMS.

## Reuse first
- work01 Python/FastAPI layout; work09 auth/RBAC; S3 for documents; event bus (work15).

## Acceptance criteria
- [ ] Onboard → upload docs → add+verify bank account → accept contract → ACTIVE.
- [ ] Bank refs stored KMS-encrypted; state transitions enforced; `merchant.*` events emitted.
- [ ] Tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
