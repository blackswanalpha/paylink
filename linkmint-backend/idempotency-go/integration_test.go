//go:build integration

// Integration tests for the go-redis adapter, Store, and the consumer-dedupe helpers against a real
// Redis and Postgres. Run with: go test -tags=integration ./...  (requires a Docker daemon)
package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

func newRedisClient(t *testing.T) *RedisClient {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcredis.Run(ctx, "redis:7",
		testcontainers.WithWaitStrategy(wait.ForLog("Ready to accept connections").WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })
	url, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("conn string: %v", err)
	}
	rc, err := NewRedisClient(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	if err := rc.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return rc
}

func TestRedisStoreLifecycleIntegration(t *testing.T) {
	rc := newRedisClient(t)
	s := New(rc, "svc", time.Hour)
	ctx := context.Background()

	cached, err := s.Begin(ctx, "create", "key-1", "fp-1")
	if err != nil || cached != nil {
		t.Fatalf("first Begin: cached=%v err=%v", cached, err)
	}
	if _, err := s.Begin(ctx, "create", "key-1", "fp-1"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected in-flight conflict, got %v", err)
	}
	body := json.RawMessage(`{"id":"p1"}`)
	if err := s.Complete(ctx, "create", "key-1", "fp-1", 201, body); err != nil {
		t.Fatal(err)
	}
	cached, err = s.Begin(ctx, "create", "key-1", "fp-1")
	if err != nil || cached == nil || cached.Status != 201 || string(cached.Body) != string(body) {
		t.Fatalf("replay: cached=%v err=%v", cached, err)
	}
	if _, err := s.Begin(ctx, "create", "key-1", "fp-2"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected body-mismatch conflict, got %v", err)
	}
	s.Release(ctx, "create", "key-1")
	if cached, err := s.Begin(ctx, "create", "key-1", "fp-1"); err != nil || cached != nil {
		t.Fatalf("after release: cached=%v err=%v", cached, err)
	}
}

func TestRedisDedupeIntegration(t *testing.T) {
	d := NewRedisDedupe(newRedisClient(t), "svc", time.Hour)
	ctx := context.Background()
	calls := 0
	for i := 0; i < 3; i++ {
		if err := d.RunOnce(ctx, "proof", "h1", func() error { calls++; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("action ran %d times across 3 deliveries, want 1", calls)
	}
}

func TestDbDedupeIntegration(t *testing.T) {
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `CREATE TABLE processed_events (
		scope text NOT NULL, dedupe_key text NOT NULL,
		processed_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (scope, dedupe_key))`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	d := NewDbDedupe("processed_events")
	calls := 0
	// Each delivery handled in its OWN transaction (mirrors a consumer handling one event per tx).
	deliver := func() error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit
		if _, err := d.RunOnce(ctx, tx, "proof", "h1", func() error { calls++; return nil }); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	for i := 0; i < 3; i++ {
		if err := deliver(); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("action ran %d times across 3 deliveries, want 1", calls)
	}
}
