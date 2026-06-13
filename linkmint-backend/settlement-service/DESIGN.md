# settlement-service — design notes (work23)

Mirrors the work20 escrow-manager Go/chi shape (ports/adapters, `internal/httpx`, DbDedupe consumer,
sweeper-style background loop). What is settlement-specific:

## Aggregation model

- A **settlement** is keyed by `(merchant_key, currency, settlement_date)`. `merchant_key` is the
  PayLink's on-chain **Receiver** address — the merchant's payout identity. It is the same value the
  gateway injects as `X-Creator-Addr`, so a merchant reads exactly their own settlements.
- The chain events were enriched (work23) so settlement needs no extra lookup: `chain.paylink.verified`
  now carries `payee` (Receiver) + `amount` (gross); `chain.fee.collected` carries the protocol
  `totalFee`. Gross/net are recorded on `verified`; the chain fee is attached on `fee.collected`.
- `net = gross − platform_fee − chain_fee`. The **platform fee** (A.5, LinkMint's margin) is an
  optional config bps on gross, kept strictly separate from the **chain fee** (the 0.5% PLN inflation
  fee the protocol mints/splits). `SETTLEMENT_PLATFORM_FEE_BPS` defaults to 0 (record chain fee only).

## Ledger (A.6) — every flow is balanced

All postings run on the same pgx transaction as the state write + the DbDedupe mark (commit together):

| Trigger | DR | CR |
|---|---|---|
| verified (gross G, platform fee Fp) | `settlement:clearing:CCY` G | `merchant_payable:<payee>` (G−Fp) [+ `fee:platform:CCY` Fp] |
| fee.collected (chain fee Fc) | `merchant_payable:<payee>` Fc | `fee:chain:CCY` Fc |
| clawback (amount C) | `merchant_payable:<payee>` C | `settlement:clearing:CCY` C |
| payout PAID (net N, on rail-file match) | `merchant_payable:<payee>` N | `settlement:clearing:CCY` N |

A payout INSTRUCTION posts nothing — only the confirmed outflow (rail file match → PAID) clears the
payable. Integration tests assert `ledger.IsBalanced` after each flow.

## Lifecycle

- Settlement `OPEN` → `CLOSED` (T+1 cutoff passed, in the merchant tz) → `PAID` (rail file matched).
- `cutoff_at` is stored per settlement = start of the day after `settlement_date` in the configured
  tz. The scheduler CAS-closes `OPEN` rows with `cutoff_at <= now`, then instructs a payout for each
  `CLOSED` row whose `net >= SETTLEMENT_MIN_PAYOUT[currency]`. Below the minimum the settlement stays
  CLOSED (carried, not paid).
- Payout `SCHEDULED`/`INSTRUCTED` (instruction only, A.1) → `PAID`|`FAILED`. One payout per settlement
  (`UNIQUE(settlement_id)`); `reference = PO-<settlement_id>` is what the rail file echoes.

## Rail-file ingest

`POST /settlements/files/ingest` (internal, token-guarded) parses JSON or CSV, matches each line to a
payout by `(reference, amount, currency)`, flips matches → PAID (+ settlement PAID, + ledger), and
leaves unmatched lines for the **work27** reconciliation algorithm (out of scope here). Idempotent on
the file id.

## Known seams / MVP gaps

- **Merchant-directory join.** `merchant.onboarded` / `merchant.bank_account.verified` are projected
  into `merchant_directory` / `bank_accounts`, but there is no on-chain-address ↔ `merchant_id` link
  anywhere in the platform yet, so payout routing falls back to `SETTLEMENT_DEFAULT_RAIL` and the
  destination is the payee address. When merchant-onboarding registers a settlement address, join on it.
- **At-most-once publish.** Events are published after the state commit (fire-and-log); a publish
  failure is not retried (DbDedupe would suppress a re-publish on redelivery). The durable fix is an
  outbox relay (same gap escrow-manager documents). State + ledger are always durable.
- **Clawback ↔ pl_id.** `refund.clawback.requested` was enriched (refund-dispute work22) to carry
  `paylink_id`, so settlement resolves the merchant from the PayLink it already settled. A clawback for
  a pl_id this service never settled is acked + logged (no offset).
- **Single currency.** `SETTLEMENT_CURRENCY` (Phase 2). Multi-currency / splitting / instant payouts
  are Phase 3.
