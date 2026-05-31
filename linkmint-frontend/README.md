# LinkMint frontend (`linkmint-frontend`)

The user-facing demo for the PayLink flow — **create a PayLink → pay via M-PESA → watch it settle
on-chain** — built for **work07**. It talks to the backend **only through the JS SDK**
(`@linkmint/sdk`, work06); there is no raw `fetch` to the LinkMint API anywhere.

> **Location note:** the work07 contract (and `workload/*`, `verification.md`) refer to `apps/web`.
> Per the project owner this app lives at the repo-root **`linkmint-frontend/`** instead. The App
> definition-of-done still applies — just at this path.

## Stack

- **Next.js 16** (App Router) + **React 19**, **TypeScript strict** (no `any`).
- **Chakra UI v3** (components) · **Zustand** (wizard + client store) · **Sonner** (toasts) ·
  **Feather icons** (`react-feather`).
- **`@linkmint/sdk`** as a local `file:` dependency — the single backend surface.

## How it works

- **Auth (dev JWT):** a Next **server component** (`src/app/page.tsx`) mints a short-lived HS256
  JWT (`src/lib/jwt.ts`, `node:crypto`, zero-dep) using server-only env, and passes only the
  **token** to the client. The signing secret never reaches the browser. The SDK sends it as a
  bearer token; the gateway derives `X-Creator-Addr` from the `creator_addr` claim.
- **CORS:** `next.config.mjs` rewrites the browser's same-origin `/v1/*` to the gateway, so the SDK
  never makes a cross-origin request.
- **Settlement:** `useSettlementStatus` polls `paylinks.get` (the on-chain source of truth) until a
  terminal status. See the work35 note below.

## Prerequisites

1. **Build the SDK once** (this app's `file:` dep resolves to its `dist/`, which is git-ignored):
   ```bash
   cd ../sdks/javascript && npm install && npm run build
   ```
2. **Bring up the local stack.** Use the `e2e` profile to actually reach `VERIFIED` (it adds the
   chain + MPesa adapter + proof-validator); the default profile only reaches `PENDING`:
   ```bash
   cd .. && docker compose --profile e2e up -d
   ```

## Run

```bash
cp .env.example .env.local      # dev defaults already match docker-compose.yml
npm install
npm run dev                     # http://localhost:3000  (/v1 proxied to the gateway on :8088)
```

> **Opening from another device (LAN):** use the **Network** URL Next prints (e.g.
> `http://<your-lan-ip>:3000`). Next 16 blocks cross-origin dev resources by default, which breaks
> hydration over a LAN IP — `next.config.mjs` sets `allowedDevOrigins` to this machine's LAN IPv4
> addresses to allow it. On the host machine itself, just use `http://localhost:3000`.

Quality gates (App DoD):

```bash
npm run lint                    # eslint (no `any`) + prettier --check
npm run typecheck               # tsc --noEmit (strict)
npm test                        # vitest (component/unit)
```

## The flow

1. **Create** — receiver `0x1111…1111`, amount `1000`, currency `KES`. Returns a PayLink (status
   `PENDING` on the integrated stack).
2. **Instructions** — M-PESA Pay Bill / Account (= the PayLink id) / Amount. The app also
   best-effort records a payment intent (`payments.initiate`, rail `mpesa`).
3. **Settlement** — pay out-of-band (real M-PESA, or the e2e Daraja stub). The status flips
   `PENDING → VERIFIED` with `vote_count` and `chain_tx_hash`.

The app **never collects or holds funds** (non-custodial, invariant A.1) — it only shows where to
pay in M-PESA. PayLink/Payment types stay rail-agnostic (A.4); the only rail reference is the
opaque `mpesa` routing label.

## Known limitation (work35)

On the integrated stack a freshly created PayLink is `PENDING`, but the orchestrator's
`payments.initiate` only accepts `CREATED` — so it returns `409 PAYLINK_NOT_PAYABLE`. This is the
open **work35** bug and is **not** fixed here. The app handles it gracefully: it shows a neutral
labeled note and keeps the PayLink poll as the settlement source of truth (which reaches
`VERIFIED` correctly via the adapter path).

## Environment

See `.env.example`. Server-only vars (`LINKMINT_JWT_*`, `LINKMINT_GATEWAY_URL`) are never shipped to
the browser; client vars are `NEXT_PUBLIC_*`. The demo API key/JWT secret are local-only — never
ship a real secret in a `NEXT_PUBLIC_` var.
