package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

// memRedis is an in-memory RedisLike for tests (TTL ignored; errors injectable).
type memRedis struct {
	mu       sync.Mutex
	data     map[string]string
	pingErr  error
	setNXErr error
	getErr   error
}

func newMemRedis() *memRedis { return &memRedis{data: map[string]string{}} }

func (m *memRedis) SetNX(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	if m.setNXErr != nil {
		return false, m.setNXErr
	}
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
	if m.getErr != nil {
		return "", false, m.getErr
	}
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

func TestFirstCallOwnsKey(t *testing.T) {
	s := New(newMemRedis(), "svc", time.Hour)
	cached, err := s.Begin(context.Background(), "create", "k1", "fp1")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if cached != nil {
		t.Fatal("first call should own the key (nil cached)")
	}
}

func TestReplay(t *testing.T) {
	s := New(newMemRedis(), "svc", time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "create", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	body := json.RawMessage(`{"id":"p1"}`)
	if err := s.Complete(ctx, "create", "k1", "fp1", 201, body); err != nil {
		t.Fatal(err)
	}
	cached, err := s.Begin(ctx, "create", "k1", "fp1")
	if err != nil {
		t.Fatalf("Begin replay: %v", err)
	}
	if cached == nil || cached.Status != 201 || string(cached.Body) != string(body) {
		t.Fatalf("replay mismatch: %+v", cached)
	}
}

func TestBodyMismatchConflict(t *testing.T) {
	s := New(newMemRedis(), "svc", time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "create", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	_, err := s.Begin(ctx, "create", "k1", "fp2")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	var ce *ConflictError
	if !errors.As(err, &ce) || ce.Reason != "body_mismatch" {
		t.Fatalf("want body_mismatch ConflictError, got %v", err)
	}
}

func TestInFlightConflict(t *testing.T) {
	s := New(newMemRedis(), "svc", time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "create", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	_, err := s.Begin(ctx, "create", "k1", "fp1")
	var ce *ConflictError
	if !errors.As(err, &ce) || ce.Reason != "in_flight" {
		t.Fatalf("want in_flight ConflictError, got %v", err)
	}
}

func TestReleaseAllowsRetry(t *testing.T) {
	s := New(newMemRedis(), "svc", time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "create", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	s.Release(ctx, "create", "k1")
	cached, err := s.Begin(ctx, "create", "k1", "fp1")
	if err != nil || cached != nil {
		t.Fatalf("after release, retry should own the key: cached=%v err=%v", cached, err)
	}
}

func TestRouteAndServiceNamespacing(t *testing.T) {
	s := New(newMemRedis(), "svc", time.Hour)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "create", "k1", "fp1"); err != nil {
		t.Fatal(err)
	}
	// Same key on a different route must not collide.
	if cached, err := s.Begin(ctx, "cancel", "k1", "fp1"); err != nil || cached != nil {
		t.Fatalf("different route should own the key: cached=%v err=%v", cached, err)
	}
	// The service name is part of the key.
	if got := s.key("create", "k1"); got != "idem:svc:create:k1" {
		t.Fatalf("key = %q", got)
	}
	if other := New(newMemRedis(), "other", time.Hour).key("create", "k1"); other == s.key("create", "k1") {
		t.Fatal("different services must produce different keys")
	}
}

// TestReservationExpiryRetriesAcquisition drives the narrow window where the reservation vanishes
// between SETNX and GET: the first SETNX fails (key present), the GET finds it gone, and the loop must
// retry the SETNX so the caller ends up owning the key.
func TestReservationExpiryRetriesAcquisition(t *testing.T) {
	base := newMemRedis()
	const keyName = "idem:svc:create:k1"
	base.data[keyName] = `{"state":"pending","fp":"other"}` // a reservation that will "expire"
	s := New(&dropOnGet{memRedis: base, key: keyName}, "svc", time.Hour)
	cached, err := s.Begin(context.Background(), "create", "k1", "fp1")
	if err != nil || cached != nil {
		t.Fatalf("after reservation expiry, retry should own the key: cached=%v err=%v", cached, err)
	}
}

// dropOnGet deletes key on the first Get, then delegates — simulating a TTL expiry mid-Begin.
type dropOnGet struct {
	*memRedis
	key     string
	dropped bool
}

func (d *dropOnGet) Get(ctx context.Context, key string) (string, bool, error) {
	if !d.dropped && key == d.key {
		d.dropped = true
		_ = d.memRedis.Del(ctx, key)
		return "", false, nil
	}
	return d.memRedis.Get(ctx, key)
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

func TestBackendErrorsWrapErrBackend(t *testing.T) {
	mr := newMemRedis()
	mr.setNXErr = errors.New("down")
	s := New(mr, "svc", time.Hour)
	if _, err := s.Begin(context.Background(), "create", "k", "fp"); !errors.Is(err, ErrBackend) {
		t.Fatalf("want ErrBackend, got %v", err)
	}
}

func TestPing(t *testing.T) {
	mr := newMemRedis()
	s := New(mr, "svc", time.Hour)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("Ping ok: %v", err)
	}
	mr.pingErr = errors.New("down")
	if err := s.Ping(context.Background()); !errors.Is(err, ErrBackend) {
		t.Fatal("Ping should wrap ErrBackend when backend is down")
	}
}
