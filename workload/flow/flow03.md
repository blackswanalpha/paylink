# flow03 — proof-validator (execution recipe · seeded skeleton)

**Work item:** [work03](../work/work03.md) · **Goal recap:** verify a normalized proof and
broadcast settlement to the lVM RPC.

## Pre-flight
- [ ] Read [work03](../work/work03.md), [rules.md](../rules.md). Confirm work02 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)

| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Map crypto verify/hash, the settlement tx + anti-replay path, and the RPC broadcast method | **Explore** (`internal/crypto`, `internal/chain`, `internal/rpc`) |
| 2 | Design verify→broadcast flow + reject semantics | **Plan** |
| 3 | Scaffold the Go/chi skeleton (mirror work02) | `/scaffold-service` |
| 4 | Implement proof verification (shape + signature) | **service-builder** |
| 5 | Implement settlement-tx broadcast via lVM JSON-RPC | **service-builder** |
| 6 | Tests incl. tampered-proof + already-settled cases; ≥80% | **service-builder** |
| 7 | Review vs invariants (A.3/A.4/A.7/A.1) + quality | **invariant-auditor** + `/code-review` |
| 8 | Verify: valid proof settles, tampered rejected | `/verify` |

## Done
- [ ] [work03](../work/work03.md) criteria met; Backend-service DoD complete; mark `done` in [backlog.md](../backlog.md).
