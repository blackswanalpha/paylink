package idempotency

import (
	"context"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// RedisDedupe is a best-effort consumer-side dedupe guard backed by Redis SETNX. Use it to skip the
// expensive part of an at-least-once event handler when the same event — identified by a caller-chosen
// stable dedupe key (an event id, a proof_hash, or a payload fingerprint) — has already been handled.
//
// It is a CHEAP PRE-FILTER, not a durable guarantee: the marker has a TTL and Redis can evict it, so a
// very-late redelivery may slip through. For a true exactly-once *effect*, back the handler's write
// with a DB UNIQUE constraint (see DbDedupe) — the Redis guard then just avoids repeating non-DB work.
type RedisDedupe struct {
	redis   RedisLike
	service string
	ttl     time.Duration
}

// NewRedisDedupe builds a RedisDedupe for the named service with the given marker TTL.
func NewRedisDedupe(redis RedisLike, service string, ttl time.Duration) *RedisDedupe {
	return &RedisDedupe{redis: redis, service: service, ttl: ttl}
}

func (d *RedisDedupe) key(scope, dedupeKey string) string {
	return "idemc:" + d.service + ":" + scope + ":" + dedupeKey
}

// SeenBefore reports whether dedupeKey was already marked under scope, without claiming it.
func (d *RedisDedupe) SeenBefore(ctx context.Context, scope, dedupeKey string) (bool, error) {
	_, found, err := d.redis.Get(ctx, d.key(scope, dedupeKey))
	if err != nil {
		return false, backendErr(err)
	}
	return found, nil
}

// RunOnce runs action at most once per (scope, dedupeKey): the first caller wins the SETNX marker and
// runs action; a later caller with the same key is skipped. If action returns an error the marker is
// removed so the redelivered event can retry (no poison-lock), mirroring the bus's commit-after-handle
// contract. Returns nil when action ran cleanly OR was skipped as a duplicate; returns action's error
// (or a backend error) otherwise.
func (d *RedisDedupe) RunOnce(ctx context.Context, scope, dedupeKey string, action func() error) error {
	rkey := d.key(scope, dedupeKey)
	reserved, err := d.redis.SetNX(ctx, rkey, "1", d.ttl)
	if err != nil {
		return backendErr(err)
	}
	if !reserved {
		return nil // already handled (or in flight) — skip
	}
	if err := action(); err != nil {
		// Roll the marker back so the redelivered event retries. Cancel-detached so cleanup still runs
		// if the caller's ctx was cancelled.
		_ = d.redis.Del(context.WithoutCancel(ctx), rkey)
		return err
	}
	return nil
}

// DBTX is the subset of pgx used by DbDedupe. *pgxpool.Pool, *pgx.Conn, and pgx.Tx all satisfy it, so
// the dedupe row is written on the caller's OWN transaction — the mark and the handler's business write
// then commit together or roll back together (mirrors the ledger-go DBTX convention).
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// safeIdent permits a plain or schema-qualified SQL identifier and nothing else.
var safeIdent = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)

// DbDedupe is a durable consumer-side dedupe guard backed by a Postgres table (default
// "processed_events"; create it from the shipped processed_events.sql in the service's own schema).
// Unlike RedisDedupe it gives a true exactly-once *effect*: the dedupe row is inserted on the SAME
// transaction as the handler's write, so the mark and the effect are atomic and survive Redis loss.
type DbDedupe struct {
	table string
}

// NewDbDedupe builds a DbDedupe writing to table (pass "" for the default "processed_events"). A name
// that is not a plain/`schema.table` identifier falls back to the default — the table is interpolated
// into SQL (identifiers can't be parameterized), so it must be a trusted constant, never user input.
func NewDbDedupe(table string) *DbDedupe {
	if !safeIdent.MatchString(table) {
		table = "processed_events"
	}
	return &DbDedupe{table: table}
}

// RunOnce inserts (scope, dedupeKey) on db and runs action only when the row is new. If the event was
// processed before (the row already exists) it returns (false, nil) WITHOUT running action. The caller
// commits db; the dedupe mark and action's writes commit together, so a redelivery never re-applies the
// effect. Returns (true, nil) when action ran, (false, nil) on a duplicate, or (_, err) on failure.
func (d *DbDedupe) RunOnce(ctx context.Context, db DBTX, scope, dedupeKey string, action func() error) (bool, error) {
	tag, err := db.Exec(ctx,
		"INSERT INTO "+d.table+" (scope, dedupe_key) VALUES ($1, $2) ON CONFLICT (scope, dedupe_key) DO NOTHING",
		scope, dedupeKey)
	if err != nil {
		return false, backendErr(err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil // duplicate — already processed
	}
	if err := action(); err != nil {
		return false, err
	}
	return true, nil
}
