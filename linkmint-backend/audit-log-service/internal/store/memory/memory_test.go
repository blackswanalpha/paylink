package memory

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/paylink/audit-log-service/internal/domain"
)

var base = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

func appendOne(t *testing.T, s *Store, i int) domain.Entry {
	t.Helper()
	e, err := s.Append(context.Background(), domain.AppendInput{
		Actor:      domain.Actor{Kind: domain.ActorService},
		Action:     "test.action",
		Resource:   "res:" + strconv.Itoa(i),
		Context:    json.RawMessage(`{"i":` + strconv.Itoa(i) + `}`),
		OccurredAt: base.Add(time.Duration(i) * time.Second),
	})
	if err != nil {
		t.Fatalf("append %d: %v", i, err)
	}
	return e
}

func TestChainIntegrity(t *testing.T) {
	s := New()
	for i := 0; i < 100; i++ {
		appendOne(t, s, i)
	}
	res, err := s.VerifyRange(context.Background(), nil, nil)
	if err != nil || !res.OK {
		t.Fatalf("clean chain should verify: %+v err=%v", res, err)
	}
	// linkage: first prev == genesis; each prev == predecessor entry_hash.
	if hex.EncodeToString(s.entries[0].PrevHash) != hex.EncodeToString(domain.GenesisHash()) {
		t.Fatal("first entry prev_hash must be genesis")
	}
	for i := 1; i < len(s.entries); i++ {
		if hex.EncodeToString(s.entries[i].PrevHash) != hex.EncodeToString(s.entries[i-1].EntryHash) {
			t.Fatalf("entry %d not linked to predecessor", i+1)
		}
	}
}

func TestTamperContentDetected(t *testing.T) {
	s := New()
	for i := 0; i < 10; i++ {
		appendOne(t, s, i)
	}
	s.entries[4].Canonical = []byte(`{"tampered":true}`) // edit the hashed bytes, leave entry_hash stale
	res, err := s.VerifyRange(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || res.BrokenAt == nil || *res.BrokenAt != 5 {
		t.Fatalf("content tamper at entry 5 not detected: %+v", res)
	}
}

func TestTamperHashDetected(t *testing.T) {
	s := New()
	for i := 0; i < 10; i++ {
		appendOne(t, s, i)
	}
	s.entries[3].EntryHash[0] ^= 0xff // flip a byte of the stored hash
	res, _ := s.VerifyRange(context.Background(), nil, nil)
	if res.OK || *res.BrokenAt != 4 {
		t.Fatalf("hash tamper at entry 4 not detected: %+v", res)
	}
}

func TestDeleteMiddleDetected(t *testing.T) {
	s := New()
	for i := 0; i < 10; i++ {
		appendOne(t, s, i)
	}
	// remove entry_id 5 (index 4); entry_id 6 now links to a missing predecessor.
	s.entries = append(s.entries[:4], s.entries[5:]...)
	res, _ := s.VerifyRange(context.Background(), nil, nil)
	if res.OK || *res.BrokenAt != 6 {
		t.Fatalf("deleted middle entry not detected at 6: %+v", res)
	}
}

func TestConcurrentAppendsProduceValidChain(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_, err := s.Append(context.Background(), domain.AppendInput{
					Actor:      domain.Actor{Kind: domain.ActorSystem},
					Action:     "concurrent",
					Resource:   "r",
					Context:    json.RawMessage(`{}`),
					OccurredAt: base,
				})
				if err != nil {
					t.Errorf("append: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()

	if len(s.entries) != 1000 {
		t.Fatalf("want 1000 entries, got %d", len(s.entries))
	}
	res, err := s.VerifyRange(context.Background(), nil, nil)
	if err != nil || !res.OK {
		t.Fatalf("concurrent chain should verify: %+v err=%v", res, err)
	}
	// every prev_hash is unique (no two entries linked to the same tail = no fork).
	seen := make(map[string]struct{}, 1000)
	for _, e := range s.entries {
		k := hex.EncodeToString(e.PrevHash)
		if _, dup := seen[k]; dup {
			t.Fatal("duplicate prev_hash — chain forked under concurrency")
		}
		seen[k] = struct{}{}
	}
}

func TestGetByID(t *testing.T) {
	s := New()
	e := appendOne(t, s, 0)
	got, err := s.GetByID(context.Background(), e.EntryID)
	if err != nil || got.EntryID != e.EntryID {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	if _, err := s.GetByID(context.Background(), 999); err != domain.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestQueryPaginationAndFilter(t *testing.T) {
	s := New()
	for i := 0; i < 25; i++ {
		appendOne(t, s, i)
	}
	// newest-first page of 10
	page, err := s.Query(context.Background(), domain.QueryFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 10 || page.NextCursor == nil {
		t.Fatalf("want 10 items + cursor, got %d cursor=%v", len(page.Items), page.NextCursor)
	}
	if page.Items[0].EntryID != 25 {
		t.Fatalf("newest first expected 25, got %d", page.Items[0].EntryID)
	}
	// page through to exhaustion
	seen := 0
	cursor := int64(0)
	for {
		p, _ := s.Query(context.Background(), domain.QueryFilter{Limit: 10, Cursor: cursor})
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
	p, _ := s.Query(context.Background(), domain.QueryFilter{Resource: "res:3"})
	if len(p.Items) != 1 || p.Items[0].Resource != "res:3" {
		t.Fatalf("resource filter failed: %+v", p.Items)
	}
}

func TestVerifyWindowed(t *testing.T) {
	s := New()
	for i := 0; i < 10; i++ {
		appendOne(t, s, i)
	}
	from := base.Add(3 * time.Second)
	to := base.Add(6 * time.Second)
	res, err := s.VerifyRange(context.Background(), &from, &to)
	if err != nil || !res.OK {
		t.Fatalf("clean window should verify: %+v err=%v", res, err)
	}
	// tamper an entry inside the window
	s.entries[5].Canonical = []byte(`{"tampered":9}`)
	res, _ = s.VerifyRange(context.Background(), &from, &to)
	if res.OK || *res.BrokenAt != 6 {
		t.Fatalf("windowed verify should detect tamper at 6: %+v", res)
	}
	// a window that ends before the tampered entry stays ok
	earlyTo := base.Add(4 * time.Second)
	res, _ = s.VerifyRange(context.Background(), &from, &earlyTo)
	if !res.OK {
		t.Fatalf("window excluding the tamper should be ok: %+v", res)
	}
}

func TestTailAndEmpty(t *testing.T) {
	s := New()
	h, n, err := s.Tail(context.Background())
	if err != nil || n != 0 || hex.EncodeToString(h) != hex.EncodeToString(domain.GenesisHash()) {
		t.Fatalf("empty tail should be genesis: n=%d", n)
	}
	res, _ := s.VerifyRange(context.Background(), nil, nil)
	if !res.OK {
		t.Fatal("empty chain verifies ok")
	}
	e := appendOne(t, s, 0)
	h, n, _ = s.Tail(context.Background())
	if n != 1 || hex.EncodeToString(h) != hex.EncodeToString(e.EntryHash) {
		t.Fatalf("tail mismatch: n=%d", n)
	}
}
