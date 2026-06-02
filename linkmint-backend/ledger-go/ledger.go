package ledger

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the subset of pgx used by the ledger helpers. *pgxpool.Pool, *pgx.Conn, and pgx.Tx all
// satisfy it, so a caller can post ledger legs on its OWN transaction — the business-state write and
// the ledger write then commit together (A.6) or roll back together. The helpers never Begin/Commit;
// the caller owns the unit of work.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// selectColumns casts uuid/numeric to text so values scan cleanly into Go types regardless of pgx
// codec defaults (the repo convention — see audit-log-service). amount::text round-trips the full
// NUMERIC(38,0) into *big.Int without a numeric codec.
const selectColumns = `id, entry_group::text, account, direction, amount::text, currency, pl_id, note, created_at`

// Post validates a balanced set of legs and writes them atomically under one entry_group. The legs
// are inserted in a single statement (atomic), so a group is all-or-nothing even on a pool; when db
// is a caller's pgx.Tx the legs join that transaction. Returns the entry_group. Rejects unbalanced
// or malformed input (ErrUnbalanced / ErrInvalidLeg) before touching the database.
func Post(ctx context.Context, db DBTX, in PostingInput) (uuid.UUID, error) {
	if err := validate(in.Entries); err != nil {
		return uuid.Nil, err
	}

	group := uuid.New()
	if in.EntryGroup != nil {
		group = *in.EntryGroup
	}
	plID := nullIfEmpty(in.PLID)
	note := nullIfEmpty(in.Note)

	var b strings.Builder
	b.WriteString(`INSERT INTO ledger.ledger_entries (entry_group, account, direction, amount, currency, pl_id, note) VALUES `)
	args := make([]any, 0, len(in.Entries)*7)
	for i, leg := range in.Entries {
		if i > 0 {
			b.WriteByte(',')
		}
		n := i * 7
		fmt.Fprintf(&b, "($%d::uuid,$%d,$%d,$%d::numeric,$%d,$%d,$%d)", n+1, n+2, n+3, n+4, n+5, n+6, n+7)
		args = append(args, group.String(), leg.Account, string(leg.Direction), leg.Amount.String(), leg.Currency, plID, note)
	}

	if _, err := db.Exec(ctx, b.String(), args...); err != nil {
		return uuid.Nil, fmt.Errorf("ledger post: %w", err)
	}
	return group, nil
}

// Reverse posts a correcting entry group: it reads the original group's legs and writes a NEW group
// with every direction flipped (DR↔CR), same amounts/currency/account/pl_id. The original is never
// edited or deleted (A.6). Returns the new entry_group. note defaults to "reversal of <group>".
func Reverse(ctx context.Context, db DBTX, group uuid.UUID, note string) (uuid.UUID, error) {
	entries, err := EntriesByGroup(ctx, db, group)
	if err != nil {
		return uuid.Nil, err
	}
	if len(entries) == 0 {
		return uuid.Nil, fmt.Errorf("%w: %s", ErrGroupNotFound, group)
	}

	flipped := make([]Leg, len(entries))
	plID := ""
	for i, e := range entries {
		flipped[i] = Leg{Account: e.Account, Direction: flip(e.Direction), Amount: e.Amount, Currency: e.Currency}
		if plID == "" && e.PLID != "" {
			// First non-empty pl_id wins — matches the Python reverse() (next(...)). A group's legs
			// all share one pl_id, so this only ever differs for externally-corrupted data.
			plID = e.PLID
		}
	}
	if note == "" {
		note = "reversal of " + group.String()
	}
	return Post(ctx, db, PostingInput{Entries: flipped, PLID: plID, Note: note})
}

// EntriesByGroup returns all legs of an entry_group, oldest id first.
func EntriesByGroup(ctx context.Context, db DBTX, group uuid.UUID) ([]Entry, error) {
	rows, err := db.Query(ctx,
		`SELECT `+selectColumns+` FROM ledger.ledger_entries WHERE entry_group=$1::uuid ORDER BY id`,
		group.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// EntriesByAccount returns the most recent legs for an account (newest first), capped by limit
// (default 100, max 1000). For reporting/reconciliation reads.
func EntriesByAccount(ctx context.Context, db DBTX, account string, limit int) ([]Entry, error) {
	rows, err := db.Query(ctx,
		`SELECT `+selectColumns+` FROM ledger.ledger_entries WHERE account=$1 ORDER BY created_at DESC, id DESC LIMIT $2`,
		account, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// EntriesByPLID returns the most recent legs tied to a pl_id (newest first), capped by limit.
func EntriesByPLID(ctx context.Context, db DBTX, plID string, limit int) ([]Entry, error) {
	rows, err := db.Query(ctx,
		`SELECT `+selectColumns+` FROM ledger.ledger_entries WHERE pl_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2`,
		plID, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// Balance returns the net balance of an account in a currency as ΣCR − ΣDR (the credit-positive
// convention). A read-only aggregate — it never moves funds (A.1).
func Balance(ctx context.Context, db DBTX, account, currency string) (*big.Int, error) {
	var s string
	if err := db.QueryRow(ctx,
		`SELECT COALESCE(SUM(CASE WHEN direction='CR' THEN amount ELSE -amount END), 0)::text
		   FROM ledger.ledger_entries WHERE account=$1 AND currency=$2`,
		account, currency).Scan(&s); err != nil {
		return nil, err
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("ledger: unparseable balance %q for %s/%s", s, account, currency)
	}
	return v, nil
}

// IsBalanced reports whether the whole ledger nets to zero for a currency (ΣDR == ΣCR across every
// account). The global double-entry invariant work27 reconciliation checks.
func IsBalanced(ctx context.Context, db DBTX, currency string) (bool, error) {
	var s string
	if err := db.QueryRow(ctx,
		`SELECT COALESCE(SUM(CASE WHEN direction='DR' THEN amount ELSE -amount END), 0)::text
		   FROM ledger.ledger_entries WHERE currency=$1`,
		currency).Scan(&s); err != nil {
		return false, err
	}
	return s == "0", nil
}

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return 100
	case n > 1000:
		return 1000
	default:
		return n
	}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanEntries(rows pgx.Rows) ([]Entry, error) {
	var out []Entry
	for rows.Next() {
		var (
			e       Entry
			groupS  string
			dir     string
			amountS string
			plID    *string
			note    *string
		)
		if err := rows.Scan(&e.ID, &groupS, &e.Account, &dir, &amountS, &e.Currency, &plID, &note, &e.CreatedAt); err != nil {
			return nil, err
		}
		g, err := uuid.Parse(groupS)
		if err != nil {
			return nil, fmt.Errorf("ledger: unparseable entry_group %q: %w", groupS, err)
		}
		amt, ok := new(big.Int).SetString(amountS, 10)
		if !ok {
			return nil, fmt.Errorf("ledger: unparseable amount %q for entry %d", amountS, e.ID)
		}
		e.EntryGroup = g
		e.Direction = Direction(dir)
		e.Amount = amt
		if plID != nil {
			e.PLID = *plID
		}
		if note != nil {
			e.Note = *note
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
