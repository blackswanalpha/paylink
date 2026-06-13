# wallet-service — design (work24)

## Goal & boundaries

A read-side surface over on-chain PLN: balances, transaction history, staking positions/rewards, and
treasury stats, built by indexing `chain.*` events. It also builds **unsigned** staking-intent
transactions. It is strictly **non-custodial (A.1)**: it never holds private keys or funds, and the
send→sign→broadcast path is deferred to work34.

It mirrors the work23 settlement-service Go/chi shape but is simpler: it talks to the chain JSON-RPC,
and it records **no** money flows — so there is **no `ledger-go` dependency, no publisher, and no
scheduler**.

## Key decisions

### 1. Balance: read-through cache, not event reconstruction
`GET /v1/wallets/{addr}` is the source of "balance truth from chain". Reconstructing exact balances
from the event stream is fragile (genesis allocations and mints are not fully derivable), so the
balance/nonce are read through `paylink_getAccount` and cached in `wallet.account_balances`
(`WALLET_BALANCE_CACHE_TTL_SECONDS`). When the chain is unreachable, a cached row is served with
`stale:true`; with no cached row, the call returns `CHAIN_UNAVAILABLE`. Everything else (history,
positions, rewards, treasury) is a **pure event projection** that keeps serving while the chain is down.

### 2. readyz: postgres hard, chain soft
The indexed read-side does not need the chain to be live, so `readyz` fails (503) only when postgres
is down. The chain is probed and reported in a non-fatal `degraded` field — readiness stays 200. This
deliberately diverges from settlement-service (which has no chain dep) so the read-side stays available.

### 3. Non-custodial intent path
`POST /v1/staking/intent` is a pure function of `(addr, action, amount, nonce, chain_id)`. It builds
the unsigned tx with `pkg/lvm` (`BuildStakeTx` / `BuildInitiateUnstakeTx` — the latter added additively
to `paylink-chain/pkg/lvm` for work24) and returns `tx.SignableBytes()`. The signature, pubkey, and
hash are left zero — **no key material is ever loaded or returned**. The live nonce comes from
`paylink_getNonce` (the one endpoint that needs the chain up). Because the request mutates no state and
is deterministic, no `Idempotency-Key` is wired (DbDedupe still guards the consumer).

### 4. Treasury accounting (avoid double-counting the burn)
`chain.fee.collected` accrues the fee / validator-reward / treasury **deltas**; `chain.token.burned`
sets the **authoritative** cumulative `total_burned`. The burn share is therefore owned by a single
event family, so a redelivery of either cannot double-count. Supply (`total_supply`/`max_supply`) is
refreshed live from `paylink_tokenStats` on read and snapshotted for the chain-down case.

### 5. Position vs reward history sources
`staking_positions.total_rewards` mirrors the chain's `TotalRewards`, bumped only by
`chain.validator.rewarded` (admin reward distribution). The per-settlement fee split
(`chain.fee.distributed`) is appended to `staking_rewards` as `fee_share` rows without touching the
position total — matching what `paylink_getValidator` would report.

## Idempotency

The chain indexer is at-least-once. Each projection runs inside one transaction joined with a DbDedupe
mark in `wallet.processed_events` (scope = event name, key = `tx_hash[:addr]`), so a redelivery applies
exactly once. Returning an error from a handler leaves the offset uncommitted → redelivery.

## Schema (`wallet`)

`account_balances` (read-through cache) · `transactions` (history, keyset-paginated by
`(block_height DESC, id DESC)`) · `staking_positions` · `staking_rewards` · `treasury_stats`
(single row) · `processed_events` (DbDedupe). NUMERIC(38,0) for all token amounts, cast to text on read
and scanned into `*big.Int` (exact integers, never float).

## Gateway

`/v1/wallets` + `/v1/staking` are authed (jwt OR key-auth) and self-scoped: the gateway post-function
injects the trustworthy `X-Creator-Addr` and the service rejects a caller reading another address.
`/v1/treasury` is a public pass-through route (no auth, not in the post-function allow-list).
