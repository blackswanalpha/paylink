# work32 — SDK suite (Python / Go / Java / Flutter)

> **Seeded** — expand with `/work 32` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python / Go / Java / Dart · **Depends on:** 06 · **Flow:** [flow32](../flow/flow32.md)
- **Phase:** 3 / Mainnet · **Spec:** CLAUDE.md SDKs + backendfeatures.md Phase 3

## Goal
Round out the SDK family beyond JS (work06): typed clients for the `/v1` API in Python, Go,
Java, and Flutter/Dart, each mirroring the JS SDK's surface and the standard error envelope.

## In scope
- One typed client per language under `sdks/{python,go,java,flutter}`, covering paylinks + payments (+ auth pass-through).
- Maps the standard error envelope to idiomatic typed errors per language.
- Per-language tests against a mock or the local stack.

## Out of scope
- New API surface (SDKs follow the existing `/v1` endpoints; if an endpoint is missing, that's a service work item).

## Invariants that apply
- Rail-agnostic (no rail-specific fields exposed); parity with the JS SDK contract.

## Reuse first
- The JS SDK (work06) as the reference surface; the OpenAPI spec in `docs/api/`.

## Acceptance criteria
- [ ] Each SDK covers the in-scope `/v1` endpoints with idiomatic typed clients + error mapping.
- [ ] Per-language tests pass; surfaces stay in parity with the JS SDK.
- [ ] Passes the SDK checklist in [definition-of-done.md](../definition-of-done.md) per language.

## Verification
[verification.md](../verification.md) → "SDK": run each SDK's tests; exercise create→read→settle
against the local stack.
