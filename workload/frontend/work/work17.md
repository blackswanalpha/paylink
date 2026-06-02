# work17 — Developer Portal (keys / quickstart / docs)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 08, 09 · backend [work09](../../work/work09.md)
- **Flow:** [flow17](../flow/flow17.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.5 (Developer Portal) / `fe08`

## Goal
A self-serve developer surface: API-key management, an SDK quickstart, and a docs/OpenAPI explorer —
so a developer can get from sign-up to a first successful API call.

## Why / context
The Platform Developer persona (PRD) evaluates on DX. identity-service provides scoped API keys; the
JS SDK exists. This packages them into a portal — reusing the account API-keys component.

## In scope
- `/developers`: **API keys** (reuse the work10 component — issue/list/revoke, scopes), an **SDK
  quickstart** (`npm i @linkmint/sdk` + a create-PayLink snippet with the user's key), and a **docs**
  area.
- **Docs/OpenAPI explorer**, **sandbox**, and **webhook tester** are marked **PLANNED** (depend on the
  work05 gateway OpenAPI-aggregation follow-up + Phase-2 webhooks), per F.7.

## Out of scope (do NOT do here)
- New backend APIs. Real webhook delivery (backend work14/Phase 2). The marketing site.

## Invariants that apply
- **F.1 SDK-only**, **F.4** (keys via identity, never client-spoofed), **F.6**, **F.7** (docs/sandbox marked PLANNED).

## Reuse first
- The API-keys component from work10; `client.users` api-key methods (work08); code-snippet rendering;
  `CopyField` (work03); the SDK README/quickstart in `../../../sdks/javascript/`.

## Acceptance criteria
- [ ] A developer can issue/list/revoke a scoped API key and copy a working quickstart snippet.
- [ ] The quickstart references the real SDK + the user's key prefix; docs/sandbox/webhook-tester marked PLANNED.
- [ ] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": issue a key, copy the quickstart,
and (manually) make a first call with it against the live gateway.

## Notes / log
- Shares the API-keys component with work10. Docs explorer waits on the work05 OpenAPI-aggregation follow-up.
