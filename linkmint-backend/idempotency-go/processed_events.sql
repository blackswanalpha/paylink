-- Canonical consumer-dedupe table for DbDedupe (work17). Create ONE per service, in the service's OWN
-- schema (no cross-schema references — one schema per service). Fold this into a numbered migration
-- (the service's SQL migrate for Go; Alembic for Python). The (scope, dedupe_key) primary key makes a
-- repeated event a no-op INSERT (ON CONFLICT DO NOTHING), so an at-least-once redelivery applies its
-- effect exactly once when the row is written on the same transaction as that effect.
CREATE TABLE IF NOT EXISTS processed_events (
    scope        TEXT        NOT NULL,
    dedupe_key   TEXT        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, dedupe_key)
);
