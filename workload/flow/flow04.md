# flow04 — MPesa adapter (execution recipe · seeded skeleton)

**Work item:** [work04](../work/work04.md) · **Goal recap:** Daraja callback → normalized proof
→ sign → broadcast to proof-validator.

## Pre-flight
- [x] Read [work04](../work/work04.md), [rules.md](../rules.md). Confirm work03 `done`. Set `in-progress`.
- [x] Daraja credentials are env-only (never committed); devnet e2e uses `DARAJA_STUB=true` (no creds).

## Steps (skeleton — refine on start)

| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Confirm Daraja callback shape + auth; map fields to the proof shape | **Explore** + `/deep-research` (Daraja specifics) |
| 2 | Design receive→normalize→sign→broadcast pipeline | **Plan** |
| 3 | Scaffold the adapter | `/scaffold-adapter` |
| 4 | Implement Daraja OAuth + STK/C2B callback handling | **service-builder** |
| 5 | Normalize to the rail-agnostic proof + sign (reuse `internal/crypto`) | **service-builder** |
| 6 | Broadcast to proof-validator; register in orchestrator config | **service-builder** |
| 7 | Tests with captured callbacks; lint/build | **service-builder** |
| 8 | Review vs A.1/A.4 + `/security-review` (handles money + secrets) | **invariant-auditor** + `/security-review` |
| 9 | Verify end-to-end settlement | `/verify` |

> **Deviation (ADR-007):** steps 4–5's Daraja integration was built as a separate **Node.js rail
> service** (`adapters/mpesa/daraja-service/`) per the user's request, while normalize/sign/broadcast
> (steps 5–6) stayed in the Go core so the proof signature is byte-exact with the validator. `/verify`
> (step 9) was run live via `docker compose --profile e2e` (DARAJA_STUB) → on-chain settlement.

## Done
- [x] [work04](../work/work04.md) criteria met; Adapter DoD complete; marked `done` in [backlog.md](../backlog.md).
