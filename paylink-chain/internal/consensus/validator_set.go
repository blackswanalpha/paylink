package consensus

import (
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// ValidatorSet manages the active set of validators for consensus.
type ValidatorSet struct {
	state *state.StateDB
}

// NewValidatorSet creates a new validator set manager.
func NewValidatorSet(s *state.StateDB) *ValidatorSet {
	return &ValidatorSet{state: s}
}

// IsActive checks if an address is an active validator.
func (vs *ValidatorSet) IsActive(addr types.Address) bool {
	return vs.state.IsActiveValidator(addr)
}

// ActiveCount returns the number of active validators.
func (vs *ValidatorSet) ActiveCount() int {
	return vs.state.ActiveValidatorCount()
}

// GetAll returns all validator addresses.
func (vs *ValidatorSet) GetAll() []types.Address {
	return vs.state.GetAllValidators()
}

// GetActive returns only active validator addresses.
func (vs *ValidatorSet) GetActive() []types.Address {
	all := vs.state.GetAllValidators()
	var active []types.Address
	for _, addr := range all {
		if vs.state.IsActiveValidator(addr) {
			active = append(active, addr)
		}
	}
	return active
}
