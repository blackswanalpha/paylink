package idempotency_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/paylink/proof-validator/internal/httpx"
	"github.com/paylink/proof-validator/internal/idempotency"
)

// fakeRedis is an in-memory RedisLike for tests.
type fakeRedis struct {
	mu sync.Mutex
	m  map[string]string
}

func newFakeRedis() *fakeRedis { return &fakeRedis{m: map[string]string{}} }

func (f *fakeRedis) SetNX(_ context.Context, k, v string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.m[k]; ok {
		return false, nil
	}
	f.m[k] = v
	return true, nil
}
func (f *fakeRedis) Set(_ context.Context, k, v string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[k] = v
	return nil
}
func (f *fakeRedis) Get(_ context.Context, k string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.m[k]
	return v, ok, nil
}
func (f *fakeRedis) Del(_ context.Context, k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, k)
	return nil
}
func (f *fakeRedis) Ping(context.Context) error { return nil }

func TestIdempotency_ReserveCompleteReplay(t *testing.T) {
	s := idempotency.New(newFakeRedis(), time.Hour)
	ctx := context.Background()
	fp := idempotency.Fingerprint([]byte(`{"a":1}`))

	cached, err := s.Begin(ctx, "route", "key1", fp)
	if err != nil || cached != nil {
		t.Fatalf("first Begin should reserve (nil,nil); got %+v,%v", cached, err)
	}
	if err := s.Complete(ctx, "route", "key1", fp, 202, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	cached, err = s.Begin(ctx, "route", "key1", fp)
	if err != nil {
		t.Fatalf("replay Begin: %v", err)
	}
	if cached == nil || cached.Status != 202 || string(cached.Body) != `{"ok":true}` {
		t.Fatalf("replay should return the cached response; got %+v", cached)
	}
}

func TestIdempotency_BodyConflict(t *testing.T) {
	s := idempotency.New(newFakeRedis(), time.Hour)
	ctx := context.Background()
	_, _ = s.Begin(ctx, "route", "key1", idempotency.Fingerprint([]byte("body-A")))
	_, err := s.Begin(ctx, "route", "key1", idempotency.Fingerprint([]byte("body-B")))
	if httpx.AsAppError(err).Code != httpx.CodeIdempotentConflict {
		t.Fatalf("expected IDEMPOTENT_CONFLICT, got %v", err)
	}
}

func TestIdempotency_InProgressConflict(t *testing.T) {
	s := idempotency.New(newFakeRedis(), time.Hour)
	ctx := context.Background()
	fp := idempotency.Fingerprint([]byte("body"))
	_, _ = s.Begin(ctx, "route", "key1", fp) // reserved, not completed
	_, err := s.Begin(ctx, "route", "key1", fp)
	if httpx.AsAppError(err).Code != httpx.CodeIdempotentConflict {
		t.Fatalf("expected in-progress IDEMPOTENT_CONFLICT, got %v", err)
	}
}

func TestIdempotency_ReleaseAllowsRetry(t *testing.T) {
	s := idempotency.New(newFakeRedis(), time.Hour)
	ctx := context.Background()
	fp := idempotency.Fingerprint([]byte("body"))
	_, _ = s.Begin(ctx, "route", "key1", fp)
	s.Release(ctx, "route", "key1")
	cached, err := s.Begin(ctx, "route", "key1", fp)
	if err != nil || cached != nil {
		t.Fatalf("after Release, Begin should reserve again; got %+v,%v", cached, err)
	}
}

func TestIdempotency_Ping(t *testing.T) {
	if err := idempotency.New(newFakeRedis(), time.Hour).Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
