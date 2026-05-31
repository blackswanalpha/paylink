-- work03 proof-validator: initial schema.
-- One schema per service ("proof_validator"); no cross-schema foreign keys — paylink_id is an
-- opaque 0x-hex reference to the on-chain PayLink.

CREATE SCHEMA IF NOT EXISTS proof_validator;

-- proofs: one row per submitted proof. proof_hash is the on-chain anti-replay identity
-- (lvm.ProofHash(pl_id, tx_id, amount)) and is the PRIMARY KEY: a proof settles a PayLink exactly
-- once (A.7), so a duplicate submission collides here locally, complementing the on-chain check.
-- The chain remains the source of truth; this table is an audit trail + double-broadcast guard.
CREATE TABLE IF NOT EXISTS proof_validator.proofs (
    proof_hash text        PRIMARY KEY,          -- 0x + 64 hex
    paylink_id text        NOT NULL,             -- 0x + 64 hex (opaque ref)
    rail       text        NOT NULL,             -- mpesa|card|bank|crypto (A.4 label only)
    tx_id      text        NOT NULL,             -- rail transaction id
    amount     bigint      NOT NULL,             -- minor units (on-chain uint64; fits in bigint for MVP)
    status     text        NOT NULL,             -- received | broadcast | settled | already_settled
    tx_hash    text        NOT NULL DEFAULT '',  -- settlement tx hash once broadcast
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS proofs_paylink_idx ON proof_validator.proofs (paylink_id);
CREATE INDEX IF NOT EXISTS proofs_status_idx ON proof_validator.proofs (status);
