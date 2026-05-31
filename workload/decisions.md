# Decisions — lightweight ADR log

Architecture Decision Records. One entry per non-obvious choice. Keep them short:
context, decision, consequences. New ADRs append; superseded ones are marked, not deleted.

Format:
```
## ADR-NNN — <title>
- Status: Proposed | Accepted | Superseded by ADR-MMM
- Date: YYYY-MM-DD
- Context: why this came up
- Decision: what we chose
- Consequences: what follows from it
```

---

## ADR-001 — Reconcile the phase-numbering discrepancy
- **Status:** Proposed (needs project-owner confirmation)
- **Date:** 2026-05-28
- **Context:** `CLAUDE.md` states the current phase is "Phase 2 (2026-Q2) — multi-validator
  VRF, fee model, P2P networking." `system.md` instead describes **Phase 1 (MVP, 2026-Q2)**
  as single-validator + MPesa + core services + basic web UI, and **Phase 2 (Beta, 2026-Q3)**
  as multi-validator + multi-rail. The two docs disagree on what "Phase 2" means and when.
- **Decision:** For planning the seeded backlog, treat the **current milestone as the MVP**
  (single validator — already built — plus core services, MPesa adapter, JS SDK, basic web
  flow, local docker-compose + CI). The chain already implements multi-validator VRF/fee/P2P,
  so those are "built, not yet operationally rolled out." Defer rollout/expansion.
- **Consequences:** `scope.md` and the backlog target MVP deliverables. The numbering label
  itself is unresolved — confirm with the owner and update `CLAUDE.md`/`system.md` to match.

## ADR-002 — Introduce the `workload/` process layer + functional `.claude/`
- **Status:** Accepted
- **Date:** 2026-05-28
- **Context:** The chain is mature but the entire application layer is unbuilt and there was
  no repeatable process for building it with Claude, nor any executable Claude Code config.
- **Decision:** Add `workload/` as a markdown process layer (standards, rules, scope, paired
  work/flow items, prompt library, DoD, verification) and a functional `.claude/` (subagents,
  project skills, slash commands, permission allowlist). `CLAUDE.md` and the design docs
  remain the source of truth; `workload/` only operationalizes them to avoid fact drift.
- **Consequences:** Work is driven via `/work <nn>`; new work via `/new-work`; invariants are
  audited via `/check-invariants`. The set is additive — no existing code changes.

## ADR-003 — Backend service stack: Python/FastAPI + Go/chi (per backendfeatures.md)
- **Status:** Accepted (resolves a CLAUDE.md ↔ backendfeatures.md conflict)
- **Date:** 2026-05-28
- **Context:** `CLAUDE.md` says backend services are "TypeScript/Node.js (Go for
  performance-critical paths)". `backendfeatures.md` — the far more detailed spec — assigns a
  concrete per-service split: **Python/FastAPI** for most services and **Go/chi** for hot
  paths. The two disagree. The initial seeded work items (work01–05) were written assuming
  TypeScript, which conflicts with the detailed spec.
- **Decision:** Follow `backendfeatures.md`. Backend services use **Python/FastAPI**, except the
  performance/throughput-sensitive ones which use **Go/chi**: payment-orchestrator,
  proof-validator, escrow-manager, settlement-service, wallet-service, audit-log-service,
  reconciliation-service, and the adapters framework. **TypeScript is used only for the JS SDK
  and the web app.** The lVM stays Go (unchanged). Updated `standard.md`, the `service-builder`
  agent, the `scaffold-service` skill, and work01–05 to match.
- **Consequences:** `CLAUDE.md`'s tech-stack line should be updated by the owner to reflect this
  split (or this ADR overridden). `standard.md` now carries Python/FastAPI conventions alongside
  Go and TS.

## ADR-004 — Event bus transport: Kafka/SQS, with backendfeatures.md as the logical event model
- **Status:** Accepted (resolves a CLAUDE.md ↔ backendfeatures.md conflict)
- **Date:** 2026-05-28
- **Context:** `CLAUDE.md` lists "Kafka or AWS SQS" for messaging. `backendfeatures.md` specifies
  **NATS JetStream** (18 domain streams, 70+ subjects) and a `chain-event-mirror` sidecar.
- **Decision:** Keep **Kafka/SQS** as the transport (per `CLAUDE.md`). Adopt
  `backendfeatures.md`'s **event catalog as the logical domain-event model** — the stream/subject
  taxonomy (e.g. `payment.proof_received`, `chain.paylink.verified`) maps onto Kafka topics / SQS
  queues. NATS JetStream is recorded as a later proposal, not adopted now.
- **Consequences:** Work items reference domain events by their logical name; the transport is
  Kafka/SQS. If the owner later prefers NATS, only the transport layer changes, not the event
  taxonomy. Revisit if exactly-once / stream-replay needs make NATS JetStream compelling.

## ADR-005 — Chain hardening (blockchainfeature.md) tracked separately, not in this backlog
- **Status:** Accepted (with a flagged risk)
- **Date:** 2026-05-28
- **Context:** `blockchainfeature.md` is a chain-hardening roadmap (~110 items). A cross-check
  found the chain implements ~41%, but several **P0 consensus items are MISSING** —
  transaction-signature verification, block-signature verification, VRF gating in the producer,
  quorum enforcement, and fork choice — flagged in the doc as *"must-fix before any multi-node
  deployment."*
- **Decision:** This `workload/` backlog covers the **application layer (backendfeatures.md)**
  only; chain work is out of its scope ([scope.md](scope.md)). Chain hardening is tracked
  separately.
- **Consequences:** ⚠️ The P0 consensus gaps are a real blocker for the Phase 2 multi-validator
  milestone and for proof-validator's quorum path (work03). Recommend a parallel chain-hardening
  backlog before multi-node rollout. Not created here because it wasn't selected.

## ADR-006 — paylink-service holds a P-256 key to submit value-less PayLink txs
- **Status:** Accepted (work01)
- **Date:** 2026-05-29
- **Context:** work01's `POST /v1/paylinks` and `/cancel` submit `TxCreatePayLink` / `TxCancelPayLink`
  to the lVM (system.md: "mints PayLink on-chain upon creation"). The lVM sets `Creator/Owner =
  tx.From` (the signer), and there is no SDK/client-signing yet (deferred to work05). Something must
  sign these txs.
- **Decision:** For the MVP, `paylink-service` holds its own NIST P-256 key (`app/chain/signer.py`,
  env `PAYLINK_CHAIN_SIGNER_KEY`) and signs create/cancel on behalf of the authenticated caller. The
  **on-chain** `from` is therefore the *service* address, while the **off-chain** `creator_addr` is
  the caller (the `X-Creator-Addr` gateway header; work05). The service reconciles a PayLink to its
  off-chain record by `pl_id`, not by on-chain creator, so this split is transparent to reads.
- **Why this does NOT break A.1 (non-custodial):** create/cancel move **no value** — they record /
  void a payment authorization. Only a `metadataHash` ever goes on-chain. The key is a
  PayLink-lifecycle key, not a custody/settlement/fund-moving credential.
- **Consequences:** (1) On-chain creator/owner is the service until work05 introduces client-signed
  txs (then the signer seam is swapped, no API change). (2) This is currently indistinguishable from
  spoofing **only because the chain does not yet verify tx signatures** (ADR-005) — when sig
  verification lands, the service key becomes the enforced on-chain identity, which is the intended
  end state for service-submitted txs. (3) **Hardening for work05:** the gateway MUST make
  `X-Creator-Addr` mandatory in non-dev environments; today `app/deps.py` falls back to the service
  address when the header is absent (dev ergonomics only). A config flag should disable that fallback
  in prod. Revisit at work05.

## ADR-007 — MPesa adapter is a hybrid: Go core + Node.js Daraja rail SDK
- **Status:** Accepted (work04)
- **Date:** 2026-05-29
- **Context:** `standard.md`/ADR-003 put the adapters framework on **Go/chi**. During work04 the
  project owner asked to use **Node.js for the Daraja rail SDK** specifically. The hard constraint:
  the proof's `proof_signature` must be a byte-exact P-256 signature over canonical bytes the
  proof-validator already trusts — effortless in Go via `paylink-chain/pkg/lvm`, but a re-derivation
  risk if reimplemented in JS.
- **Decision:** Split the adapter at the rail boundary. A **Go/chi core** (`adapters/mpesa/`) keeps
  the protocol-critical path — normalize → **sign** (reusing `pkg/lvm`, byte-exact) → broadcast — plus
  `/v1/charges`, `/v1/callbacks/mpesa`, the Redis correlation store, and idempotency. A **Node.js rail
  service** (`adapters/mpesa/daraja-service/`, plain Node, built-ins only) owns everything MPesa: OAuth,
  STK push, and parsing the raw Daraja callback; it hands the core only **rail-neutral** fields over a
  token-authed internal hop. The A.4 boundary is the Node→core handoff. Orchestrator registration is
  **config-only** (`PAYMENT_ADAPTER_MPESA_URL`, logged at boot; the orchestrator does not call the
  adapter — keeps work02, already `done`, untouched and the rail label opaque). Anti-replay (A.7) is
  not duplicated in the adapter: it broadcasts with a deterministic `Idempotency-Key` (`mpesa:<tx_id>`)
  and defers to the validator + on-chain proof-hash check.
- **Consequences:** Two deployables per rail (a Go core + a Node rail SDK), wired in docker-compose.
  Signing/anti-replay stay in the proven Go path; only the rail wire-format lives in Node, so future
  rails can follow the same shape. The `/scaffold-adapter` skill (still showing a single-service TS
  layout) is now doubly stale — update it to "Go core + optional Node rail SDK" (filed as a follow-up).
  Per-merchant Daraja credentials/shortcodes and a Safaricom IP allowlist + split tokens are deferred.

## ADR-008 — api-gateway is Kong (DB-less declarative), amending ADR-003
- **Status:** Accepted (work05) — amends ADR-003 (the api-gateway row only)
- **Date:** 2026-05-29
- **Context:** work05 needs one authenticated ingress: route `/v1/paylinks*` → paylink-service and
  `/v1/payments*` → payment-orchestrator; validate JWT (OAuth2) + partner API keys, rejecting with
  the standard error envelope; propagate the `X-Request-Id` correlation id; rate-limit; and, per
  ADR-006, inject a trustworthy `X-Creator-Addr` while stripping any client-supplied one. The work05
  spec allowed "Kong config **or** a thin custom FastAPI gateway (decide in step 2; record as an
  ADR)". ADR-003 had tentatively listed api-gateway under Python/FastAPI. **The project owner chose
  Kong.**
- **Decision:** Build a **Kong Gateway OSS 3.7** ingress in **DB-less mode** (`KONG_DATABASE=off`)
  at `linkmint-backend/api-gateway`, driven by a declarative config (`kong/kong.yml.tmpl`) rendered
  from the environment at container start via `envsubst` (12-factor; the rendered `kong.yml` is
  git-ignored and is the only place a secret materializes). Bundled OSS plugins do routing
  (`services`/`routes`, `strip_path:false`), auth (`jwt` + `key-auth`, each with `anonymous`
  fallback so a request passes on EITHER credential), `rate-limiting` (redis policy, reusing the
  shared Redis), `correlation-id` (`X-Request-Id`), `prometheus` (metrics on the status listener),
  and a catch-all `request-termination` (404). A single global **serverless `post-function`** covers
  the two things no OSS plugin expresses declaratively: in `access` it strips inbound
  `X-Creator-Addr`/`X-Partner-Id`, requires a credential (rejecting the anonymous consumer with
  401), and injects `X-Creator-Addr` from the verified JWT claim (or the key-auth consumer's
  `custom_id`); in `header_filter`/`body_filter` it rewrites every ≥400 response into the LinkMint
  envelope `{"error":{code,message,details,trace_id}}`, **passing an upstream's own envelope through
  unchanged**. The serverless code runs in Kong's Lua **sandbox** with `cjson` allow-listed
  (`KONG_UNTRUSTED_LUA_SANDBOX_REQUIRES=cjson,cjson.safe`). JWT validation is a **config-only seam**:
  HS256 dev secret now, an RS256 registered public key for identity-service (work09) later — no IdP
  built here. Per the owner, the gateway is authoritative for `X-Creator-Addr` at the edge but the
  paired paylink-service `PAYLINK_REQUIRE_CREATOR_ADDR` enforcement flag is **deferred** to a new
  backlog item. Kong's admin API is bound to `127.0.0.1` and never exposed.
- **Consequences:** (1) No Python deployable for the gateway; the bespoke logic is ~one small Lua
  block, verified by an integration matrix (routing / 401·403·404·429·502·504 envelopes /
  X-Creator-Addr inject+strip / credential hygiene / correlation / rate-limit) on an isolated
  compose stack (gateway + Redis + echo upstreams) — so work05 closes against the **Infra/CI**
  definition-of-done, not the per-language ≥80%-coverage line (there is no app code to cover). (2)
  Swapping to identity-service is config-only (register its RS256 key / set
  `GATEWAY_JWT_ALGORITHM=RS256`); dynamic JWKS is a follow-up (OSS `jwt` validates against a
  registered key, not a live JWKS). (3) Serverless functions require the sandbox `cjson` allow-list;
  if a future plugin needs more, extend that list rather than disabling the sandbox. (4) Partner API
  keys are a single declarative credential for the MVP; a real rotatable key store is a follow-up.
  (5) payment-orchestrator currently reads no caller header — binding it to the injected
  `X-Creator-Addr` is a follow-up (not required for work05's routing+auth criteria). (6) If a future
  phase needs WAF / declarative quotas / a dev portal, Kong Enterprise (`openid-connect`,
  `exit-transformer`) drops in without changing the `/v1` contract or the envelope. (7)
  `standard.md`'s ADR-003 api-gateway row is annotated to point here.
