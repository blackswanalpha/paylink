package state

import (
	"fmt"

	"github.com/paylink/paylink-chain/internal/types"
)

// CreatePayLink creates a new PayLink record and updates indexes.
func (s *StateDB) CreatePayLink(pl *types.PayLink) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.paylinks[pl.ID]; exists {
		return fmt.Errorf("paylink already exists: %s", pl.ID)
	}
	cp := *pl
	s.paylinks[pl.ID] = &cp

	// Update indexes
	s.paylinksByCreator[pl.Creator] = append(s.paylinksByCreator[pl.Creator], pl.ID)
	s.paylinksByReceiver[pl.Receiver] = append(s.paylinksByReceiver[pl.Receiver], pl.ID)
	s.paylinksByStatus[pl.Status] = append(s.paylinksByStatus[pl.Status], pl.ID)
	s.paylinksByOwner[pl.Owner] = append(s.paylinksByOwner[pl.Owner], pl.ID)
	return nil
}

// GetPayLink returns a PayLink by ID.
func (s *StateDB) GetPayLink(id types.Hash) *types.PayLink {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if pl, ok := s.paylinks[id]; ok {
		cp := *pl
		return &cp
	}
	return nil
}

// SetPayLinkStatus updates the status of a PayLink and updates the status index.
func (s *StateDB) SetPayLinkStatus(id types.Hash, status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pl, ok := s.paylinks[id]
	if !ok {
		return fmt.Errorf("paylink not found: %s", id)
	}

	// Remove from old status index
	oldStatus := pl.Status
	s.removeFromStatusIndex(oldStatus, id)

	pl.Status = status

	// Add to new status index
	s.paylinksByStatus[status] = append(s.paylinksByStatus[status], id)
	return nil
}

func (s *StateDB) removeFromStatusIndex(status types.Status, id types.Hash) {
	ids := s.paylinksByStatus[status]
	for i, h := range ids {
		if h == id {
			s.paylinksByStatus[status] = append(ids[:i], ids[i+1:]...)
			return
		}
	}
}

// IncrementVoteCount increments the vote count for a PayLink and returns the new count.
func (s *StateDB) IncrementVoteCount(id types.Hash) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pl, ok := s.paylinks[id]
	if !ok {
		return 0, fmt.Errorf("paylink not found: %s", id)
	}
	pl.VoteCount++
	return pl.VoteCount, nil
}

// HasVoted checks if a validator has already voted on a PayLink.
func (s *StateDB) HasVoted(plId types.Hash, validator types.Address) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.votes[voteKey{PayLinkID: plId, Validator: validator}]
}

// RecordVote records a validator's vote on a PayLink.
func (s *StateDB) RecordVote(plId types.Hash, validator types.Address) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.votes[voteKey{PayLinkID: plId, Validator: validator}] = true
}

// IsProofUsed checks if a proof hash has been used (anti-replay).
func (s *StateDB) IsProofUsed(proofHash types.Hash) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usedProofs[proofHash]
}

// MarkProofUsed marks a proof hash as used.
func (s *StateDB) MarkProofUsed(proofHash types.Hash) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usedProofs[proofHash] = true
}

// GetSubmittedProof returns the first submitted proof hash for a PayLink.
func (s *StateDB) GetSubmittedProof(plId types.Hash) (types.Hash, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.submittedProofs[plId]
	return h, ok
}

// SetSubmittedProof records the first submitted proof hash for a PayLink.
func (s *StateDB) SetSubmittedProof(plId types.Hash, proofHash types.Hash) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.submittedProofs[plId] = proofHash
}

// GetVoteCount returns the vote count for a PayLink.
func (s *StateDB) GetVoteCount(plId types.Hash) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if pl, ok := s.paylinks[plId]; ok {
		return pl.VoteCount
	}
	return 0
}

// AllPayLinks returns a copy of all paylinks (for Merkle root computation).
func (s *StateDB) AllPayLinks() map[types.Hash]*types.PayLink {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clonePaylinks()
}

// GetPayLinksByCreator returns all PayLink IDs created by an address.
func (s *StateDB) GetPayLinksByCreator(creator types.Address) []types.Hash {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.paylinksByCreator[creator]
	out := make([]types.Hash, len(ids))
	copy(out, ids)
	return out
}

// GetPayLinksByReceiver returns all PayLink IDs for a receiver address.
func (s *StateDB) GetPayLinksByReceiver(receiver types.Address) []types.Hash {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.paylinksByReceiver[receiver]
	out := make([]types.Hash, len(ids))
	copy(out, ids)
	return out
}

// GetPayLinksByStatus returns all PayLink IDs with a given status.
func (s *StateDB) GetPayLinksByStatus(status types.Status) []types.Hash {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.paylinksByStatus[status]
	out := make([]types.Hash, len(ids))
	copy(out, ids)
	return out
}

// PayLinkCount returns the total number of paylinks.
func (s *StateDB) PayLinkCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.paylinks)
}

// GetVotersForPayLink returns all validator addresses that voted on a PayLink,
// sorted by address (same contract as GetVoters — deterministic order).
func (s *StateDB) GetVotersForPayLink(plId types.Hash) []types.Address {
	return s.GetVoters(plId)
}

// UsedProofCount returns the total number of used proofs.
func (s *StateDB) UsedProofCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.usedProofs)
}

// AccountCount returns the total number of accounts.
func (s *StateDB) AccountCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.accounts)
}

// ── NFT-style ownership operations ──

// SetPayLinkOwner transfers ownership, clears single-paylink approval,
// increments TransferCount, and updates the owner index.
func (s *StateDB) SetPayLinkOwner(id types.Hash, newOwner types.Address) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pl, ok := s.paylinks[id]
	if !ok {
		return fmt.Errorf("paylink not found: %s", id)
	}

	oldOwner := pl.Owner

	// Remove from old owner index
	s.removeFromOwnerIndex(oldOwner, id)

	pl.Owner = newOwner
	pl.Approved = types.Address{} // clear single-paylink approval on transfer
	pl.TransferCount++

	// Add to new owner index
	s.paylinksByOwner[newOwner] = append(s.paylinksByOwner[newOwner], id)
	return nil
}

func (s *StateDB) removeFromOwnerIndex(owner types.Address, id types.Hash) {
	ids := s.paylinksByOwner[owner]
	for i, h := range ids {
		if h == id {
			s.paylinksByOwner[owner] = append(ids[:i], ids[i+1:]...)
			return
		}
	}
}

// SetPayLinkApproval sets or revokes single-paylink approval.
func (s *StateDB) SetPayLinkApproval(id types.Hash, approved types.Address) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pl, ok := s.paylinks[id]
	if !ok {
		return fmt.Errorf("paylink not found: %s", id)
	}
	pl.Approved = approved
	return nil
}

// SetOperatorApproval sets or revokes operator approval for all of an owner's paylinks.
func (s *StateDB) SetOperatorApproval(owner, operator types.Address, approved bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := operatorKey{Owner: owner, Operator: operator}
	if approved {
		s.operatorApprovals[key] = true
	} else {
		delete(s.operatorApprovals, key)
	}
}

// IsApprovedOrOwner checks if spender is the owner, the approved address,
// or a global operator for the paylink's owner.
func (s *StateDB) IsApprovedOrOwner(id types.Hash, spender types.Address) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pl, ok := s.paylinks[id]
	if !ok {
		return false
	}
	if spender == pl.Owner {
		return true
	}
	if spender == pl.Approved {
		return true
	}
	return s.operatorApprovals[operatorKey{Owner: pl.Owner, Operator: spender}]
}

// IsOperatorApproved checks if operator is approved for all of owner's paylinks.
func (s *StateDB) IsOperatorApproved(owner, operator types.Address) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.operatorApprovals[operatorKey{Owner: owner, Operator: operator}]
}

// GetPayLinksByOwner returns all PayLink IDs owned by an address.
func (s *StateDB) GetPayLinksByOwner(owner types.Address) []types.Hash {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.paylinksByOwner[owner]
	out := make([]types.Hash, len(ids))
	copy(out, ids)
	return out
}

// OwnerPayLinkCount returns the number of PayLinks owned by an address.
func (s *StateDB) OwnerPayLinkCount(owner types.Address) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.paylinksByOwner[owner])
}

// GetPayLinkOwner returns the owner of a specific PayLink.
func (s *StateDB) GetPayLinkOwner(id types.Hash) (types.Address, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pl, ok := s.paylinks[id]
	if !ok {
		return types.Address{}, false
	}
	return pl.Owner, true
}
