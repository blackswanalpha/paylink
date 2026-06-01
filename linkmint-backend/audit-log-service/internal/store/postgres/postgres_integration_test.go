//go:build integration

// Integration tests for the pgx-backed store. Run with: go test -tags=integration ./...
// Requires a Docker daemon (testcontainers spins an ephemeral postgres:16).
package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/paylink/audit-log-service/internal/domain"
)

var base = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("paylink"),
		tcpostgres.WithUsername("paylink"),
		tcpostgres.WithPassword("paylink"),
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
	s, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("re-migrate (must be idempotent): %v", err)
	}
	return s
}

func appendOne(t *testing.T, s *Store, i int, ctx string) domain.Entry {
	t.Helper()
	e, err := s.Append(context.Background(), domain.AppendInput{
		Actor:      domain.Actor{Kind: domain.ActorService},
		Action:     "test.action",
		Resource:   "res:" + strconv.Itoa(i),
		Context:    json.RawMessage(ctx),
		OccurredAt: base.Add(time.Duration(i) * time.Second),
	})
	if err != nil {
		t.Fatalf("append %d: %v", i, err)
	}
	return e
}

func TestAppendChainAndVerify(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	var prevID int64
	for i := 0; i < 50; i++ {
		e := appendOne(t, s, i, `{"i":`+strconv.Itoa(i)+`}`)
		if e.EntryID <= prevID {
			t.Fatalf("entry_id not increasing: %d after %d", e.EntryID, prevID)
		}
		prevID = e.EntryID
	}
	res, err := s.VerifyRange(ctx, nil, nil)
	if err != nil || !res.OK {
		t.Fatalf("clean chain should verify: %+v err=%v", res, err)
	}
	first, err := s.GetByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	genesis := domain.GenesisHash()
	if len(first.PrevHash) != len(genesis) {
		t.Fatalf("prev hash len %d, want %d", len(first.PrevHash), len(genesis))
	}
	for _, b := range first.PrevHash {
		if b != 0 {
			t.Fatal("first entry prev_hash must be genesis (all zero)")
		}
	}
}

func TestGetByIDQueryPagination(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		appendOne(t, s, i, `{}`)
	}
	if _, err := s.GetByID(ctx, 1000); err != domain.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	page, err := s.Query(ctx, domain.QueryFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 10 || page.NextCursor == nil || page.Items[0].EntryID != 25 {
		t.Fatalf("bad page: n=%d cursor=%v first=%d", len(page.Items), page.NextCursor, page.Items[0].EntryID)
	}
	seen := 0
	cursor := int64(0)
	for {
		p, _ := s.Query(ctx, domain.QueryFilter{Limit: 10, Cursor: cursor})
		seen += len(p.Items)
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	if seen != 25 {
		t.Fatalf("paged %d, want 25", seen)
	}
	// resource filter
	p, _ := s.Query(ctx, domain.QueryFilter{Resource: "res:7"})
	if len(p.Items) != 1 || p.Items[0].Resource != "res:7" {
		t.Fatalf("resource filter: %+v", p.Items)
	}
}

func TestTamperContentDetected(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		appendOne(t, s, i, `{"i":`+strconv.Itoa(i)+`}`)
	}
	// Corrupt the authoritative hashed bytes for entry 5.
	if _, err := s.pool.Exec(ctx, `UPDATE audit.entries SET canonical_bytes=decode('00','hex') WHERE entry_id=5`); err != nil {
		t.Fatal(err)
	}
	res, err := s.VerifyRange(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || res.BrokenAt == nil || *res.BrokenAt != 5 {
		t.Fatalf("content tamper at 5 not detected: %+v", res)
	}
}

func TestTamperHashDetected(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		appendOne(t, s, i, `{}`)
	}
	// flip the first byte of entry 4's stored hash
	if _, err := s.pool.Exec(ctx,
		`UPDATE audit.entries SET entry_hash = set_byte(entry_hash, 0, (get_byte(entry_hash,0) # 255)) WHERE entry_id=4`); err != nil {
		t.Fatal(err)
	}
	res, _ := s.VerifyRange(ctx, nil, nil)
	if res.OK || *res.BrokenAt != 4 {
		t.Fatalf("hash tamper at 4 not detected: %+v", res)
	}
}

func TestDeleteMiddleDetected(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		appendOne(t, s, i, `{}`)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM audit.entries WHERE entry_id=5`); err != nil {
		t.Fatal(err)
	}
	res, _ := s.VerifyRange(ctx, nil, nil)
	// entry 6 now links to the (removed) entry 5, so the break surfaces at 6.
	if res.OK || *res.BrokenAt != 6 {
		t.Fatalf("deleted middle entry not detected at 6: %+v", res)
	}
}

func TestCanonicalRoundTripTrickyPayload(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	// Unsorted keys, nested objects, arrays, unicode + HTML chars, and a >2^53 integer. After the
	// jsonb store/read round-trip, the recomputed hash must still match (canonical re-normalizes).
	payload := `{"z":{"b":2,"a":1},"arr":[3,1,2],"s":"café <x>&","n":900719925474099123,"f":0.82,"exp":1e6,"nested":{"k":[{"y":2,"x":1}]}}`
	e := appendOne(t, s, 0, payload)
	got, err := s.GetByID(ctx, e.EntryID)
	if err != nil {
		t.Fatal(err)
	}
	if !domain.BuildProof(got).Valid {
		t.Fatal("recomputed proof invalid after jsonb round-trip — canonical not stable")
	}
	res, _ := s.VerifyRange(ctx, nil, nil)
	if !res.OK {
		t.Fatalf("verify failed on tricky payload round-trip: %+v", res)
	}
}

func TestFloatPayloadVerifies(t *testing.T) {
	// Regression: Postgres jsonb normalizes 1e6→1000000 and scaled decimals, so re-canonicalizing
	// from the jsonb columns would falsely report these clean entries as broken. Verify recomputes
	// from the stored canonical_bytes instead, so float/exponent payloads must verify OK.
	s := newStore(t)
	ctx := context.Background()
	for i, p := range []string{
		`{"score":0.82}`,
		`{"big":1e6}`,
		`{"scaled":1.50}`,
		`{"neg":-3.14159,"arr":[0.1,0.2,0.3]}`,
	} {
		e := appendOne(t, s, i, p)
		got, err := s.GetByID(ctx, e.EntryID)
		if err != nil {
			t.Fatal(err)
		}
		if !domain.BuildProof(got).Valid {
			t.Fatalf("float payload %q recomputed proof invalid after round-trip", p)
		}
	}
	res, err := s.VerifyRange(ctx, nil, nil)
	if err != nil || !res.OK {
		t.Fatalf("chain with float payloads should verify OK: %+v err=%v", res, err)
	}
}

func TestConcurrentAppends(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if _, err := s.Append(ctx, domain.AppendInput{
					Actor:      domain.Actor{Kind: domain.ActorSystem},
					Action:     "concurrent",
					Resource:   "r",
					Context:    json.RawMessage(`{}`),
					OccurredAt: base,
				}); err != nil {
					t.Errorf("append: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	_, count, err := s.Tail(ctx)
	if err != nil || count != 1000 {
		t.Fatalf("want 1000 entries, got %d err=%v", count, err)
	}
	var dups int64
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM (SELECT prev_hash FROM audit.entries GROUP BY prev_hash HAVING count(*) > 1) d`).
		Scan(&dups); err != nil {
		t.Fatal(err)
	}
	if dups != 0 {
		t.Fatalf("found %d duplicated prev_hash values — chain forked under concurrency", dups)
	}
	res, err := s.VerifyRange(ctx, nil, nil)
	if err != nil || !res.OK {
		t.Fatalf("concurrent chain should verify: %+v err=%v", res, err)
	}
}

func TestWindowedVerify(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		appendOne(t, s, i, `{}`) // entry_id i+1, occurred_at base+i s
	}
	from := base.Add(3 * time.Second)
	to := base.Add(6 * time.Second)
	res, err := s.VerifyRange(ctx, &from, &to)
	if err != nil || !res.OK {
		t.Fatalf("clean window should verify: %+v err=%v", res, err)
	}
	// tamper entry_id 5 (occurred_at base+4s, inside the window)
	if _, err := s.pool.Exec(ctx, `UPDATE audit.entries SET canonical_bytes=decode('00','hex') WHERE entry_id=5`); err != nil {
		t.Fatal(err)
	}
	res, _ = s.VerifyRange(ctx, &from, &to)
	if res.OK || *res.BrokenAt != 5 {
		t.Fatalf("windowed verify should detect tamper at 5: %+v", res)
	}
	// a window before the tampered entry stays ok
	earlyTo := base.Add(2 * time.Second)
	res, _ = s.VerifyRange(ctx, nil, &earlyTo)
	if !res.OK {
		t.Fatalf("window excluding the tamper should be ok: %+v", res)
	}
}
