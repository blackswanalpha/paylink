// Package postgres is the production domain.Store backed by PostgreSQL (pgx). Appends are
// serialized by a transaction-scoped advisory lock so prev_hash always links to the true tail
// (the audit-log analogue of the orchestrator's SELECT ... FOR UPDATE — there is no single tail
// row to lock, so a named advisory lock serializes the whole chain).
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/paylink/audit-log-service/internal/domain"
)

// appendLockKey is the fixed advisory-lock key serializing all appends to the single chain.
const appendLockKey int64 = 0x41554449545f4c47 // ASCII "AUDIT_LG"

// Store is a pgx-backed domain.Store.
type Store struct {
	pool *pgxpool.Pool
}

// New connects a pool to the given DSN.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks DB connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// auditColumns casts uuid/jsonb to text so the values scan cleanly into sql.NullString / []byte
// regardless of pgx codec defaults. The hash is recomputed from these (canonical re-normalizes the
// jsonb text), so PG's jsonb normalization does not affect verification for non-float payloads.
const auditColumns = `entry_id, occurred_at, actor_id::text, actor_kind, action, resource, ` +
	`before_state::text, after_state::text, context::text, prev_hash, entry_hash, canonical_bytes`

type scanner interface{ Scan(dest ...any) error }

func scanEntry(r scanner) (domain.Entry, error) {
	var (
		e       domain.Entry
		actorID sql.NullString
		kind    string
		before  []byte
		after   []byte
		ctxb    []byte
	)
	if err := r.Scan(&e.EntryID, &e.OccurredAt, &actorID, &kind, &e.Action, &e.Resource,
		&before, &after, &ctxb, &e.PrevHash, &e.EntryHash, &e.Canonical); err != nil {
		return domain.Entry{}, err
	}
	e.Actor.Kind = domain.ActorKind(kind)
	if actorID.Valid {
		u, err := uuid.Parse(actorID.String)
		if err != nil {
			return domain.Entry{}, fmt.Errorf("parse actor_id %q: %w", actorID.String, err)
		}
		e.Actor.ID = &u
	}
	e.Before = rawOrNil(before)
	e.After = rawOrNil(after)
	e.Context = json.RawMessage(ctxb)
	return e, nil
}

func rawOrNil(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

// Append serializes against concurrent appends, links to the tail, hashes, and inserts.
func (s *Store) Append(ctx context.Context, in domain.AppendInput) (domain.Entry, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Entry{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	// Serialize all appends — released automatically on COMMIT/ROLLBACK.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, appendLockKey); err != nil {
		return domain.Entry{}, err
	}

	prev := domain.GenesisHash()
	var tail []byte
	err = tx.QueryRow(ctx, `SELECT entry_hash FROM audit.entries ORDER BY entry_id DESC LIMIT 1`).Scan(&tail)
	switch {
	case err == nil:
		prev = tail
	case errors.Is(err, pgx.ErrNoRows):
		// first entry — prev stays genesis
	default:
		return domain.Entry{}, err
	}

	e, err := domain.BuildEntry(in, prev)
	if err != nil {
		return domain.Entry{}, err
	}

	if err := tx.QueryRow(ctx,
		`INSERT INTO audit.entries
		   (occurred_at, actor_id, actor_kind, action, resource, before_state, after_state, context, prev_hash, entry_hash, canonical_bytes)
		 VALUES ($1, $2::uuid, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9, $10, $11)
		 RETURNING entry_id`,
		e.OccurredAt, actorIDArg(e.Actor), string(e.Actor.Kind), e.Action, e.Resource,
		jsonbArg(e.Before), jsonbArg(e.After), jsonbArg(e.Context), e.PrevHash, e.EntryHash, e.Canonical,
	).Scan(&e.EntryID); err != nil {
		return domain.Entry{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Entry{}, err
	}
	return e, nil
}

func actorIDArg(a domain.Actor) any {
	if a.ID == nil {
		return nil
	}
	return a.ID.String()
}

func jsonbArg(raw json.RawMessage) any {
	t := strings.TrimSpace(string(raw))
	if len(t) == 0 || t == "null" {
		return nil
	}
	return t
}

// GetByID returns the entry by id, or domain.ErrNotFound.
func (s *Store) GetByID(ctx context.Context, id int64) (domain.Entry, error) {
	e, err := scanEntry(s.pool.QueryRow(ctx, `SELECT `+auditColumns+` FROM audit.entries WHERE entry_id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Entry{}, domain.ErrNotFound
	}
	return e, err
}

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return 20
	case n > 100:
		return 100
	default:
		return n
	}
}

// Query returns a newest-first page (entry_id DESC) matching the filter, with a cursor for the
// next page. The cursor is the smallest entry_id in the page, applied as entry_id < cursor.
func (s *Store) Query(ctx context.Context, f domain.QueryFilter) (domain.Page, error) {
	limit := clampLimit(f.Limit)
	var clauses []string
	var args []any
	add := func(format string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(format, len(args)))
	}
	if f.Actor != nil {
		add("actor_id = $%d::uuid", f.Actor.String())
	}
	if f.Resource != "" {
		add("resource = $%d", f.Resource)
	}
	if f.From != nil {
		add("occurred_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("occurred_at <= $%d", *f.To)
	}
	if f.Cursor > 0 {
		add("entry_id < $%d", f.Cursor)
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, limit+1) // fetch one extra to detect a next page
	q := `SELECT ` + auditColumns + ` FROM audit.entries ` + where +
		fmt.Sprintf(` ORDER BY entry_id DESC LIMIT $%d`, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return domain.Page{}, err
	}
	defer rows.Close()
	items := make([]domain.Entry, 0, limit)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return domain.Page{}, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return domain.Page{}, err
	}
	var next *int64
	if len(items) > limit {
		items = items[:limit]
		c := items[len(items)-1].EntryID
		next = &c
	}
	return domain.Page{Items: items, NextCursor: next}, nil
}

// VerifyRange verifies the contiguous chain segment [startID, endID], where startID is the smallest
// entry_id with occurred_at >= from and endID the largest with occurred_at <= to (full chain when
// both nil). Verifying a contiguous-by-entry_id segment (not an occurred_at-filtered set with holes)
// is what hash-chain integrity requires: a deleted/edited middle entry is detected via the linkage
// of the entry that follows it. Linkage is seeded from the entry immediately preceding startID.
func (s *Store) VerifyRange(ctx context.Context, from, to *time.Time) (domain.VerifyResult, error) {
	startID, ok, err := s.boundID(ctx, "min", "occurred_at >= $1", from)
	if err != nil {
		return domain.VerifyResult{}, err
	}
	if !ok {
		return domain.VerifyResult{OK: true}, nil
	}
	endID, ok, err := s.boundID(ctx, "max", "occurred_at <= $1", to)
	if err != nil {
		return domain.VerifyResult{}, err
	}
	if !ok || endID < startID {
		return domain.VerifyResult{OK: true}, nil
	}

	expected := domain.GenesisHash()
	var predHash []byte
	switch err := s.pool.QueryRow(ctx,
		`SELECT entry_hash FROM audit.entries WHERE entry_id < $1 ORDER BY entry_id DESC LIMIT 1`, startID).
		Scan(&predHash); {
	case err == nil:
		expected = predHash
	case errors.Is(err, pgx.ErrNoRows):
		// startID is the chain head — seed from genesis
	default:
		return domain.VerifyResult{}, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT `+auditColumns+` FROM audit.entries WHERE entry_id >= $1 AND entry_id <= $2 ORDER BY entry_id ASC`,
		startID, endID)
	if err != nil {
		return domain.VerifyResult{}, err
	}
	defer rows.Close()
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return domain.VerifyResult{}, err
		}
		selfOK, linkOK := domain.CheckEntry(e, expected)
		if !selfOK || !linkOK {
			id := e.EntryID
			return domain.VerifyResult{OK: false, BrokenAt: &id}, nil
		}
		expected = e.EntryHash
	}
	if err := rows.Err(); err != nil {
		return domain.VerifyResult{}, err
	}
	return domain.VerifyResult{OK: true}, nil
}

// boundID returns min/max entry_id subject to an optional time predicate. ok=false when no row
// matches (min/max over an empty set is NULL).
func (s *Store) boundID(ctx context.Context, agg, pred string, t *time.Time) (int64, bool, error) {
	q := `SELECT ` + agg + `(entry_id) FROM audit.entries`
	var args []any
	if t != nil {
		q += ` WHERE ` + pred
		args = append(args, *t)
	}
	var id *int64
	if err := s.pool.QueryRow(ctx, q, args...).Scan(&id); err != nil {
		return 0, false, err
	}
	if id == nil {
		return 0, false, nil
	}
	return *id, true, nil
}

// Tail returns the head entry_hash (genesis when empty) and the entry count.
func (s *Store) Tail(ctx context.Context) ([]byte, int64, error) {
	var count int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit.entries`).Scan(&count); err != nil {
		return nil, 0, err
	}
	if count == 0 {
		return domain.GenesisHash(), 0, nil
	}
	var hash []byte
	if err := s.pool.QueryRow(ctx, `SELECT entry_hash FROM audit.entries ORDER BY entry_id DESC LIMIT 1`).Scan(&hash); err != nil {
		return nil, 0, err
	}
	return hash, count, nil
}
