package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/paylink/payment-orchestrator/internal/httpx"
)

// memRedis is an in-memory RedisLike for tests (TTL ignored; pingErr injectable).
type memRedis struct {
	mu      sync.Mutex
	data    map[string]string
	pingErr error
}

func newMemRedis() *memRedis { return &memRedis{data: map[string]string{}} }

func (m *memRedis) SetNX(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.data[key] = value
	return true, nil
}

func (m *memRedis) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *memRedis) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	return v, ok, nil
}

func (m *memRedis) Del(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *memRedis) Ping(_ context.Context) error { return m.pingErr }

func appErrCode(t *testing.T, err error) httpx.ErrorCode {
	t.Helper()
	var ae *httpx.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *httpx.AppError, got %v", err)
	}
	return ae.Code
}

func TestIdempotencyFirstCallOwnsKey(t *testing.T) {
	s := New(newMemRedis(), time.Hour)
	ctx := context.Background()
	cached, err := s.Begin(ctx, "initiate", "k1", "fp1")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if cached != nil {
		t.Fatal("first call should own the key (nil cached)")
	}
}

func TestIdempotencyReplay(t *testing.T) {
	s := New(newMemRedis(), time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "initiate", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	body := json.RawMessage(`{"id":"p1"}`)
	if err := s.Complete(ctx, "initiate", "k1", "fp1", 201, body); err != nil {
		t.Fatal(err)
	}
	cached, err := s.Begin(ctx, "initiate", "k1", "fp1")
	if err != nil {
		t.Fatalf("Begin replay: %v", err)
	}
	if cached == nil || cached.Status != 201 || string(cached.Body) != string(body) {
		t.Fatalf("replay mismatch: %+v", cached)
	}
}

func TestIdempotencyBodyMismatchConflict(t *testing.T) {
	s := New(newMemRedis(), time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "initiate", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	_, err := s.Begin(ctx, "initiate", "k1", "fp2")
	if code := appErrCode(t, err); code != httpx.CodeIdempotentConflict {
		t.Fatalf("want IDEMPOTENT_CONFLICT, got %s", code)
	}
}

func TestIdempotencyInFlightConflict(t *testing.T) {
	s := New(newMemRedis(), time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "initiate", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	// same key + body, but not yet completed -> in-flight conflict
	_, err := s.Begin(ctx, "initiate", "k1", "fp1")
	if code := appErrCode(t, err); code != httpx.CodeIdempotentConflict {
		t.Fatalf("want IDEMPOTENT_CONFLICT, got %s", code)
	}
}

func TestIdempotencyReleaseAllowsRetry(t *testing.T) {
	s := New(newMemRedis(), time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "initiate", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	s.Release(ctx, "initiate", "k1")
	cached, err := s.Begin(ctx, "initiate", "k1", "fp1")
	if err != nil || cached != nil {
		t.Fatalf("after release, retry should own the key: cached=%v err=%v", cached, err)
	}
}

func TestIdempotencyRouteNamespacing(t *testing.T) {
	s := New(newMemRedis(), time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "initiate", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	// Same key on a different route must not collide.
	cached, err := s.Begin(ctx, "cancel", "k1", "fp1")
	if err != nil || cached != nil {
		t.Fatalf("different route should own the key: cached=%v err=%v", cached, err)
	}
}

func TestFingerprintStable(t *testing.T) {
	a := Fingerprint([]byte(`{"a":1}`))
	b := Fingerprint([]byte(`{"a":1}`))
	c := Fingerprint([]byte(`{"a":2}`))
	if a != b {
		t.Error("same bytes should fingerprint equal")
	}
	if a == c {
		t.Error("different bytes should fingerprint differently")
	}
}

func TestPing(t *testing.T) {
	mr := newMemRedis()
	s := New(mr, time.Hour)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("Ping ok: %v", err)
	}
	mr.pingErr = errors.New("down")
	if err := s.Ping(context.Background()); err == nil {
		t.Fatal("Ping should fail when backend is down")
	}
}
