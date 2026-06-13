-- work24 wallet-service: initial schema.
-- One schema per service ("wallet"); no cross-schema foreign keys — addresses, tx hashes, and
-- paylink ids are opaque references to entities owned by the chain.
--
-- NON-CUSTODIAL (A.1): no funds are held here. account_balances is a READ-THROUGH CACHE of on-chain
-- truth (paylink_getAccount), refreshed on GET /v1/wallets/{addr}; transactions / staking_positions /
-- staking_rewards / treasury_stats are pure projections of the chain.* event stream (work15).

CREATE SCHEMA IF NOT EXISTS wallet;

-- account_balances: read-through cache of paylink_getAccount. Served within the cache TTL, else
-- refreshed from the chain RPC; served stale (with the chain down) when a row exists.
CREATE TABLE IF NOT EXISTS wallet.account_balances (
    addr         text          PRIMARY KEY,
    balance      numeric(38,0) NOT NULL DEFAULT 0,
    nonce        bigint        NOT NULL DEFAULT 0,
    block_height bigint        NOT NULL DEFAULT 0,
    source       text          NOT NULL DEFAULT 'rpc',
    fetched_at   timestamptz   NOT NULL DEFAULT now()
);

-- transactions: per-address movement history, projected from chain.account.transfer and
-- chain.validator.* events. A transfer yields two rows (an 'out' row on the sender, an 'in' row on
-- the receiver) so each address sees its own side. Newest-first by (block_height, id).
CREATE TABLE IF NOT EXISTS wallet.transactions (
    id           text          PRIMARY KEY,
    addr         text          NOT NULL,
    counterparty text          NOT NULL DEFAULT '',
    direction    text          NOT NULL CHECK (direction IN ('in', 'out', 'self')),
    kind         text          NOT NULL CHECK (kind IN ('transfer', 'stake', 'unstake_start', 'unstake_complete', 'reward', 'slash')),
    amount       numeric(38,0) NOT NULL DEFAULT 0,
    tx_hash      text          NOT NULL DEFAULT '',
    block_height bigint        NOT NULL DEFAULT 0,
    occurred_at  timestamptz   NOT NULL,
    created_at   timestamptz   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS transactions_addr_idx
    ON wallet.transactions (addr, block_height DESC, id DESC);

-- staking_positions: one row per validator address, mutated by chain.validator.* events. Mirrors the
-- on-chain ValidatorInfo (staked/pending/rewards/slashed/active).
CREATE TABLE IF NOT EXISTS wallet.staking_positions (
    addr               text          PRIMARY KEY,
    staked_amount      numeric(38,0) NOT NULL DEFAULT 0,
    pending_withdrawal numeric(38,0) NOT NULL DEFAULT 0,
    total_rewards      numeric(38,0) NOT NULL DEFAULT 0,
    total_slashed      numeric(38,0) NOT NULL DEFAULT 0,
    withdrawable_at    timestamptz,
    is_active          boolean       NOT NULL DEFAULT false,
    updated_at         timestamptz   NOT NULL DEFAULT now()
);

-- staking_rewards: append-only reward history. validator_reward rows come from chain.validator.rewarded
-- (admin reward distribution), fee_share rows from chain.fee.distributed (per-settlement fee split).
CREATE TABLE IF NOT EXISTS wallet.staking_rewards (
    id            text          PRIMARY KEY,
    addr          text          NOT NULL,
    amount        numeric(38,0) NOT NULL DEFAULT 0,
    total_rewards numeric(38,0) NOT NULL DEFAULT 0,
    source        text          NOT NULL CHECK (source IN ('validator_reward', 'fee_share')),
    tx_hash       text          NOT NULL DEFAULT '',
    block_height  bigint        NOT NULL DEFAULT 0,
    occurred_at   timestamptz   NOT NULL,
    created_at    timestamptz   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS staking_rewards_addr_idx
    ON wallet.staking_rewards (addr, block_height DESC, id DESC);

-- treasury_stats: single-row (id=1) running aggregate accumulated from chain.fee.collected (fees /
-- validator-reward / treasury deltas) and chain.token.burned (authoritative cumulative burn).
-- Supply/max_supply are refreshed live from paylink_tokenStats on read and snapshotted here for the
-- chain-down case.
CREATE TABLE IF NOT EXISTS wallet.treasury_stats (
    id                int           PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    total_supply      numeric(38,0) NOT NULL DEFAULT 0,
    max_supply        numeric(38,0) NOT NULL DEFAULT 0,
    total_burned      numeric(38,0) NOT NULL DEFAULT 0,
    fees_collected    numeric(38,0) NOT NULL DEFAULT 0,
    validator_rewards numeric(38,0) NOT NULL DEFAULT 0,
    treasury_amount   numeric(38,0) NOT NULL DEFAULT 0,
    chain_height      bigint        NOT NULL DEFAULT 0,
    updated_at        timestamptz   NOT NULL DEFAULT now()
);
INSERT INTO wallet.treasury_stats (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

-- processed_events: canonical consumer-dedupe table for DbDedupe (work17) — column definitions
-- byte-identical to idempotency-go/processed_events.sql, created in this service's own schema. The
-- dedupe row is inserted on the SAME transaction as the projection write, so an at-least-once
-- redelivery applies its effect exactly once.
CREATE TABLE IF NOT EXISTS wallet.processed_events (
    scope        text        NOT NULL,
    dedupe_key   text        NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, dedupe_key)
);
