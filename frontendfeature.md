# LinkMint Frontend Feature Specification

Scope: the **client surfaces** that sit in front of the off-chain fleet — the web app
(`linkmint-frontend/`), its design system, and every screen each persona touches. For the services
those screens call see [`backendfeatures.md`](backendfeatures.md); for the chain layer see
[`blockchainfeature.md`](blockchainfeature.md); for product context and personas see [`prd.md`](prd.md);
for protocol design see [`system.md`](system.md). The typed client every screen goes through is
[`@linkmint/sdk`](sdks/javascript/).

This document is the contract the frontend is built against. Each surface section lists screens, the
components it uses, **its data source (SDK method + the backend `§`/work item behind it)**, its
states, a **phase tag**, and acceptance criteria — so a Phase-1 reader can ignore everything tagged
PLANNED.

**Status legend.** Each feature is tagged:
- **LIVE** — backed by a completed backend work item (work01–work14) and buildable today.
- **PLANNED** — depends on a backend work item not yet shipped (Phase 2/3). Specified here for a
  coherent blueprint; the UI must mark it as unavailable, never fake it as working (Invariant F.7).

> **Build state (2026-06):** work01–work14 are done (paylink-service, payment-orchestrator,
> proof-validator, MPesa adapter, Kong gateway, JS SDK, identity, merchant-onboarding,
> admin-backoffice, compliance-risk, audit-log; notification work14 is `todo`). The frontend baseline
> is **work07** — a 3-step create→pay→settle wizard. This spec defines the premium product UI built on
> that baseline; the flagship Merchant Dashboard is the first screen shipped against it.

---

## Invariants

Every screen and component must respect these rules without exception. They are the frontend
projection of the protocol invariants in `backendfeatures.md` + `CLAUDE.md`.

1. **F.1 SDK-only.** All LinkMint API access goes through `@linkmint/sdk` — never raw `fetch` to a
   `/v1` path. The single client is built in `src/lib/linkmint.ts`. (App DoD,
   [`workload/definition-of-done.md`](workload/definition-of-done.md).)
2. **F.2 Non-custodial UI.** The frontend collects, holds, and moves **no funds**. It never captures a
   card PAN/CVV or an MPesa PIN — rail credential entry happens rail-side (STK push on the payer's
   handset, or a rail's hosted fields). Payer flows only *initiate*, *show instructions*, and *poll
   status*. (Protocol invariant A.1.)
3. **F.3 Rail-agnostic presentation.** Core PayLink and Payment views carry no rail-specific fields.
   The only rail reference in the type surface is the opaque `PaymentRail` label (`mpesa | card | bank
   | crypto`), chosen at pay time and shown as a label + icon. (Protocol invariant A.4.)
4. **F.4 Edge-authoritative identity.** The browser never sends `X-Creator-Addr` or any identity
   header; the bearer token is the sole credential and the gateway injects identity from its claims
   (ADR-006/008). Tokens are short-lived and server-minted (a Next server component mints them; the
   signing secret never reaches the browser).
5. **F.5 Error-envelope-aware.** Every error is rendered from the SDK's typed error hierarchy
   (`LinkMintApiError` subclasses + connection/timeout), normalized through `src/lib/errors.ts`, with
   the `trace_id` surfaced for support. No raw stack traces, no swallowed failures.
6. **F.6 Accessibility — WCAG 2.1 AA.** Keyboard operable, visible focus rings, ≥4.5:1 text contrast
   on the ivory canvas, `prefers-reduced-motion` honored, semantic landmarks, labelled controls.
7. **F.7 Phase-honest.** Every surface is tagged LIVE/PLANNED. A PLANNED surface is visibly marked
   (badge + disabled/empty state) and never presents fabricated data as real.
8. **F.8 Idempotent mutations.** Every state-mutating call rides the SDK's auto-generated
   `Idempotency-Key`; retries and React Strict-Mode double-mounts never double-execute. (Protocol
   invariant A.7; the SDK is the dedupe author client-side, the chain server-side.)

---

## 1. Frontend Architecture Overview

### Topology

```
        ┌───────────────────────────────────────────────────────────────┐
        │  Browser (one origin) — Next.js 16 App Router                  │
        │                                                                │
        │  Server Components ──mint short-lived JWT (node:crypto)──┐     │
        │     (secret stays server-side)                           │     │
        │                                                          ▼     │
        │  Client Components ── @linkmint/sdk (bearer token) ──► /v1/*   │
        └──────────────────────────────────────────────┬───────────────┘
                                  next.config rewrites  │  (same-origin → no CORS)
                                                         ▼
                                              ┌─────────────────┐
                                              │   api-gateway   │  Kong (work05)
                                              │  auth · rate    │  injects X-Creator-Addr
                                              └────────┬────────┘
                                                       ▼
                            paylink · payments · identity · merchant · admin ·
                            compliance · audit · adapters   (work01–14)
```

The frontend is a **single Next.js app, one origin**. Server Components mint the dev JWT per request;
Client Components build one SDK client (`src/lib/linkmint.ts`) and make every call through it.
`next.config.mjs` rewrites `/v1/*` to the Kong gateway so the SDK stays same-origin (no CORS). This is
the proven work07 pattern — the premium UI extends it, it does not replace it.

### Surface map (the "apps")

| Surface | Persona | Phase | Backed by |
|---|---|---|---|
| **Public Resolve & Pay** | Payer / guest | LIVE | work01/02/04 |
| **Auth & Account** | All users | LIVE | work09 (+work12) |
| **Merchant Dashboard** | Merchant | LIVE core · PLANNED analytics | work01/02/10 (+19/22/23/26) |
| **Admin Console** | Admin / staff | LIVE | work11/13 |
| **Developer Portal** | Developer | LIVE core · PLANNED docs/sandbox | work09 |
| **Wallet & Staking** | Validator / advanced | PLANNED | work24/34 |

### Routing / information architecture

```
/                      Public resolve+pay entry (the work07 wizard lives here today)
/pay/[plId]            Public PayLink resolution + pay (PLANNED route; folds the wizard)
/login  /register      Auth (LIVE — needs SDK identity resource, §4)
/account               Profile · MFA · sessions · API keys · KYC status (LIVE/§4)
/dashboard             Merchant overview ◀── FLAGSHIP (LIVE)
/dashboard/paylinks    PayLink list / create / cancel (LIVE)
/dashboard/payments    Payment list + detail (LIVE)
/dashboard/onboarding  KYB docs · bank · contracts · fee tier (LIVE/§4)
/dashboard/settlements Settlement batches (PLANNED — work23)
/dashboard/analytics   Revenue/conversion (PLANNED — work26)
/admin                 Search + entity drill-down + audit (LIVE/§4 — work11/13)
/developers            API keys · quickstart · docs (LIVE core / PLANNED docs)
```

### State & data

- **Client/session/UI state** → Zustand (`src/store/`). The existing `app.ts` store holds the SDK
  client + wizard nav; new feature stores follow the same shape (one store per surface, no prop
  drilling). Per-call async (loading/error) lives in hooks, not the store — the established pattern.
- **Server data** → the SDK. List/detail via `paylinks.*` / `payments.*` today; identity/merchant/
  admin resources are an SDK-expansion dependency (§4).
- **Freshness** → polling now (`useSettlementStatus` polls `paylinks.get` to terminal status); a
  WebSocket upgrade over the chain `datastream` is the PLANNED path (§5).

### Cross-cutting UI patterns

- **Loading** → skeletons (never bare spinners on content regions); **Empty** → branded `EmptyState`
  with a primary action; **Error** → `ErrorBanner` (envelope-aware) + a Sonner toast for transient
  failures.
- **Optimistic updates** for cancel/edit, reconciled against the next read.
- **Money** → always integer minor units in logic; `formatMinorUnits` (`src/lib/money.ts`) for
  display; locale-aware grouping.
- **Identifiers** → `pl_id`/addresses/tx hashes shown as truncated mono `AddressChip`s with copy.
- **Responsive** → mobile-first; the dashboard shell collapses the sidebar to a drawer < `md`.

### Tech stack

| Concern | Choice | Notes |
|---|---|---|
| Framework | Next.js 16 (App Router) + React 19 | server components mint JWT |
| Language | TypeScript strict, **no `any`** | ESLint + Prettier clean |
| UI kit | **Chakra UI v3** custom system | `createSystem` (Ivory Premium), replaces `defaultSystem` |
| Fonts | **Fraunces** (display) + **Inter** (UI/body) via `next/font` | mono for hashes |
| State | Zustand v5 | one store per surface |
| Toasts | Sonner | restyled to the palette |
| Icons | `react-feather` | one icon set, consistent stroke |
| Data | `@linkmint/sdk` (`file:` dep) | the only backend surface |
| Charts | zero-dep inline SVG (Sparkline) | a charting lib is a PLANNED add for §3.3 analytics |

---

## 2. Design System — "Ivory Premium"

A warm, editorial, light-first system. The feeling is a private-bank statement, not a crypto
dashboard: ivory paper, ink type, a single emerald jewel accent, restrained gold for moments of
celebration (a settlement), generous air, and hairline structure instead of heavy borders.

### 2.1 Brand & voice

- **Tone:** calm, precise, trustworthy. Short verbs ("Create PayLink", "Settled"). Numbers are the
  hero — large, in the display serif's lining figures.
- **Logo lockup:** wordmark in Fraunces; the diamond glyph `◇` as the mark.
- **One-liner (PRD):** *Pay anyone, anywhere, through any rail — with a link.*

### 2.2 Color tokens

Light-first. Dark mode is scaffolded as a semantic-token seam (Phase 2 toggle).

| Token | Value | Use |
|---|---|---|
| `canvas` | `#FAF7F0` | app background (ivory) |
| `surface` | `#FFFFFF` | cards / panels |
| `surface.subtle` | `#F4F0E7` | inset rows, table header |
| `ink` | `#1C1A17` | primary text |
| `ink.muted` | `#6B655C` | secondary text |
| `hairline` | `#E7E1D5` | 1px borders / dividers |
| `emerald.50…900` | accent ramp; **`emerald.600 #0F6E4E`** = primary | buttons, links, focus |
| `emerald.subtle` | `#E6F0EA` | accent-tinted backgrounds |
| `champagne` | `#C8A24B` | gold highlight — settlement success, premium badges |
| **Status** | | semantic — drive `StatusPill` |
| `status.success` | emerald.600 | VERIFIED / SETTLED / VERIFIED-bank / ACTIVE |
| `status.pending` | `#B8860B` (amber-gold) | PENDING / AWAITING_PAYMENT / PENDING_VERIFY |
| `status.neutral` | ink.muted | CREATED / DRAFT |
| `status.danger` | `#B4452F` (terracotta) | FAILED / CANCELLED / REJECTED |
| `status.expired` | `#8A7E6A` (taupe) | EXPIRED / SUSPENDED |

These are declared as Chakra `tokens` + `semanticTokens` in `src/theme/system.ts`, so components read
`bg="canvas"`, `color="ink.muted"`, `borderColor="hairline"`, and `StatusPill` reads
`status.{kind}` — colors are never hardcoded in components (unlike today's `StatusBadge.tsx`).

### 2.3 Typography

| Role | Family | Weight / treatment |
|---|---|---|
| Display / numerals / H1–H2 | **Fraunces** (`--font-display`) | 600, optical size, lining figures |
| Headings H3–H5, UI, body | **Inter** (`--font-body`) | 400/500/600 |
| Code / hashes / addresses | mono stack | 400 |

Type scale (rem): `xs .75 · sm .875 · md 1 · lg 1.125 · xl 1.375 · 2xl 1.75 · 3xl 2.25 · 4xl 3`.
Large monetary figures use Fraunces at `3xl`/`4xl` with tight tracking.

### 2.4 Space, radius, elevation, motion

- **Spacing:** 4px base grid (`1=4px … 6=24px … 10=40px`). Page gutters `6` mobile / `10` desktop.
  Cards pad `6`; sections gap `8`.
- **Radius:** `sm 6 · md 10 · lg 14 · xl 20 · full`. Cards `lg`; pills `full`; inputs `md`.
- **Elevation (soft, layered):** `xs` `0 1px 2px rgba(28,26,23,.05)` · `sm` `0 2px 8px
  rgba(28,26,23,.06)` · `md` `0 8px 24px rgba(28,26,23,.08)` · `lg` `0 20px 48px rgba(28,26,23,.10)`.
  Prefer hairline + `sm` over heavy shadows; reserve `lg` for modals/popovers.
- **Motion:** durations `fast 120ms · base 200ms · slow 320ms`; easing
  `cubic-bezier(.2,.8,.2,1)`. Hover lifts a card by `-2px` + `sm→md`. All motion is gated on
  `prefers-reduced-motion` (no transform/opacity transitions when reduced).

### 2.5 Component library

Components live in `src/components/ui/` and are themed via the system (recipes where Chakra v3
supports them). Each is responsive and AA-accessible.

| Component | Spec |
|---|---|
| `Button` | variants `solid` (emerald), `outline` (hairline), `ghost`, `gold` (champagne, for celebratory CTAs); `loading`+`loadingText`; icon slot |
| `Field`/`Input` | hairline border, `md` radius, emerald focus ring, helper + error text from validation, mono variant for addresses |
| `Card`/`Panel` | white surface, hairline, `sm` shadow, `lg` radius, optional header/footer slots |
| `PageHeader` | Fraunces title + subtitle + actions row |
| `StatusPill` | reads `status.{kind}` semantic token from a status→kind map; subtle tinted bg + solid dot |
| `MetricCard` | label (Inter sm muted) + value (Fraunces 3xl) + delta chip + optional `Sparkline` |
| `Sparkline` | zero-dep inline SVG trend (emerald stroke, soft area fill) |
| `DataTable` | hairline rows, `surface.subtle` header, hover row lift, sticky header, cursor "Load more" |
| `EmptyState` | icon + Fraunces title + muted copy + primary action |
| `Skeleton` | shimmer blocks for cards/rows/metrics |
| `CopyField` / `AddressChip` | truncated mono value + copy (reuses the `KeyValueRow` copy affordance) |
| `AmountDisplay` | `formatMinorUnits`, currency in muted caps, figure in Fraunces |
| `Modal`/`Drawer` | `lg` shadow, scrim, focus trap; Drawer is the mobile sidebar |
| `Tabs` | underline emerald active; for account/onboarding/dev sections |
| `QRBlock` | PayLink URL → QR for the payer hand-off (PLANNED dep on a QR lib; placeholder LIVE) |
| `Stepper`/`Wizard` | the create→pay→settle progress (refactor of work07's wizard) |

### 2.6 Status → visual mapping (the source of truth for `StatusPill`)

| Domain | Status | kind |
|---|---|---|
| **PayLink** | CREATED | neutral |
| | PENDING | pending |
| | VERIFIED | success |
| | FAILED / CANCELLED | danger |
| | EXPIRED | expired |
| **Payment** | AWAITING_PAYMENT | pending |
| | SETTLED | success |
| | FAILED / CANCELLED | danger |
| **Merchant** | DRAFT | neutral |
| | PENDING_VERIFICATION | pending |
| | ACTIVE | success |
| | REJECTED | danger |
| | SUSPENDED | expired |
| **Bank account** | PENDING_VERIFY | pending · VERIFIED → success · REJECTED → danger |
| **KYC tier** | 0 | neutral · 1 → pending · 2 → success (+ risk flags as danger chips) |
| **API key / session** | ACTIVE | success · REVOKED → expired |

### 2.7 Accessibility & responsive rules

- All interactive elements reachable and operable by keyboard; focus order matches visual order.
- Focus ring: 2px `emerald.600` offset 2px on `canvas`.
- Color is never the only signal — `StatusPill` pairs color with a dot + text label.
- Tables collapse to stacked cards < `sm`; the sidebar becomes a Drawer < `md`.
- Respect `prefers-reduced-motion` and `prefers-color-scheme` (dark seam).

---

## 3. Surfaces & Screens

Each surface: **persona · screens · components · data (SDK + backend §/work) · states · phase ·
acceptance**.

### 3.1 Public Resolve & Pay *(Payer — LIVE)*

**Persona.** Guest payer who tapped a link / scanned a QR. No account (guest checkout per PRD F1.2).

**Screens.**
- **Pay page** (`/pay/[plId]`): merchant + amount (`AmountDisplay`) + expiry + status; method picker
  (rail label+icon, F.3); for MPesa → STK push instructions (`PayInstructions`) + paybill/account
  copy; live settlement (`SettlementStatus` poll) → success receipt with chain tx hash.

**Data.** `paylinks.get(plId)` (public, work01 §2.5) · `payments.initiate({paylink_id, rail})`
(work02 §2.10) · poll `paylinks.get` to VERIFIED (on-chain source of truth). MPesa charge is driven
by the adapter (work04 §2.14) behind the orchestrator.

**States.** Loading skeleton on resolve; not-found / expired / already-settled rendered as
distinct, calm screens; `PAYLINK_NOT_PAYABLE` (the work35 gap) shown as a neutral note while the poll
remains the settlement truth (the work07 behavior, preserved).

**Phase.** LIVE. Today the wizard at `/` covers create→pay→settle; the dedicated `/pay/[plId]`
resolution route is the productized form (folds the existing wizard components).

**Acceptance.** A real PayLink resolves; selecting MPesa initiates a charge; the page flips
PENDING→VERIFIED from on-chain state and shows the tx hash; no PIN/PAN is ever entered in the UI (F.2).

### 3.2 Auth & Account *(All users — LIVE, needs SDK identity resource — §4)*

**Persona.** Any registered user (merchant, developer, admin).

**Screens.**
- **Login / Register** (`/login`, `/register`): email/phone + password; MFA challenge when enabled.
- **MFA**: enroll (TOTP secret + otpauth QR), verify, disable.
- **Account** (`/account`): profile (email/phone, edit), **sessions** (list + revoke), **API keys**
  (issue → secret shown once → list → revoke), **organizations** + members, **KYC status** (tier +
  risk flags).

**Data.** `/v1/auth/*`, `/v1/users/me`, `/v1/users/me/api-keys`, `/v1/organizations`, `/v1/sessions`
(work09 §2.2); `/v1/compliance/status` + `/v1/kyc/sessions` (work12 §2.6).

**States.** Refresh-token rotation handled transparently; reuse-detection → forced re-login;
issued API-key secret shown exactly once with a copy + "you won't see this again" warning; KYC tier
gates surfaced as upgrade prompts.

**Phase.** LIVE backend; **SDK gap** — `@linkmint/sdk` covers only paylinks/payments today. Requires
an identity/compliance resource addition to the SDK (§4) before these screens are buildable per F.1.

**Acceptance.** Register→login→MFA enroll→login-requires-MFA; issue/list/revoke an API key; revoke a
session; view KYC tier — all through the SDK, errors via the envelope.

### 3.3 Merchant Dashboard *(Merchant — LIVE core + PLANNED analytics)*

**Persona.** SMB merchant/receiver. Low-to-moderate technical skill; wants status at a glance.

**Screens.**
- **Overview** (`/dashboard`) — **FLAGSHIP**: `MetricCard`s (Total settled, Active PayLinks,
  Pending), a recent-activity `Sparkline`, and a recent PayLinks `DataTable`, with a "Create PayLink"
  CTA. *(LIVE — aggregates derived client-side from `paylinks.list`.)*
- **PayLinks** (`/dashboard/paylinks`): filterable/paginated table; create (amount, currency, expiry,
  usage, metadata); detail drawer; cancel. *(LIVE — work01.)*
- **Payments** (`/dashboard/payments`): list + detail (rail, status, timestamps). *(LIVE — work02.)*
- **Onboarding** (`/dashboard/onboarding`): KYB business details, document upload, bank-account add +
  verify, contract accept, fee-tier view. *(LIVE — work10 §2.3; SDK gap §4.)*
- **Settlements** (`/dashboard/settlements`): payout batches + reconciliation. *(PLANNED — work23.)*
- **Analytics** (`/dashboard/analytics`): revenue series, conversion, rail mix. *(PLANNED — work26.)*
- **Refunds / Disputes** *(PLANNED — work22)*; **Invoices** *(PLANNED — work19)*; **Webhooks**
  *(PLANNED — work14 + Phase-2 webhook registration).*

**Data.** `paylinks.list/get/create/cancel` (work01), `payments.get` (work02), `/v1/merchants/*`
(work10). Overview aggregates: count by `status`, sum `amount` where `VERIFIED`, sparkline bucketed
by `created_at` — all computed from the live list (no analytics service needed for the basics).

**States.** Skeleton metric cards + table on load; `EmptyState` ("No PayLinks yet → Create your
first") when empty; PLANNED tabs render a labelled "Coming in Phase 2" panel (F.7).

**Phase.** Overview + PayLinks + Payments **LIVE**; Onboarding LIVE pending SDK §4; the rest PLANNED.

**Acceptance.** Overview lists real PayLinks with correct `StatusPill`s; metrics match the data;
create→appears in table; cancel→reflects new status; PLANNED tabs never show fake numbers.

### 3.4 Admin Console *(Admin / staff — LIVE)*

**Persona.** Internal ops staff. Requires admin role + **MFA** + `support.read` scope (work11).

**Screens.**
- **Search** (`/admin`): unified query across users/merchants/PayLinks/payments; degraded-result
  banner when an upstream is down.
- **Entity drill-down**: user / merchant / paylink / payment detail panels.
- **Audit log** viewer: filterable entries + a "Verify chain integrity" action showing
  `{ok, broken_at?}` (work13 §2.17).
- **Mutations** (suspend, force-refund, override) — *(PLANNED — Phase-2 work11 mutations.)*

**Data.** `/v1/admin/search`, `/v1/admin/{users,merchants,paylinks,payments}/{id}` (work11 §2.18);
`/v1/audit-log*` (work13). MFA claim required — the console refuses non-MFA tokens.

**Phase.** LIVE (read-only); SDK gap §4. Mutations PLANNED.

**Acceptance.** MFA-gated entry; search returns results across types and degrades gracefully; audit
verify reports ok/broken honestly.

### 3.5 Developer Portal *(Developer — LIVE core + PLANNED)*

**Screens.** API keys (issue/list/revoke, reuses §3.2) · SDK quickstart (`npm i @linkmint/sdk` +
create-paylink snippet) · **docs / OpenAPI explorer**, **sandbox**, **webhook tester**, **usage
metrics** *(PLANNED — depends on the work05 OpenAPI-aggregation follow-up + Phase-2 webhooks).*

**Data.** `/v1/users/me/api-keys` (work09). Docs render the gateway's aggregated OpenAPI once
available.

**Phase.** API keys + quickstart LIVE (via SDK §4); docs/sandbox PLANNED.

**Acceptance.** A developer can mint a key, copy the quickstart, and make a first call.

### 3.6 Wallet & Staking *(Validator / advanced — PLANNED)*

PLN balance, staking positions, rewards, and non-custodial token send (build→sign→broadcast).
**PLANNED — work24/34.** Specified for completeness; rendered as a locked "Coming soon" surface.

---

## 4. SDK ↔ UI data contract

The frontend's F.1 (SDK-only) means a screen can only be built once its data has a typed SDK method.
Today `@linkmint/sdk` (work06) exposes **paylinks** + **payments** only.

| Surface | SDK resource needed | Exists today? |
|---|---|---|
| Public Pay, Merchant PayLinks/Payments, Dashboard Overview | `paylinks.*`, `payments.*` | ✅ yes |
| Auth & Account, Developer keys | `auth.*`, `users.*`, `apiKeys.*`, `organizations.*`, `sessions.*` | ❌ **SDK gap** |
| Merchant Onboarding | `merchants.*` | ❌ **SDK gap** |
| KYC status | `compliance.*` | ❌ **SDK gap** |
| Admin Console | `admin.*`, `auditLog.*` | ❌ **SDK gap** |
| Settlements / Analytics / Refunds / Invoices / Wallet | resources | ❌ (PLANNED backend) |

**Dependency.** Expanding `@linkmint/sdk` with identity/merchant/compliance/admin/audit resources is a
prerequisite for §3.2/3.4/3.5 and the onboarding tab of §3.3. Filed as a **frontend backlog item
(fe-sdk)** / work06 follow-up — types mirror the wire shape byte-for-byte (the work06 convention),
errors reuse the existing typed hierarchy (`src/lib/errors.ts:toDisplayError`). Screens backed only by
paylinks/payments (Public Pay, Dashboard Overview, PayLinks, Payments) are buildable **now**.

---

## 5. Realtime & polling

**Today (LIVE).** Settlement freshness is polling: `useSettlementStatus` polls `paylinks.get` every
`NEXT_PUBLIC_SETTLEMENT_POLL_MS` (default 2500ms) with an `AbortController`, stopping at a terminal
status. Cheap, robust, and the on-chain read is the source of truth.

**Upgrade path (PLANNED).** The chain exposes a WebSocket `datastream`; a thin client subscription
would push `paylink.verified/failed/cancelled` for instant updates, with polling as the fallback.
Tracked as a frontend backlog item; no UI contract change (the same status drives the same `StatusPill`).

---

## 6. Frontend Work Backlog & Coverage Matrix

Parallels [`workload/backlog.md`](workload/backlog.md). Frontend items are `fe`-prefixed; each maps to
its surface, the backend work it consumes, and a phase. Anchors: **work07** (web baseline, done) and
**work33** (dashboards, Phase 3).

> **Execution:** this `feNN` backlog is now executed through a dedicated work/flow tree at
> [`workload/frontend/`](workload/frontend/backlog.md) — 30 `work`/`flow` pairs (foundation, system UX,
> per-feature screens, polish). Run `/work <nn>` there to build a surface.

| # | Item | Surface (§) | Phase | Depends on | Status |
|---|---|---|---|---|---|
| fe01 | Ivory Premium design system + theme | §2 | 1 | work07 | **this change** |
| fe02 | App shell (sidebar/topbar) + base components | §2.5 | 1 | fe01 | **this change** |
| fe03 | Merchant Dashboard overview (flagship) | §3.3 | 1 | work01, fe02 | **this change** |
| fe04 | PayLinks management (list/create/cancel/detail) | §3.3 | 1 | work01 | todo |
| fe05 | Public Resolve & Pay route (`/pay/[plId]`) | §3.1 | 1 | work01/02/04 | todo (wizard LIVE at `/`) |
| fe-sdk | SDK expansion (identity/merchant/compliance/admin/audit) | §4 | 1 | work06,09,10,11,12,13 | todo |
| fe06 | Auth & Account | §3.2 | 1 | fe-sdk, work09/12 | todo |
| fe07 | Admin Console (read-only) | §3.4 | 1 | fe-sdk, work11/13 | todo |
| fe08 | Developer Portal (keys + quickstart) | §3.5 | 1 | fe-sdk, work09 | todo |
| fe09 | Merchant Onboarding (KYB/bank/contracts/fee) | §3.3 | 1 | fe-sdk, work10 | todo |
| fe10 | Settlements + Analytics | §3.3 | 2 | work23, work26 | PLANNED |
| fe11 | Refunds / Disputes / Invoices / Webhooks | §3.3 | 2 | work22/19/14 | PLANNED |
| fe12 | Realtime (WS datastream) | §5 | 2 | chain datastream | PLANNED |
| fe13 | Wallet & Staking + token send | §3.6 | 2 | work24/34 | PLANNED |
| fe14 | Dark mode + i18n + mobile (Flutter parity) | §2 | 3 | work32/33 | PLANNED |

**Persona coverage.** Payer → §3.1. Merchant → §3.3 (+3.2). Developer → §3.5 (+3.2). Admin/staff →
§3.4. Validator → §3.6.

## 7. Frontend Definition of Done

Extends the **App** checklist in [`workload/definition-of-done.md`](workload/definition-of-done.md). A
frontend item is done only when:

- [ ] Talks to the API **only** through `@linkmint/sdk` (F.1); no raw `/v1` fetch.
- [ ] Handles **loading / empty / error** states; errors via the standard envelope (`ErrorBanner` +
      `toDisplayError`) with `trace_id` (F.5).
- [ ] Respects F.2 (non-custodial — no PIN/PAN capture) and F.3 (rail-agnostic views).
- [ ] **WCAG 2.1 AA** (F.6): keyboard, focus, contrast, reduced-motion.
- [ ] Every surface tagged LIVE/PLANNED; no PLANNED surface fakes data (F.7).
- [ ] Strict TS, **no `any`**; `npm run typecheck` + `npm run lint` (ESLint + Prettier) clean.
- [ ] `npm run build` (`next build`) green — building `@linkmint/sdk` first (it's a `file:` dep).
- [ ] Component tests where logic warrants (Vitest); coverage tracked toward the ≥80% project target.
- [ ] **Verified live** against `docker compose --profile e2e` per
      [`workload/verification.md`](workload/verification.md); result reported honestly.

## 8. Acceptance criteria by phase

- **Phase 1 (this spec's live scope):** the design system (§2) is the app-wide theme; the flagship
  Merchant Dashboard (§3.3) renders real `paylinks.list` data with correct status, metrics, and
  styling; the work07 create→pay→settle wizard still passes under the new theme; fe04–fe09 buildable
  once `fe-sdk` lands.
- **Phase 2:** settlements/analytics/refunds/invoices/webhooks/realtime surfaces light up as
  work19–26 ship; dark mode toggle.
- **Phase 3:** wallet/staking + token send (work24/34); SDK suite + Flutter mobile parity (work32);
  full i18n.
