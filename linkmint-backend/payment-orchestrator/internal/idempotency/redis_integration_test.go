//go:build integration

// Integration tests for the go-redis adapter + Store against a real Redis.
// Run with: go test -tags=integration ./...  (requires a Docker daemon)
package idempotency

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/paylink/payment-orchestrator/internal/httpx"
)

func newRedis(t *testing.T) *RedisClient {
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

func TestRedisStoreLifecycle(t *testing.T) {
	rc := newRedis(t)
	s := New(rc, time.Hour)
	ctx := context.Background()

	// First call owns the key.
	cached, err := s.Begin(ctx, "initiate", "key-1", "fp-1")
	if err != nil || cached != nil {
		t.Fatalf("first Begin: cached=%v err=%v", cached, err)
	}
	// In-flight duplicate -> conflict.
	if _, err := s.Begin(ctx, "initiate", "key-1", "fp-1"); httpx.AsAppError(err).Code != httpx.CodeIdempotentConflict {
		t.Fatalf("expected in-flight conflict, got %v", err)
	}
	// Complete then replay.
	body := json.RawMessage(`{"id":"p1"}`)
	if err := s.Complete(ctx, "initiate", "key-1", "fp-1", 201, body); err != nil {
		t.Fatal(err)
	}
	cached, err = s.Begin(ctx, "initiate", "key-1", "fp-1")
	if err != nil || cached == nil || cached.Status != 201 || string(cached.Body) != string(body) {
		t.Fatalf("replay: cached=%v err=%v", cached, err)
	}
	// Different body -> conflict.
	if _, err := s.Begin(ctx, "initiate", "key-1", "fp-2"); httpx.AsAppError(err).Code != httpx.CodeIdempotentConflict {
		t.Fatalf("expected body-mismatch conflict, got %v", err)
	}
	// Release allows reuse.
	s.Release(ctx, "initiate", "key-1")
	if cached, err := s.Begin(ctx, "initiate", "key-1", "fp-1"); err != nil || cached != nil {
		t.Fatalf("after release: cached=%v err=%v", cached, err)
	}
	if err := s.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
