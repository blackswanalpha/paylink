# work25 — fraud-detection-service (real-time scoring, block path)

> **Seeded** — expand with `/work 25` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python/FastAPI (+ XGBoost/Redis) · **Depends on:** 02, 12 · **Flow:** [flow25](../flow/flow25.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.16

## Goal
Real-time behavioral fraud scoring that can block a payment before it hits the rail — a hybrid
rules + ML (XGBoost) model returning allow/review/block.

## In scope
- `/v1/fraud/evaluate` (mTLS), `/feedback`, `/decisions/{id}` (admin), `/rules` (admin).
- Hybrid scoring: explicit rules (velocity, amount σ, geo/IP mismatch) + XGBoost; per-merchant thresholds.
- Owns `fraud` schema (decisions, feedback, device fingerprints, rules); consumes outcomes for training labels.
- p95 evaluate < 80ms target.

## Out of scope
- Nightly model retraining infra depth + adversarial-pattern auto-detection (Phase 3 hardening).

## Invariants that apply
- Non-custodial; PII minimization; a `block` decision must actually prevent rail initiation (via work02).

## Reuse first
- work01 Python/FastAPI layout; compliance risk inputs (work12); payment context from orchestrator (work02); event bus (work15).

## Acceptance criteria
- [ ] `/v1/fraud/evaluate` returns {decision, score, signals}; block prevents payment initiation.
- [ ] Rules editable + pausable; feedback loop records labels; p95 < 80ms on the eval path.
- [ ] Tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
