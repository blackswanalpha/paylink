package state

import (
	"fmt"

	"github.com/paylink/paylink-chain/internal/types"
)

// GetValidator returns validator info by address.
func (s *StateDB) GetValidator(addr types.Address) *types.ValidatorInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.validators[addr]; ok {
		cp := *v
		return &cp
	}
	return nil
}

// IsActiveValidator checks if an address is an active validator.
func (s *StateDB) IsActiveValidator(addr types.Address) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.validators[addr]; ok {
		return v.IsActive
	}
	return false
}

// Stake adds to a validator's stake. Activates them if they meet minimum stake.
func (s *StateDB) Stake(addr types.Address, amount uint64, timestamp int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if amount == 0 {
		return fmt.Errorf("zero amount")
	}

	v, ok := s.validators[addr]
	if !ok {
		v = &types.ValidatorInfo{
			Address:  addr,
			JoinedAt: timestamp,
		}
		s.validators[addr] = v
		s.validatorList = append(s.validatorList, addr)
	}

	v.StakedAmount += amount

	// Activate if at or above minimum stake
	if !v.IsActive && v.StakedAmount >= s.minimumStake {
		v.IsActive = true
	}

	return nil
}

// InitiateWithdrawal starts the withdrawal process for a validator.
func (s *StateDB) InitiateWithdrawal(addr types.Address, amount uint64, timestamp int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.validators[addr]
	if !ok {
		return fmt.Errorf("validator not found: %s", addr)
	}

	if v.PendingWithdrawal > 0 {
		return fmt.Errorf("withdrawal already pending")
	}

	if amount > v.StakedAmount {
		return fmt.Errorf("insufficient stake: have %d, want %d", v.StakedAmount, amount)
	}

	remaining := v.StakedAmount - amount
	// If partial withdrawal, remaining must be >= minimum or == 0 (full withdrawal)
	if remaining > 0 && remaining < s.minimumStake {
		return fmt.Errorf("remaining stake %d below minimum %d", remaining, s.minimumStake)
	}

	v.PendingWithdrawal = amount
	v.WithdrawableAt = timestamp + s.withdrawalCooldown

	// Deactivate if full withdrawal
	if remaining < s.minimumStake {
		v.IsActive = false
	}

	return nil
}

// CompleteWithdrawal completes a pending withdrawal if cooldown has elapsed.
func (s *StateDB) CompleteWithdrawal(addr types.Address, timestamp int64) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.validators[addr]
	if !ok {
		return 0, fmt.Errorf("validator not found: %s", addr)
	}

	if v.PendingWithdrawal == 0 {
		return 0, fmt.Errorf("no withdrawal pending")
	}

	if timestamp < v.WithdrawableAt {
		return 0, fmt.Errorf("cooldown not elapsed: withdrawable at %d, current %d", v.WithdrawableAt, timestamp)
	}

	amount := v.PendingWithdrawal
	v.StakedAmount -= amount
	v.PendingWithdrawal = 0
	v.WithdrawableAt = 0

	return amount, nil
}

// CancelWithdrawal cancels a pending withdrawal and reactivates if eligible.
func (s *StateDB) CancelWithdrawal(addr types.Address) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.validators[addr]
	if !ok {
		return fmt.Errorf("validator not found: %s", addr)
	}

	if v.PendingWithdrawal == 0 {
		return fmt.Errorf("no withdrawal pending")
	}

	v.PendingWithdrawal = 0
	v.WithdrawableAt = 0

	// Reactivate if stake is sufficient
	if v.StakedAmount >= s.minimumStake {
		v.IsActive = true
	}

	return nil
}

// Slash reduces a validator's stake by the given amount.
func (s *StateDB) Slash(addr types.Address, amount uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.validators[addr]
	if !ok {
		return fmt.Errorf("validator not found: %s", addr)
	}

	if amount > v.StakedAmount {
		return fmt.Errorf("insufficient stake for slash: have %d, want %d", v.StakedAmount, amount)
	}

	v.StakedAmount -= amount
	v.TotalSlashed += amount

	// Deactivate if below minimum
	if v.StakedAmount < s.minimumStake {
		v.IsActive = false
	}

	return nil
}

// DistributeReward records a reward for an active validator.
func (s *StateDB) DistributeReward(addr types.Address, amount uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.validators[addr]
	if !ok || !v.IsActive {
		return fmt.Errorf("validator not active: %s", addr)
	}

	v.TotalRewards += amount
	return nil
}

// ActiveValidatorCount returns the number of active validators.
func (s *StateDB) ActiveValidatorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, v := range s.validators {
		if v.IsActive {
			count++
		}
	}
	return count
}

// GetAllValidators returns a copy of the ordered validator list.
func (s *StateDB) GetAllValidators() []types.Address {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cloneValidatorList()
}

// GetValidatorCount returns the total number of validators (active + inactive).
func (s *StateDB) GetValidatorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.validatorList)
}

// AllValidators returns a copy of all validator info (for Merkle root computation).
func (s *StateDB) AllValidators() map[types.Address]*types.ValidatorInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cloneValidators()
}

// TotalStaked returns the total amount staked across all validators.
func (s *StateDB) TotalStaked() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var total uint64
	for _, v := range s.validators {
		total += v.StakedAmount
	}
	return total
}

// ── Phase 2: VRF key management ──

// SetVRFPublicKey registers a VRF public key for a validator.
func (s *StateDB) SetVRFPublicKey(addr types.Address, vrfPubKey []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.validators[addr]
	if !ok {
		return fmt.Errorf("validator not found: %s", addr)
	}
	if !v.IsActive {
		return fmt.Errorf("validator not active: %s", addr)
	}

	cp := make([]byte, len(vrfPubKey))
	copy(cp, vrfPubKey)
	s.vrfKeys[addr] = cp
	v.VRFPublicKey = cp
	return nil
}

// GetVRFPublicKey returns the VRF public key for a validator.
func (s *StateDB) GetVRFPublicKey(addr types.Address) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if key, ok := s.vrfKeys[addr]; ok {
		cp := make([]byte, len(key))
		copy(cp, key)
		return cp
	}
	return nil
}

// GetActiveValidatorsWithStake returns active validators with their stake and VRF keys.
func (s *StateDB) GetActiveValidatorsWithStake() []types.ValidatorStakeInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []types.ValidatorStakeInfo
	for _, addr := range s.validatorList {
		v := s.validators[addr]
		if v != nil && v.IsActive {
			info := types.ValidatorStakeInfo{
				Address:      addr,
				StakedAmount: v.StakedAmount,
			}
			if key, ok := s.vrfKeys[addr]; ok {
				info.VRFPublicKey = make([]byte, len(key))
				copy(info.VRFPublicKey, key)
			}
			result = append(result, info)
		}
	}
	return result
}

// GetVoters returns all validators who voted on a given PayLink.
func (s *StateDB) GetVoters(plId types.Hash) []types.Address {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var voters []types.Address
	for key := range s.votes {
		if key.PayLinkID == plId {
			voters = append(voters, key.Validator)
		}
	}
	return voters
}
