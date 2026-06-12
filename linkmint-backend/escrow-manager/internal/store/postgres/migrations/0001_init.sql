-- work20 escrow-manager: initial schema.
-- One schema per service ("escrow"); no cross-schema foreign keys — pl_id is an opaque
-- reference to the PayLink owned by paylink-service / the chain.
--
-- NON-CUSTODIAL (A.1): there are NO balance columns anywhere. amount/currency mirror the
-- PayLink so release/refund events can carry a complete INSTRUCTION; funds are never held here.

CREATE SCHEMA IF NOT EXISTS escrow;

-- escrows: one coordination record per PayLink (pl_id UNIQUE — one escrow per PayLink, A.7).
-- state follows the WAITING→CONDITIONS_MET→RELEASED|REFUNDED|DISPUTED machine (internal/fsm);
-- CONDITIONS_MET is allowed by the CHECK but never persisted (ConditionsMet+Release are applied
-- together in one transaction). funded is a flag set by the chain.paylink.verified consumer,
-- not a state.
CREATE TABLE IF NOT EXISTS escrow.escrows (
    id               text          PRIMARY KEY,
    pl_id            text          NOT NULL UNIQUE,
    creator_addr     text          NOT NULL,
    payee_addr       text          NOT NULL,
    refund_to        text          NOT NULL,
    amount           numeric(30,0) NOT NULL CHECK (amount > 0),
    currency         text          NOT NULL,
    condition_type   text          NOT NULL CHECK (condition_type IN
                         ('delivery_confirmation', 'time_lock', 'multi_party_approval')),
    condition_params jsonb         NOT NULL DEFAULT '{}'::jsonb,
    state            text          NOT NULL CHECK (state IN
                         ('WAITING', 'CONDITIONS_MET', 'RELEASED', 'REFUNDED', 'DISPUTED')),
    funded           boolean       NOT NULL DEFAULT false,
    funded_tx_hash   text          NOT NULL DEFAULT '',
    release_at       timestamptz,
    timeout_at       timestamptz   NOT NULL,
    dispute_reason   text          NOT NULL DEFAULT '',
    created_at       timestamptz   NOT NULL,
    updated_at       timestamptz   NOT NULL
);

-- Sweep indexes: the sweeper CAS-updates WAITING rows by due time.
CREATE INDEX IF NOT EXISTS escrows_sweep_timeout_idx
    ON escrow.escrows (timeout_at) WHERE state = 'WAITING';
CREATE INDEX IF NOT EXISTS escrows_sweep_release_idx
    ON escrow.escrows (release_at) WHERE state = 'WAITING' AND funded;
-- Creator-scoped listing.
CREATE INDEX IF NOT EXISTS escrows_creator_idx
    ON escrow.escrows (creator_addr, created_at DESC);

-- approvals: recorded confirmations/approvals. The (escrow_id, approver_addr) primary key makes
-- N-of-M approval idempotent — a repeated approval is a no-op insert.
CREATE TABLE IF NOT EXISTS escrow.approvals (
    escrow_id     text        NOT NULL REFERENCES escrow.escrows (id),
    approver_addr text        NOT NULL,
    approved_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (escrow_id, approver_addr)
);

-- processed_events: canonical consumer-dedupe table for DbDedupe (work17) — column definitions
-- byte-identical to idempotency-go/processed_events.sql, created in this service's own schema.
-- The dedupe row is inserted on the SAME transaction as the funded-write, so an at-least-once
-- chain.paylink.verified redelivery applies its effect exactly once.
CREATE TABLE IF NOT EXISTS escrow.processed_events (
    scope        TEXT        NOT NULL,
    dedupe_key   TEXT        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, dedupe_key)
);
