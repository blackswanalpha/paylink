-- work02 payment-orchestrator: initial schema.
-- One schema per service ("payment"); no cross-schema foreign keys — paylink_id is an opaque
-- reference to the PayLink owned by paylink-service / the chain (id hex string).

CREATE SCHEMA IF NOT EXISTS payment;

-- payments: one orchestration record per PayLink. A PayLink settles exactly once (A.7), so
-- paylink_id is UNIQUE. status mirrors the on-chain PayLink FSM projection (lifecycle.State).
CREATE TABLE IF NOT EXISTS payment.payments (
    id              uuid        PRIMARY KEY,
    paylink_id      text        NOT NULL UNIQUE,
    rail            text        NOT NULL,
    status          text        NOT NULL,
    last_event_seq  bigint      NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS payments_status_idx ON payment.payments (status);

-- applied_chain_events: append-only anti-replay ledger. The (paylink_id, seq) primary key makes
-- duplicate chain events idempotent (A.7) and provides an audit trail of every event applied.
CREATE TABLE IF NOT EXISTS payment.applied_chain_events (
    paylink_id  text        NOT NULL,
    seq         bigint      NOT NULL,
    kind        text        NOT NULL,
    tx_hash     text        NOT NULL DEFAULT '',
    applied_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (paylink_id, seq)
);
