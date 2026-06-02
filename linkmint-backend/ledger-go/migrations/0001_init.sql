-- CANONICAL DDL — byte-identical copy in:
--   linkmint-backend/ledger-go/migrations/0001_init.sql
--   linkmint-backend/ledger-python/src/linkmint_ledger/migrations/0001_init.sql
-- Edit BOTH together (the ledger-python CI job diffs them).
--
-- work16 double-entry ledger (spec backendfeatures.md §4). One shared schema ("ledger"), written
-- only through the balanced posting helper and read freely by other services for reconciliation.
-- No cross-schema foreign keys — account and pl_id are opaque references to entities owned by other
-- services (paylink, payment, settlement, ...). Non-custodial (A.1): this records flows, never holds
-- funds. Append-only (A.6): corrections are NEW reversing entry groups, never edits/deletes.

CREATE SCHEMA IF NOT EXISTS ledger;

CREATE TABLE IF NOT EXISTS ledger.ledger_entries (
  id          BIGSERIAL     PRIMARY KEY,
  entry_group UUID          NOT NULL,                          -- groups the balanced DR+CR legs
  account     TEXT          NOT NULL,                          -- e.g. 'paylink:PLK...', 'treasury', 'validator:0x...'
  direction   TEXT          NOT NULL CHECK (direction IN ('DR','CR')),
  amount      NUMERIC(38,0) NOT NULL CHECK (amount > 0),       -- minor units, scale 0, strictly positive
  currency    TEXT          NOT NULL,
  pl_id       TEXT,
  note        TEXT,
  created_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ledger_account_idx ON ledger.ledger_entries (account, created_at DESC);
CREATE INDEX IF NOT EXISTS ledger_group_idx   ON ledger.ledger_entries (entry_group);

-- Append-only enforcement (A.6). No service has an UPDATE/DELETE code path; this trigger makes the
-- guarantee DB-enforced so even raw SQL, an ORM slip, or a future service cannot rewrite history.
-- Corrections are NEW reversing entry_groups. TRUNCATE is intentionally NOT blocked (admin/test
-- reset only; unreachable through the posting helper).
CREATE OR REPLACE FUNCTION ledger.reject_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'ledger.ledger_entries is append-only (A.6): use a reversing entry, never UPDATE/DELETE';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ledger_entries_append_only ON ledger.ledger_entries;
CREATE TRIGGER ledger_entries_append_only
  BEFORE UPDATE OR DELETE ON ledger.ledger_entries
  FOR EACH ROW EXECUTE FUNCTION ledger.reject_mutation();
