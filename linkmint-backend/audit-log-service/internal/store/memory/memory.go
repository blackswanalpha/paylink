// Package memory is an in-memory domain.Store for unit tests and local dev. A single mutex stands
// in for the postgres advisory lock, serializing appends so prev_hash always links to the true
// tail. Entries are stored in append (entry_id) order; entry_id is len+1 (contiguous, deterministic),
// so memory and postgres compute identical hashes for identical input.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/paylink/audit-log-service/internal/domain"
)

// Store is a goroutine-safe in-memory audit chain.
type Store struct {
	mu      sync.Mutex
	entries []domain.Entry
}

// New returns an empty Store.
func New() *Store { return &Store{} }

func cloneRaw(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// Append links the entry to the tail, computes its hash, and stores it.
func (s *Store) Append(_ context.Context, in domain.AppendInput) (domain.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := domain.GenesisHash()
	if n := len(s.entries); n > 0 {
		prev = s.entries[n-1].EntryHash
	}
	e, err := domain.BuildEntry(in, prev)
	if err != nil {
		return domain.Entry{}, err
	}
	e.EntryID = int64(len(s.entries) + 1)
	// Defensive copies so a caller mutating its request buffers can't alter stored entries.
	e.Before = cloneRaw(e.Before)
	e.After = cloneRaw(e.After)
	e.Context = cloneRaw(e.Context)
	e.Canonical = cloneRaw(e.Canonical)
	s.entries = append(s.entries, e)
	return e, nil
}

// GetByID returns the entry by id, or domain.ErrNotFound.
func (s *Store) GetByID(_ context.Context, id int64) (domain.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.entries {
		if s.entries[i].EntryID == id {
			return s.entries[i], nil
		}
	}
	return domain.Entry{}, domain.ErrNotFound
}

// Query returns a newest-first page matching the filter.
func (s *Store) Query(_ context.Context, f domain.QueryFilter) (domain.Page, error) {
	limit := f.Limit
	switch {
	case limit <= 0:
		limit = 20
	case limit > 100:
		limit = 100
	}
	s.mu.Lock()
	hits := make([]domain.Entry, 0)
	for _, e := range s.entries {
		if f.Actor != nil && (e.Actor.ID == nil || *e.Actor.ID != *f.Actor) {
			continue
		}
		if f.Resource != "" && e.Resource != f.Resource {
			continue
		}
		if f.From != nil && e.OccurredAt.Before(*f.From) {
			continue
		}
		if f.To != nil && e.OccurredAt.After(*f.To) {
			continue
		}
		if f.Cursor > 0 && e.EntryID >= f.Cursor {
			continue
		}
		hits = append(hits, e)
	}
	s.mu.Unlock()

	sort.Slice(hits, func(i, j int) bool { return hits[i].EntryID > hits[j].EntryID })
	var next *int64
	if len(hits) > limit {
		hits = hits[:limit]
		c := hits[len(hits)-1].EntryID
		next = &c
	}
	return domain.Page{Items: hits, NextCursor: next}, nil
}

// VerifyRange verifies the contiguous segment [startIdx, endIdx] selected by the time bounds,
// mirroring the postgres store: startIdx is the first entry (by entry_id) with occurred_at >= from,
// endIdx the last with occurred_at <= to. Linkage is seeded from the entry preceding startIdx.
func (s *Store) VerifyRange(_ context.Context, from, to *time.Time) (domain.VerifyResult, error) {
	s.mu.Lock()
	snap := make([]domain.Entry, len(s.entries))
	copy(snap, s.entries)
	s.mu.Unlock()

	if len(snap) == 0 {
		return domain.VerifyResult{OK: true}, nil
	}
	startIdx := -1
	for i := range snap {
		if from == nil || !snap[i].OccurredAt.Before(*from) {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return domain.VerifyResult{OK: true}, nil
	}
	endIdx := -1
	for i := len(snap) - 1; i >= 0; i-- {
		if to == nil || !snap[i].OccurredAt.After(*to) {
			endIdx = i
			break
		}
	}
	if endIdx == -1 || endIdx < startIdx {
		return domain.VerifyResult{OK: true}, nil
	}

	expected := domain.GenesisHash()
	if startIdx > 0 {
		expected = snap[startIdx-1].EntryHash
	}
	for i := startIdx; i <= endIdx; i++ {
		selfOK, linkOK := domain.CheckEntry(snap[i], expected)
		if !selfOK || !linkOK {
			id := snap[i].EntryID
			return domain.VerifyResult{OK: false, BrokenAt: &id}, nil
		}
		expected = snap[i].EntryHash
	}
	return domain.VerifyResult{OK: true}, nil
}

// Tail returns the head entry_hash (genesis when empty) and the entry count.
func (s *Store) Tail(_ context.Context) ([]byte, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.entries)
	if n == 0 {
		return domain.GenesisHash(), 0, nil
	}
	return s.entries[n-1].EntryHash, int64(n), nil
}

// Ping always succeeds for the in-memory store.
func (s *Store) Ping(_ context.Context) error { return nil }
