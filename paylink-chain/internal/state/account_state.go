package state

import (
	"fmt"

	"github.com/paylink/paylink-chain/internal/types"
)

// GetAccount returns the account for an address, or nil if not found.
func (s *StateDB) GetAccount(addr types.Address) *types.Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accounts[addr]
}

// GetOrCreateAccount returns the account for an address, creating one if it doesn't exist.
func (s *StateDB) GetOrCreateAccount(addr types.Address) *types.Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accounts[addr]
	if !ok {
		acc = &types.Account{Balance: 0, Nonce: 0}
		s.accounts[addr] = acc
	}
	return acc
}

// GetBalance returns the balance of an address.
func (s *StateDB) GetBalance(addr types.Address) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if acc, ok := s.accounts[addr]; ok {
		return acc.Balance
	}
	return 0
}

// GetNonce returns the nonce of an address.
func (s *StateDB) GetNonce(addr types.Address) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if acc, ok := s.accounts[addr]; ok {
		return acc.Nonce
	}
	return 0
}

// SetBalance sets the balance of an address.
func (s *StateDB) SetBalance(addr types.Address, balance uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accounts[addr]
	if !ok {
		acc = &types.Account{}
		s.accounts[addr] = acc
	}
	acc.Balance = balance
}

// IncrementNonce increments the nonce of an address.
func (s *StateDB) IncrementNonce(addr types.Address) {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accounts[addr]
	if !ok {
		acc = &types.Account{}
		s.accounts[addr] = acc
	}
	acc.Nonce++
}

// Transfer moves tokens from one account to another.
func (s *StateDB) Transfer(from, to types.Address, amount uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fromAcc, ok := s.accounts[from]
	if !ok || fromAcc.Balance < amount {
		return fmt.Errorf("insufficient balance: have %d, need %d", s.safeBalance(from), amount)
	}

	toAcc, ok := s.accounts[to]
	if !ok {
		toAcc = &types.Account{}
		s.accounts[to] = toAcc
	}

	fromAcc.Balance -= amount
	toAcc.Balance += amount
	return nil
}

// AddBalance adds tokens to an account (for minting/rewards).
func (s *StateDB) AddBalance(addr types.Address, amount uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accounts[addr]
	if !ok {
		acc = &types.Account{}
		s.accounts[addr] = acc
	}
	acc.Balance += amount
}

// SubBalance subtracts tokens from an account.
func (s *StateDB) SubBalance(addr types.Address, amount uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accounts[addr]
	if !ok || acc.Balance < amount {
		return fmt.Errorf("insufficient balance: have %d, need %d", s.safeBalance(addr), amount)
	}
	acc.Balance -= amount
	return nil
}

func (s *StateDB) safeBalance(addr types.Address) uint64 {
	if acc, ok := s.accounts[addr]; ok {
		return acc.Balance
	}
	return 0
}

// AllAccounts returns a copy of all accounts (for Merkle root computation).
func (s *StateDB) AllAccounts() map[types.Address]*types.Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cloneAccounts()
}
