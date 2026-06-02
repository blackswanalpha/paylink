package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestRedisDedupeRunsOnceThenSkips(t *testing.T) {
	d := NewRedisDedupe(newMemRedis(), "svc", time.Hour)
	ctx := context.Background()
	calls := 0
	act := func() error { calls++; return nil }
	for i := 0; i < 3; i++ {
		if err := d.RunOnce(ctx, "proof", "h1", act); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("action ran %d times, want 1", calls)
	}
}

func TestRedisDedupeErrorRollsBackMarker(t *testing.T) {
	d := NewRedisDedupe(newMemRedis(), "svc", time.Hour)
	ctx := context.Background()
	boom := errors.New("boom")
	if err := d.RunOnce(ctx, "proof", "h1", func() error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	// Marker rolled back -> a redelivery retries (action runs again, this time cleanly).
	calls := 0
	if err := d.RunOnce(ctx, "proof", "h1", func() error { calls++; return nil }); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("after rollback, action ran %d times, want 1", calls)
	}
}

func TestRedisDedupeSeenBefore(t *testing.T) {
	d := NewRedisDedupe(newMemRedis(), "svc", time.Hour)
	ctx := context.Background()
	if seen, _ := d.SeenBefore(ctx, "proof", "h1"); seen {
		t.Fatal("should not be seen yet")
	}
	_ = d.RunOnce(ctx, "proof", "h1", func() error { return nil })
	if seen, _ := d.SeenBefore(ctx, "proof", "h1"); !seen {
		t.Fatal("should be seen after RunOnce")
	}
}

func TestRedisDedupeBackendError(t *testing.T) {
	mr := newMemRedis()
	mr.setNXErr = errors.New("down")
	d := NewRedisDedupe(mr, "svc", time.Hour)
	if err := d.RunOnce(context.Background(), "proof", "h1", func() error { return nil }); !errors.Is(err, ErrBackend) {
		t.Fatalf("want ErrBackend, got %v", err)
	}
	mr2 := newMemRedis()
	mr2.getErr = errors.New("down")
	d2 := NewRedisDedupe(mr2, "svc", time.Hour)
	if _, err := d2.SeenBefore(context.Background(), "proof", "h1"); !errors.Is(err, ErrBackend) {
		t.Fatalf("SeenBefore want ErrBackend, got %v", err)
	}
}

// fakeDB is an in-memory DBTX recording inserted (scope,key) pairs, emulating ON CONFLICT DO NOTHING.
type fakeDB struct {
	seen    map[string]bool
	execErr error
}

func newFakeDB() *fakeDB { return &fakeDB{seen: map[string]bool{}} }

func (f *fakeDB) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}
	k := args[0].(string) + "\x00" + args[1].(string)
	if f.seen[k] {
		return pgconn.NewCommandTag("INSERT 0 0"), nil
	}
	f.seen[k] = true
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func TestDbDedupeRunsOnceThenSkips(t *testing.T) {
	d := NewDbDedupe("")
	db := newFakeDB()
	ctx := context.Background()
	calls := 0
	ran, err := d.RunOnce(ctx, db, "proof", "h1", func() error { calls++; return nil })
	if err != nil || !ran {
		t.Fatalf("first delivery: ran=%v err=%v (want true,nil)", ran, err)
	}
	ran, err = d.RunOnce(ctx, db, "proof", "h1", func() error { calls++; return nil })
	if err != nil || ran {
		t.Fatalf("redelivery: ran=%v err=%v (want false,nil)", ran, err)
	}
	if calls != 1 {
		t.Fatalf("action ran %d times, want 1", calls)
	}
}

func TestDbDedupeBackendError(t *testing.T) {
	d := NewDbDedupe("processed_events")
	db := newFakeDB()
	db.execErr = errors.New("down")
	if _, err := d.RunOnce(context.Background(), db, "s", "k", func() error { return nil }); !errors.Is(err, ErrBackend) {
		t.Fatalf("want ErrBackend, got %v", err)
	}
}

func TestNewDbDedupeValidatesTable(t *testing.T) {
	if got := NewDbDedupe("bad; DROP TABLE x").table; got != "processed_events" {
		t.Fatalf("unsafe table should fall back to default, got %q", got)
	}
	if got := NewDbDedupe("svc.processed_events").table; got != "svc.processed_events" {
		t.Fatalf("schema-qualified table should be kept, got %q", got)
	}
	if got := NewDbDedupe("").table; got != "processed_events" {
		t.Fatalf("empty table should default, got %q", got)
	}
}
