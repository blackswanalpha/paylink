// Package memory is an in-memory domain.Store for unit tests and local dev. It mirrors the
// postgres store's atomicity (a single mutex stands in for row locks + transactions) and its
// idempotency semantics: an approval set stands in for the approvals PK, and a processed set
// stands in for the escrow.processed_events DbDedupe table.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/paylink/escrow-manager/internal/domain"
	"github.com/paylink/escrow-manager/internal/fsm"
)

// Store is a goroutine-safe in-memory escrow store.
type Store struct {
	mu        sync.Mutex
	byID      map[string]domain.Escrow
	byPL      map[string]string   // plID -> escrowID
	approvals map[string][]string // escrowID -> recorded approver addrs (insertion order)
	processed map[string]bool     // "scope:plID:txHash" -> seen (consumer dedupe)
	now       func() time.Time
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		byID:      map[string]domain.Escrow{},
		byPL:      map[string]string{},
		approvals: map[string][]string{},
		processed: map[string]bool{},
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// dedupeScope mirrors the postgres store's DbDedupe scope.
const dedupeScope = "chain.paylink.verified"

// CreateEscrow inserts an escrow, enforcing one escrow per PayLink (domain.ErrEscrowExists).
func (s *Store) CreateEscrow(_ context.Context, e domain.Escrow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byPL[e.PLID]; ok {
		return domain.ErrEscrowExists
	}
	s.byID[e.ID] = e
	s.byPL[e.PLID] = e.ID
	return nil
}

// GetEscrow returns the escrow by id, or domain.ErrNotFound.
func (s *Store) GetEscrow(_ context.Context, id string) (domain.Escrow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.byID[id]
	if !ok {
		return domain.Escrow{}, domain.ErrNotFound
	}
	return e, nil
}

// ListEscrows returns the creator's escrows, optionally filtered by state, most recent first.
func (s *Store) ListEscrows(_ context.Context, creatorAddr, state string, limit int) ([]domain.Escrow, error) {
	if limit <= 0 {
		limit = 20
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	hits := make([]domain.Escrow, 0)
	for _, e := range s.byID {
		if e.CreatorAddr != creatorAddr {
			continue
		}
		if state != "" && string(e.State) != state {
			continue
		}
		hits = append(hits, e)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].CreatedAt.Equal(hits[j].CreatedAt) {
			return hits[i].ID > hits[j].ID
		}
		return hits[i].CreatedAt.After(hits[j].CreatedAt)
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

// Mutate locks the escrow by id, calls fn, and applies the returned Update atomically.
func (s *Store) Mutate(_ context.Context, id string, fn domain.MutateFn) (domain.Escrow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.byID[id]
	if !ok {
		return domain.Escrow{}, domain.ErrNotFound
	}
	up, err := fn(e, append([]string(nil), s.approvals[id]...))
	if err != nil {
		return domain.Escrow{}, err
	}
	s.apply(&e, up)
	return e, nil
}

// ApplyFunding is Mutate keyed by pl_id, deduplicated on (scope, plID:txHash) exactly like the
// postgres store's DbDedupe row (mark + effect commit together; an fn error rolls both back).
func (s *Store) ApplyFunding(_ context.Context, plID, txHash string, fn domain.MutateFn) (domain.Escrow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byPL[plID]
	if !ok {
		return domain.Escrow{}, false, domain.ErrNotFound
	}
	e := s.byID[id]
	key := dedupeScope + ":" + plID + ":" + txHash
	if s.processed[key] {
		return e, false, nil // duplicate — already processed
	}
	up, err := fn(e, append([]string(nil), s.approvals[id]...))
	if err != nil {
		return domain.Escrow{}, false, err // "rollback": the dedupe mark is not recorded
	}
	s.apply(&e, up)
	s.processed[key] = true
	return e, true, nil
}

// ReleaseDueTimeLocks CAS-releases funded time_lock escrows whose release_at has passed.
func (s *Store) ReleaseDueTimeLocks(_ context.Context, now time.Time) ([]domain.Escrow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Escrow
	for id, e := range s.byID {
		if e.State != fsm.StateWaiting || !e.Funded || e.ConditionType != domain.ConditionTimeLock {
			continue
		}
		if e.ReleaseAt == nil || now.Before(*e.ReleaseAt) {
			continue
		}
		e.State = fsm.StateReleased
		e.UpdatedAt = s.now()
		s.byID[id] = e
		out = append(out, e)
	}
	sortByCreated(out)
	return out, nil
}

// RefundTimedOut CAS-refunds WAITING escrows whose timeout_at has passed (DISPUTED untouched).
func (s *Store) RefundTimedOut(_ context.Context, now time.Time) ([]domain.Escrow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Escrow
	for id, e := range s.byID {
		if e.State != fsm.StateWaiting || now.Before(e.TimeoutAt) {
			continue
		}
		e.State = fsm.StateRefunded
		e.UpdatedAt = s.now()
		s.byID[id] = e
		out = append(out, e)
	}
	sortByCreated(out)
	return out, nil
}

// Ping always succeeds for the in-memory store.
func (s *Store) Ping(_ context.Context) error { return nil }

// Approvals returns the recorded approvals for an escrow (test helper).
func (s *Store) Approvals(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.approvals[id]...)
}

// apply mirrors the postgres store's single-transaction update semantics. Caller holds mu.
func (s *Store) apply(e *domain.Escrow, up domain.Update) {
	changed := false
	if up.AddApproval != "" {
		found := false
		for _, a := range s.approvals[e.ID] {
			if a == up.AddApproval {
				found = true
				break
			}
		}
		if !found {
			s.approvals[e.ID] = append(s.approvals[e.ID], up.AddApproval)
		}
	}
	if up.SetFunded && !e.Funded {
		e.Funded = true
		e.FundedTxHash = up.FundedTxHash
		changed = true
	}
	if up.SetState != "" && up.SetState != e.State {
		e.State = up.SetState
		changed = true
	}
	if up.DisputeReason != "" {
		e.DisputeReason = up.DisputeReason
		changed = true
	}
	if changed {
		e.UpdatedAt = s.now()
	}
	s.byID[e.ID] = *e
}

func sortByCreated(list []domain.Escrow) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID < list[j].ID
		}
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
}
