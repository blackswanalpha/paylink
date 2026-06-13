-- work23 settlement-service: initial schema.
-- One schema per service ("settlement"); no cross-schema foreign keys — pl_id / merchant_key /
-- merchant_id are opaque references to entities owned by the chain, paylink-service, and
-- merchant-onboarding.
--
-- NON-CUSTODIAL (A.1): there are NO balance columns. amount/gross/fee/net mirror on-chain truth so
-- a payout can carry a complete INSTRUCTION; funds are never held here. Every monetary flow is also
-- recorded as a balanced double-entry posting in the shared "ledger" schema (A.6, work16).

CREATE SCHEMA IF NOT EXISTS settlement;

-- settlements: one row per (merchant_key, currency, settlement_date) period.
-- status OPEN (accumulating) → CLOSED (cutoff passed, payout schedulable) → PAID (rail file matched).
-- cutoff_at is the T+1 instant (start of the day after settlement_date in the merchant's tz).
CREATE TABLE IF NOT EXISTS settlement.settlements (
    id              text          PRIMARY KEY,
    merchant_key    text          NOT NULL,
    currency        text          NOT NULL,
    settlement_date date          NOT NULL,
    status          text          NOT NULL CHECK (status IN ('OPEN', 'CLOSED', 'PAID')),
    gross           numeric(38,0) NOT NULL DEFAULT 0,
    platform_fee    numeric(38,0) NOT NULL DEFAULT 0,
    chain_fee       numeric(38,0) NOT NULL DEFAULT 0,
    net             numeric(38,0) NOT NULL DEFAULT 0,
    cutoff_at       timestamptz   NOT NULL,
    opened_at       timestamptz   NOT NULL DEFAULT now(),
    closed_at       timestamptz,
    UNIQUE (merchant_key, currency, settlement_date)
);
CREATE INDEX IF NOT EXISTS settlements_merchant_idx ON settlement.settlements (merchant_key, opened_at DESC);
-- Scheduler scans: due-to-close OPEN settlements, and CLOSED settlements awaiting a payout.
CREATE INDEX IF NOT EXISTS settlements_due_idx ON settlement.settlements (cutoff_at) WHERE status = 'OPEN';
CREATE INDEX IF NOT EXISTS settlements_closed_idx ON settlement.settlements (status) WHERE status = 'CLOSED';

-- settlement_items: one paylink item per pl_id (idempotent via the partial unique index below).
-- A clawback (refund recovery) is an item with kind='clawback' and a negative net that offsets the
-- merchant's payout; multiple clawbacks per pl_id are allowed (deduped by refund_id upstream).
CREATE TABLE IF NOT EXISTS settlement.settlement_items (
    id               text          PRIMARY KEY,
    settlement_id    text          NOT NULL REFERENCES settlement.settlements (id),
    pl_id            text          NOT NULL,
    kind             text          NOT NULL DEFAULT 'paylink' CHECK (kind IN ('paylink', 'clawback')),
    gross            numeric(38,0) NOT NULL DEFAULT 0,
    platform_fee     numeric(38,0) NOT NULL DEFAULT 0,
    chain_fee        numeric(38,0) NOT NULL DEFAULT 0,
    net              numeric(38,0) NOT NULL,
    verified_tx_hash text          NOT NULL DEFAULT '',
    created_at       timestamptz   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS settlement_items_paylink_uniq
    ON settlement.settlement_items (pl_id) WHERE kind = 'paylink';
CREATE INDEX IF NOT EXISTS settlement_items_settlement_idx ON settlement.settlement_items (settlement_id);
CREATE INDEX IF NOT EXISTS settlement_items_pl_idx ON settlement.settlement_items (pl_id);

-- payouts: one instruction per settlement (UNIQUE settlement_id). reference is the stable token a
-- rail settlement file echoes for matching. SCHEDULED→INSTRUCTED (A.1 — instruction only)→PAID|FAILED.
CREATE TABLE IF NOT EXISTS settlement.payouts (
    id            text          PRIMARY KEY,
    settlement_id text          NOT NULL REFERENCES settlement.settlements (id),
    merchant_key  text          NOT NULL,
    rail          text          NOT NULL,
    currency      text          NOT NULL,
    amount        numeric(38,0) NOT NULL CHECK (amount > 0),
    status        text          NOT NULL CHECK (status IN ('SCHEDULED', 'INSTRUCTED', 'PAID', 'FAILED')),
    reference     text          NOT NULL UNIQUE,
    scheduled_for timestamptz   NOT NULL,
    instructed_at timestamptz,
    paid_at       timestamptz,
    created_at    timestamptz   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS payouts_one_per_settlement ON settlement.payouts (settlement_id);
CREATE INDEX IF NOT EXISTS payouts_merchant_idx ON settlement.payouts (merchant_key, created_at DESC);

-- merchant_directory / bank_accounts: projections from merchant.* events (work10) that enrich payout
-- routing. No plaintext bank details (A.4 — rail + currency + status only).
CREATE TABLE IF NOT EXISTS settlement.merchant_directory (
    merchant_id  text        PRIMARY KEY,
    tz           text        NOT NULL DEFAULT '',
    default_rail text        NOT NULL DEFAULT '',
    status       text        NOT NULL DEFAULT '',
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS settlement.bank_accounts (
    bank_account_id text        PRIMARY KEY,
    merchant_id     text        NOT NULL,
    rail            text        NOT NULL,
    currency        text        NOT NULL,
    status          text        NOT NULL,
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS bank_accounts_merchant_idx ON settlement.bank_accounts (merchant_id);

-- rail_files / rail_file_lines: ingested rail settlement files. Matched lines flip a payout to PAID;
-- UNMATCHED lines are left for the work27 reconciliation algorithm.
CREATE TABLE IF NOT EXISTS settlement.rail_files (
    id            text        PRIMARY KEY,
    rail          text        NOT NULL,
    line_count    int         NOT NULL DEFAULT 0,
    matched_count int         NOT NULL DEFAULT 0,
    received_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS settlement.rail_file_lines (
    id         bigserial     PRIMARY KEY,
    file_id    text          NOT NULL REFERENCES settlement.rail_files (id),
    reference  text          NOT NULL,
    amount     numeric(38,0) NOT NULL,
    currency   text          NOT NULL,
    status     text          NOT NULL CHECK (status IN ('MATCHED', 'UNMATCHED')),
    created_at timestamptz   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS rail_file_lines_file_idx ON settlement.rail_file_lines (file_id);
CREATE INDEX IF NOT EXISTS rail_file_lines_unmatched_idx
    ON settlement.rail_file_lines (reference) WHERE status = 'UNMATCHED';

-- clawbacks: refund clawbacks recorded as negative offsets (links to the offsetting settlement).
CREATE TABLE IF NOT EXISTS settlement.clawbacks (
    id            text          PRIMARY KEY,
    refund_id     text          NOT NULL,
    merchant_key  text          NOT NULL,
    pl_id         text          NOT NULL,
    amount        numeric(38,0) NOT NULL CHECK (amount > 0),
    currency      text          NOT NULL,
    settlement_id text          NOT NULL REFERENCES settlement.settlements (id),
    created_at    timestamptz   NOT NULL DEFAULT now()
);

-- processed_events: canonical consumer-dedupe table for DbDedupe (work17) — column definitions
-- byte-identical to idempotency-go/processed_events.sql, created in this service's own schema. The
-- dedupe row is inserted on the SAME transaction as the state + ledger writes, so an at-least-once
-- redelivery applies its effect exactly once.
CREATE TABLE IF NOT EXISTS settlement.processed_events (
    scope        text        NOT NULL,
    dedupe_key   text        NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, dedupe_key)
);
