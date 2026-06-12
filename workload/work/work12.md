# work12 — compliance-risk (basic KYC + risk scoring)

> **Seeded** — expand with `/work 12` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 09 · **Flow:** [flow12](../flow/flow12.md)
- **Phase:** 1 / MVP (basic KYC) · **Spec:** backendfeatures.md §2.6 (the "§2.15" seed was a stale ref — §2.15 is fraud-detection)

## Goal
KYC orchestration and transaction risk scoring. Phase 1: basic KYC tiers + a `/v1/risk/evaluate`
decision (allow/block/review) for above-threshold actions. Full sanctions/KYB/multi-jurisdiction is Phase 2.

## In scope (Phase 1)
- `/v1/kyc/sessions`, `/v1/kyc/callbacks/{provider}` (HMAC), `/v1/compliance/status`, `/v1/risk/evaluate` (internal mTLS).
- KYC tiers 0..2; basic risk inputs (velocity, amount-vs-tier, geo); Kenya AML threshold (KES 150,000 cumulative).
- Owns `compliance` schema; publishes `compliance.kyc.*`, `compliance.check.*`, `compliance.flag.raised`.

## Out of scope (Phase 2+)
- Sanctions screening (OFAC/UN/EU), KYB, per-jurisdiction rule sets, ML anomaly scoring.

## Invariants that apply
- Non-custodial; PII via KMS; a `compliance.check.failed` must be able to block a PayLink (consumed by work01).

## Reuse first
- work01 Python/FastAPI layout; work09 user records; event bus (work15); standard error envelope.

## Acceptance criteria
- [x] KYC session create + provider callback updates `kyc_tier`.
- [x] `/v1/risk/evaluate` returns {decision, score, reasons}; above-threshold blocks correctly.
- [x] Compliance events published; tests ≥80%; lint/build clean.
- [x] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
