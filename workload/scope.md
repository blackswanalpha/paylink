# Scope — boundaries to prevent scope creep

The purpose of this doc is to make "no, not now" cheap and unambiguous. Before starting
work, confirm it's inside the active work item's fence **and** inside the current phase.
Anything else becomes a new backlog item, not an expansion of the current one.

---

## The hard boundary (never crosses, any phase)

The **non-custodial invariant** ([`rules.md`](rules.md) A.1) is the outermost scope fence.
No feature, shortcut, or "temporary" workaround may make LinkMint hold or move user funds.
If a request needs custody, it is out of scope for LinkMint entirely — not deferred.

---

## Current phase

> **Known discrepancy (see ADR-001 in [`decisions.md`](decisions.md)):** `CLAUDE.md`
> labels the current phase "Phase 2 (2026-Q2)", while `system.md` describes Phase 1 (MVP,
> 2026-Q2) as the single-validator + MPesa milestone and Phase 2 (Beta, 2026-Q3) as
> multi-validator + multi-rail. The seeded backlog targets the **MVP deliverables**
> (single validator already built, MPesa-first, core services, basic web UI) regardless
> of the label. Confirm the canonical numbering with the project owner.

**The backlog now covers the full application layer** described in `backendfeatures.md` — all
20 services + cross-cutting infrastructure — **phase-tagged** so scope is bounded by *phase*,
not by omission. "In scope" means "in scope **for its phase**". Work the current phase; don't
pull Phase 2/3 items forward without a reason.

- **Phase 1 (MVP):** api-gateway, identity, merchant-onboarding, admin-backoffice (read-only),
  paylink-service, payment-orchestrator (MPesa), proof-validator, compliance-risk (basic KYC),
  audit-log, notification (SMS/email); cross-cutting: event bus, double-entry ledger,
  idempotency, observability; local docker-compose + CI.
- **Phase 2 (Beta):** invoice-subscription, escrow-manager, fee-pricing, refund-dispute,
  settlement, wallet, fraud-detection, reporting-analytics, reconciliation; card + crypto
  adapters; JS SDK + web app; admin mutations (dual-approval); compliance (sanctions/KYB).
- **Phase 3 (Mainnet):** bank adapter, subscriptions, full SDK suite (Python/Go/Java/Flutter),
  dashboards (merchant/admin/mobile), enterprise SSO, instant payouts, multi-currency,
  governance, and the rest of the `backendfeatures.md` Phase-3 list.

See [backlog.md](backlog.md) for the per-item phase, stack, and dependencies.

**Out of scope for this backlog entirely (tracked elsewhere):**
- **Chain hardening (`blockchainfeature.md`).** The lVM's P0 consensus gaps (tx-signature
  verification, block-signature verification, VRF gating, quorum enforcement, fork choice) and
  the rest of that roadmap are **not** in this backlog — see ADR-005. ⚠️ They block the Phase 2
  multi-validator milestone and proof-validator's quorum path; track them in a parallel
  chain-hardening backlog.
- Production infra depth (Terraform, Helm, multi-AZ K8s) beyond local docker-compose — Phase 3.

---

## Per-item scope fences

Each `work/workNN.md` carries its own **In scope / Out of scope** section. Treat them as
binding. Typical creep traps to refuse:

- "While I'm here, also add the card adapter" → new item (deferred rail).
- "Let's also stand up Kubernetes" → out of phase (local docker-compose only for now).
- "Add KYC checks to the PayLink create path" → Compliance service is deferred; don't
  inline it.
- "Make the SDK support every language" → JS only in this phase.

---

## How to handle discovered work

1. If it blocks the current item and is tiny, do it and note it in the work item's log.
2. If it's a separate concern, add a new entry to [`backlog.md`](backlog.md) and keep going.
3. If it touches a [`rules.md`](rules.md) invariant, stop and write/seek an ADR first.
