package state

import (
	"fmt"
	"sync"

	"github.com/paylink/paylink-chain/internal/types"
)

// StateDB holds all in-memory chain state with snapshot/revert support.
type StateDB struct {
	mu sync.RWMutex

	// Core state maps
	accounts   map[types.Address]*types.Account
	paylinks   map[types.Hash]*types.PayLink
	validators map[types.Address]*types.ValidatorInfo

	// Validation tracking
	usedProofs      map[types.Hash]bool       // anti-replay
	submittedProofs map[types.Hash]types.Hash  // paylink ID -> first proof hash
	votes           map[voteKey]bool           // per-validator votes

	// Indexes for queries
	paylinksByCreator  map[types.Address][]types.Hash   // creator -> paylink IDs
	paylinksByReceiver map[types.Address][]types.Hash   // receiver -> paylink IDs
	paylinksByStatus   map[types.Status][]types.Hash    // status -> paylink IDs
	paylinksByOwner    map[types.Address][]types.Hash   // owner -> paylink IDs

	// NFT-style operator approvals (owner+operator -> approved)
	operatorApprovals map[operatorKey]bool

	// Validator ordering
	validatorList []types.Address

	// VRF key registry (Phase 2)
	vrfKeys map[types.Address][]byte // validator address -> VRF public key (32 bytes)

	// Chain parameters (from genesis)
	totalSupply         uint64
	maxSupply           uint64
	minimumStake        uint64
	withdrawalCooldown  int64  // seconds
	requiredValidations uint64
	adminAddress        types.Address

	// Phase 2: Fee and treasury parameters
	treasuryAddress      types.Address
	feeRateBasisPoints   uint64  // 50 = 0.5%
	validatorRewardShare uint64  // 70%
	treasuryShare        uint64  // 20%
	burnShare            uint64  // 10%
	targetCommitteeSize  int
	quorumFraction       float64
	totalBurned          uint64

	// Snapshot support
	snapshots []snapshot
}

type voteKey struct {
	PayLinkID types.Hash
	Validator types.Address
}

type operatorKey struct {
	Owner    types.Address
	Operator types.Address
}

type snapshot struct {
	accounts           map[types.Address]*types.Account
	paylinks           map[types.Hash]*types.PayLink
	validators         map[types.Address]*types.ValidatorInfo
	usedProofs         map[types.Hash]bool
	submittedProofs    map[types.Hash]types.Hash
	votes              map[voteKey]bool
	paylinksByCreator  map[types.Address][]types.Hash
	paylinksByReceiver map[types.Address][]types.Hash
	paylinksByStatus   map[types.Status][]types.Hash
	paylinksByOwner    map[types.Address][]types.Hash
	operatorApprovals  map[operatorKey]bool
	validatorList      []types.Address
	vrfKeys            map[types.Address][]byte
	totalSupply        uint64
	totalBurned        uint64
}

// NewStateDB creates a new empty state database.
func NewStateDB(genesis *types.GenesisConfig) *StateDB {
	s := &StateDB{
		accounts:           make(map[types.Address]*types.Account),
		paylinks:           make(map[types.Hash]*types.PayLink),
		validators:         make(map[types.Address]*types.ValidatorInfo),
		usedProofs:         make(map[types.Hash]bool),
		submittedProofs:    make(map[types.Hash]types.Hash),
		votes:              make(map[voteKey]bool),
		paylinksByCreator:  make(map[types.Address][]types.Hash),
		paylinksByReceiver: make(map[types.Address][]types.Hash),
		paylinksByStatus:   make(map[types.Status][]types.Hash),
		paylinksByOwner:    make(map[types.Address][]types.Hash),
		operatorApprovals:  make(map[operatorKey]bool),
		validatorList:      make([]types.Address, 0),
		vrfKeys:            make(map[types.Address][]byte),
	}

	if genesis != nil {
		s.maxSupply = genesis.MaxSupply
		s.minimumStake = genesis.MinimumStake
		s.withdrawalCooldown = genesis.WithdrawalCooldown
		s.requiredValidations = genesis.RequiredValidations
		s.adminAddress = genesis.AdminAddress

		// Phase 2 parameters with defaults
		s.treasuryAddress = genesis.TreasuryAddress
		s.feeRateBasisPoints = genesis.FeeRateBasisPoints
		if s.feeRateBasisPoints == 0 {
			s.feeRateBasisPoints = 50 // 0.5% default
		}
		s.validatorRewardShare = genesis.ValidatorRewardShare
		if s.validatorRewardShare == 0 {
			s.validatorRewardShare = 70
		}
		s.treasuryShare = genesis.TreasuryShare
		if s.treasuryShare == 0 {
			s.treasuryShare = 20
		}
		s.burnShare = genesis.BurnShare
		if s.burnShare == 0 {
			s.burnShare = 10
		}
		s.targetCommitteeSize = genesis.TargetCommitteeSize
		if s.targetCommitteeSize == 0 {
			s.targetCommitteeSize = 5
		}
		s.quorumFraction = genesis.QuorumFraction
		if s.quorumFraction == 0 {
			s.quorumFraction = 0.6
		}

		// Fund initial balances
		for _, b := range genesis.InitialBalances {
			s.accounts[b.Address] = &types.Account{
				Balance: b.Balance,
				Nonce:   0,
			}
			s.totalSupply += b.Balance
		}
	}

	return s
}

// Snapshot takes a snapshot of the current state. Returns the snapshot ID.
func (s *StateDB) Snapshot() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := snapshot{
		accounts:           s.cloneAccounts(),
		paylinks:           s.clonePaylinks(),
		validators:         s.cloneValidators(),
		usedProofs:         s.cloneUsedProofs(),
		submittedProofs:    s.cloneSubmittedProofs(),
		votes:              s.cloneVotes(),
		paylinksByCreator:  s.cloneHashSliceMap(s.paylinksByCreator),
		paylinksByReceiver: s.cloneHashSliceMap(s.paylinksByReceiver),
		paylinksByStatus:   s.cloneStatusIndex(),
		paylinksByOwner:    s.cloneHashSliceMap(s.paylinksByOwner),
		operatorApprovals:  s.cloneOperatorApprovals(),
		validatorList:      s.cloneValidatorList(),
		vrfKeys:            s.cloneVRFKeys(),
		totalSupply:        s.totalSupply,
		totalBurned:        s.totalBurned,
	}
	s.snapshots = append(s.snapshots, snap)
	return len(s.snapshots) - 1
}

// Revert restores the state to a previous snapshot.
func (s *StateDB) Revert(snapID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if snapID < 0 || snapID >= len(s.snapshots) {
		return fmt.Errorf("invalid snapshot ID: %d", snapID)
	}

	snap := s.snapshots[snapID]
	s.accounts = snap.accounts
	s.paylinks = snap.paylinks
	s.validators = snap.validators
	s.usedProofs = snap.usedProofs
	s.submittedProofs = snap.submittedProofs
	s.votes = snap.votes
	s.paylinksByCreator = snap.paylinksByCreator
	s.paylinksByReceiver = snap.paylinksByReceiver
	s.paylinksByStatus = snap.paylinksByStatus
	s.paylinksByOwner = snap.paylinksByOwner
	s.operatorApprovals = snap.operatorApprovals
	s.validatorList = snap.validatorList
	s.vrfKeys = snap.vrfKeys
	s.totalSupply = snap.totalSupply
	s.totalBurned = snap.totalBurned

	// Discard all snapshots after this one
	s.snapshots = s.snapshots[:snapID]
	return nil
}

// DiscardSnapshots removes all snapshots (call after committing a block).
func (s *StateDB) DiscardSnapshots() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = s.snapshots[:0]
}

// AdminAddress returns the admin address.
func (s *StateDB) AdminAddress() types.Address {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.adminAddress
}

// RequiredValidations returns the required validation count.
func (s *StateDB) RequiredValidations() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.requiredValidations
}

// MinimumStake returns the minimum stake amount.
func (s *StateDB) MinimumStake() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.minimumStake
}

// WithdrawalCooldown returns the withdrawal cooldown in seconds.
func (s *StateDB) WithdrawalCooldown() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.withdrawalCooldown
}

// TotalSupply returns the current total supply.
func (s *StateDB) TotalSupply() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalSupply
}

// MaxSupply returns the maximum supply.
func (s *StateDB) MaxSupply() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxSupply
}

// ── Clone helpers for snapshot ──

func (s *StateDB) cloneAccounts() map[types.Address]*types.Account {
	c := make(map[types.Address]*types.Account, len(s.accounts))
	for k, v := range s.accounts {
		cp := *v
		c[k] = &cp
	}
	return c
}

func (s *StateDB) clonePaylinks() map[types.Hash]*types.PayLink {
	c := make(map[types.Hash]*types.PayLink, len(s.paylinks))
	for k, v := range s.paylinks {
		cp := *v
		if len(v.Rules) > 0 {
			cp.Rules = make([]byte, len(v.Rules))
			copy(cp.Rules, v.Rules)
		}
		c[k] = &cp
	}
	return c
}

func (s *StateDB) cloneValidators() map[types.Address]*types.ValidatorInfo {
	c := make(map[types.Address]*types.ValidatorInfo, len(s.validators))
	for k, v := range s.validators {
		cp := *v
		c[k] = &cp
	}
	return c
}

func (s *StateDB) cloneUsedProofs() map[types.Hash]bool {
	c := make(map[types.Hash]bool, len(s.usedProofs))
	for k, v := range s.usedProofs {
		c[k] = v
	}
	return c
}

func (s *StateDB) cloneSubmittedProofs() map[types.Hash]types.Hash {
	c := make(map[types.Hash]types.Hash, len(s.submittedProofs))
	for k, v := range s.submittedProofs {
		c[k] = v
	}
	return c
}

func (s *StateDB) cloneVotes() map[voteKey]bool {
	c := make(map[voteKey]bool, len(s.votes))
	for k, v := range s.votes {
		c[k] = v
	}
	return c
}

func (s *StateDB) cloneValidatorList() []types.Address {
	c := make([]types.Address, len(s.validatorList))
	copy(c, s.validatorList)
	return c
}

func (s *StateDB) cloneHashSliceMap(m map[types.Address][]types.Hash) map[types.Address][]types.Hash {
	c := make(map[types.Address][]types.Hash, len(m))
	for k, v := range m {
		sl := make([]types.Hash, len(v))
		copy(sl, v)
		c[k] = sl
	}
	return c
}

func (s *StateDB) cloneStatusIndex() map[types.Status][]types.Hash {
	c := make(map[types.Status][]types.Hash, len(s.paylinksByStatus))
	for k, v := range s.paylinksByStatus {
		sl := make([]types.Hash, len(v))
		copy(sl, v)
		c[k] = sl
	}
	return c
}

func (s *StateDB) cloneOperatorApprovals() map[operatorKey]bool {
	c := make(map[operatorKey]bool, len(s.operatorApprovals))
	for k, v := range s.operatorApprovals {
		c[k] = v
	}
	return c
}

func (s *StateDB) cloneVRFKeys() map[types.Address][]byte {
	c := make(map[types.Address][]byte, len(s.vrfKeys))
	for k, v := range s.vrfKeys {
		cp := make([]byte, len(v))
		copy(cp, v)
		c[k] = cp
	}
	return c
}

// ── Phase 2 accessors ──

// TreasuryAddress returns the treasury address.
func (s *StateDB) TreasuryAddress() types.Address {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.treasuryAddress
}

// FeeRateBasisPoints returns the fee rate in basis points (50 = 0.5%).
func (s *StateDB) FeeRateBasisPoints() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.feeRateBasisPoints
}

// ValidatorRewardShare returns the validator reward share percentage.
func (s *StateDB) ValidatorRewardShare() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.validatorRewardShare
}

// TreasurySharePct returns the treasury share percentage.
func (s *StateDB) TreasurySharePct() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.treasuryShare
}

// BurnShare returns the burn share percentage.
func (s *StateDB) BurnShare() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.burnShare
}

// TargetCommitteeSize returns the target committee size.
func (s *StateDB) TargetCommitteeSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.targetCommitteeSize
}

// QuorumFraction returns the quorum fraction (e.g. 0.6 = 3 of 5).
func (s *StateDB) QuorumFraction() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.quorumFraction
}

// TotalBurned returns the total amount of tokens burned.
func (s *StateDB) TotalBurned() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalBurned
}

// MintTokens creates new tokens and credits them to the given address.
func (s *StateDB) MintTokens(to types.Address, amount uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.totalSupply+amount > s.maxSupply {
		return fmt.Errorf("mint would exceed max supply: current %d + %d > %d", s.totalSupply, amount, s.maxSupply)
	}

	acc := s.accounts[to]
	if acc == nil {
		acc = &types.Account{}
		s.accounts[to] = acc
	}
	acc.Balance += amount
	s.totalSupply += amount
	return nil
}

// BurnTokens destroys tokens from total supply.
func (s *StateDB) BurnTokens(amount uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalBurned += amount
	if s.totalSupply >= amount {
		s.totalSupply -= amount
	}
}
