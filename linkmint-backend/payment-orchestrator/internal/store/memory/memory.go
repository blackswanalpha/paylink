// Package memory is an in-memory domain.Store for unit tests and local dev. It mirrors the
// postgres store's atomicity (a single mutex stands in for row locks + transactions) and its
// idempotency semantics (an applied-events set stands in for the applied_chain_events table).
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/paylink/payment-orchestrator/internal/domain"
)

// Store is a goroutine-safe in-memory payment store.
type Store struct {
	mu      sync.Mutex
	byID    map[string]domain.Payment
	byPL    map[string]string // paylinkID -> paymentID
	applied map[string]bool   // "paylinkID:seq" -> seen (event dedupe)
	now     func() time.Time
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		byID:    map[string]domain.Payment{},
		byPL:    map[string]string{},
		applied: map[string]bool{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func dedupeKey(paylinkID string, seq uint64) string {
	return paylinkID + ":" + itoa(seq)
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// CreatePayment inserts a payment, enforcing one payment per PayLink (domain.ErrPaymentExists).
func (s *Store) CreatePayment(_ context.Context, p domain.Payment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byPL[p.PayLinkID]; ok {
		return domain.ErrPaymentExists
	}
	s.byID[p.ID] = p
	s.byPL[p.PayLinkID] = p.ID
	return nil
}

// GetPayment returns the payment by id, or domain.ErrNotFound.
func (s *Store) GetPayment(_ context.Context, id string) (domain.Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.byID[id]
	if !ok {
		return domain.Payment{}, domain.ErrNotFound
	}
	return p, nil
}

// GetPaymentByPayLink returns the payment for a paylink id, or domain.ErrNotFound.
func (s *Store) GetPaymentByPayLink(_ context.Context, paylinkID string) (domain.Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byPL[paylinkID]
	if !ok {
		return domain.Payment{}, domain.ErrNotFound
	}
	return s.byID[id], nil
}

// ApplyChainEvent advances the payment idempotently. Duplicate (paylinkID, seq) refs are no-ops.
func (s *Store) ApplyChainEvent(_ context.Context, ev domain.ChainEventRef, project domain.ProjectFn) (domain.Payment, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byPL[ev.PayLinkID]
	if !ok {
		return domain.Payment{}, false, domain.ErrNotFound
	}
	p := s.byID[id]

	key := dedupeKey(ev.PayLinkID, ev.Seq)
	if s.applied[key] {
		return p, false, nil // duplicate event — already seen
	}

	next, changed, perr := project(p.Status)
	s.applied[key] = true // record as seen regardless of outcome
	if perr != nil {
		return p, false, perr
	}
	if !changed {
		if ev.Seq > p.LastEventSeq {
			p.LastEventSeq = ev.Seq
			s.byID[id] = p
		}
		return p, false, nil
	}

	p.Status = next
	if ev.Seq > p.LastEventSeq {
		p.LastEventSeq = ev.Seq
	}
	p.UpdatedAt = s.now()
	s.byID[id] = p
	return p, true, nil
}

// Reconcile advances the payment toward on-chain truth without event dedupe (read path).
func (s *Store) Reconcile(_ context.Context, paylinkID string, project domain.ProjectFn) (domain.Payment, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byPL[paylinkID]
	if !ok {
		return domain.Payment{}, false, domain.ErrNotFound
	}
	p := s.byID[id]
	next, changed, perr := project(p.Status)
	if perr != nil {
		return p, false, perr
	}
	if !changed {
		return p, false, nil
	}
	p.Status = next
	p.UpdatedAt = s.now()
	s.byID[id] = p
	return p, true, nil
}

// Ping always succeeds for the in-memory store.
func (s *Store) Ping(_ context.Context) error { return nil }
