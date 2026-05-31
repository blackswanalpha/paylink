package consensus

import (
	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/types"
)

// PoV implements Proof-of-Validation consensus.
// Phase 1: single proposer (the admin/node operator).
// Phase 2: VRF-based committee selection.
type PoV struct {
	validatorSet *ValidatorSet
	proposerAddr types.Address // Fixed proposer for Phase 1
}

// NewPoV creates a new PoV consensus engine.
func NewPoV(vs *ValidatorSet, proposer types.Address) *PoV {
	return &PoV{
		validatorSet: vs,
		proposerAddr: proposer,
	}
}

// Proposer returns the current block proposer.
// Phase 1: always returns the configured proposer address.
func (p *PoV) Proposer() types.Address {
	return p.proposerAddr
}

// ValidateBlock checks that a block is valid according to PoV rules.
func (p *PoV) ValidateBlock(block *types.Block) error {
	// Verify block hash
	expectedHash := crypto.SHA256Hash(block.HeaderBytes())
	if block.Hash != expectedHash {
		return &ConsensusError{Msg: "invalid block hash"}
	}
	return nil
}

// ConsensusError represents a consensus validation error.
type ConsensusError struct {
	Msg string
}

func (e *ConsensusError) Error() string {
	return "consensus: " + e.Msg
}
