# flow34 — Token send & payment submission (execution recipe · seeded skeleton)

**Work item:** [work34](../work/work34.md) · **Goal recap:** payer-side build → sign (client) →
broadcast, non-custodially, so tokens move to pay.

## Pre-flight
- [ ] Read [work34](../work/work34.md), [rules.md](../rules.md) (A.1/A.7/A.2). Confirm work24 + work06 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study `TxTransfer` + `SignableBytes()` (`internal/types`), ECDSA signing (`internal/crypto`), and `paylink_getNonce`/`sendTransaction`/`getTransactionReceipt` | **Explore** |
| 2 | Design build (unsigned) + client signer + relay submit; pin a shared sign/verify test vector | **Plan** |
| 3 | wallet-service: `POST /v1/transactions` (build unsigned, nonce, fee) | **service-builder** (Go) |
| 4 | wallet-service: `POST /v1/transactions/submit` (broadcast pre-signed; reject key material) | **service-builder** (Go) |
| 5 | JS SDK: client-side ECDSA signer matching `SignableBytes()` byte-for-byte + build/sign/submit helpers | **service-builder** (TS) |
| 6 | Tests: Go↔TS signature parity vector, build, submit, stale-nonce rejection; ≥80% | **service-builder** |
| 7 | Review **A.1 (no key custody)** + **A.7 (nonce/replay)** + `/security-review` | **invariant-auditor** + `/security-review` |
| 8 | Verify build→sign→submit moves tokens on-chain (receipt + balance) | `/verify` |

## Done
- [ ] [work34](../work/work34.md) criteria met; Backend-service + SDK DoD complete; mark `done` in [backlog.md](../backlog.md).
