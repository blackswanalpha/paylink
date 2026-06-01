-- work13 audit-log-service: initial schema (spec §2.17). One schema per service ("audit").
-- Append-only, tamper-evident hash chain: entry_hash = SHA256(prev_hash || canonical_json(entry)).
-- No cross-schema foreign keys — actor_id and resource are opaque references to entities owned by
-- other services (identity, merchant, paylink, payment, ...).

CREATE SCHEMA IF NOT EXISTS audit;

-- entries: the chain. entry_id is GENERATED ALWAYS AS IDENTITY so it can never be supplied or
-- overwritten by a writer (append-only id assignment). prev_hash/entry_hash are 32-byte SHA-256.
CREATE TABLE IF NOT EXISTS audit.entries (
    entry_id     bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at  timestamptz NOT NULL,
    actor_id     uuid,                                   -- nullable (service/system actors)
    actor_kind   text        NOT NULL,                   -- user|service|system
    action       text        NOT NULL,                   -- e.g. 'merchant.suspend', 'admin.search'
    resource     text        NOT NULL,                   -- canonical resource ref, e.g. 'user:<uuid>'
    before_state jsonb,
    after_state  jsonb,
    context      jsonb       NOT NULL,                    -- ip, trace_id, reason, ...
    prev_hash    bytea       NOT NULL,
    entry_hash   bytea       NOT NULL,
    -- canonical_bytes is the integrity-authoritative serialization: the exact deterministic JSON
    -- that was hashed (entry_hash = SHA256(prev_hash || canonical_bytes)). It is stored verbatim
    -- so verify recomputes the hash without re-canonicalizing the jsonb columns — Postgres jsonb
    -- normalizes numbers (1e6 → 1000000, scaled decimals), which would otherwise make a clean
    -- entry hash differently on read. The before_state/after_state/context jsonb columns are an
    -- indexed/queryable projection of the same input.
    canonical_bytes bytea    NOT NULL
);

CREATE INDEX IF NOT EXISTS audit_actor_idx ON audit.entries (actor_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS audit_resource_idx ON audit.entries (resource, occurred_at DESC);

-- anchors: Phase-2 on-chain anchoring (nightly TxAuditAnchor writes the latest entry_hash here so
-- external auditors can verify history was not rewritten). Created now as forward schema; the
-- anchoring job is out of scope for Phase 1.
CREATE TABLE IF NOT EXISTS audit.anchors (
    anchor_id       bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    anchored_at     timestamptz NOT NULL DEFAULT now(),
    last_entry_id   bigint      NOT NULL,
    last_entry_hash bytea       NOT NULL,
    chain_tx_hash   text
);
