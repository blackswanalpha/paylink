// Package memory is an in-memory domain.Store for dev and unit tests.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/paylink/proof-validator/internal/domain"
)

// Store is an in-memory domain.Store keyed by proof_hash.
type Store struct {
	mu     sync.Mutex
	proofs map[string]domain.ProofRecord
}

// New builds an empty Store.
func New() *Store { return &Store{proofs: map[string]domain.ProofRecord{}} }

// InsertProof inserts a record, returning ErrProofExists on a duplicate proof_hash.
func (s *Store) InsertProof(_ context.Context, r domain.ProofRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.proofs[r.ProofHash]; ok {
		return domain.ErrProofExists
	}
	s.proofs[r.ProofHash] = r
	return nil
}

// MarkBroadcast records the settlement tx hash and advances status.
func (s *Store) MarkBroadcast(_ context.Context, proofHash, txHash, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.proofs[proofHash]
	if !ok {
		return domain.ErrNotFound
	}
	r.TxHash = txHash
	r.Status = status
	r.UpdatedAt = time.Now().UTC()
	s.proofs[proofHash] = r
	return nil
}

// GetByProofHash returns the record, or ErrNotFound.
func (s *Store) GetByProofHash(_ context.Context, proofHash string) (domain.ProofRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.proofs[proofHash]
	if !ok {
		return domain.ProofRecord{}, domain.ErrNotFound
	}
	return r, nil
}

// Ping always succeeds.
func (s *Store) Ping(context.Context) error { return nil }
