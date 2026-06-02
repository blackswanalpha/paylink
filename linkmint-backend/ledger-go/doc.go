// Package ledger is LinkMint's shared double-entry ledger helper (work16): the append-only
// ledger.ledger_entries table plus balanced posting/reversal/read helpers used (read/write) by
// services to record every monetary flow for reconciliation and reporting.
//
// Invariant A.6 (double-entry, append-only): every posting is a balanced set of legs — the DR total
// equals the CR total per currency under one entry_group, written in a single statement. The table
// is append-only; a DB trigger rejects UPDATE/DELETE, so corrections are NEW reversing entry groups
// (see Reverse), never edits. The Python counterpart (linkmint_ledger) ships a byte-identical schema
// migration so both languages post into the same shape.
//
// The helpers take a DBTX (a *pgxpool.Pool, *pgx.Conn, or pgx.Tx) and never open or commit their
// own transaction: a caller posts ledger legs on the SAME transaction as its business-state change,
// so the two commit together or not at all. The ledger is non-custodial (A.1) — it RECORDS flows
// between opaque account labels (e.g. "paylink:PLK...", "treasury", "validator:0x..."); it never
// holds or moves funds. Amounts are minor units (NUMERIC(38,0), strictly positive); balances are
// read-only SUMs (Balance = ΣCR − ΣDR for an account+currency).
package ledger
