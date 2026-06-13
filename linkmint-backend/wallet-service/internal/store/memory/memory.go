// Package memory is an in-memory domain.Store for unit tests (domain/server/consumer). It mirrors
// the postgres store's projection math and DbDedupe semantics without needing a database.
package memory

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/paylink/wallet-service/internal/domain"
)

// Store is a goroutine-safe in-memory domain.Store.
type Store struct {
	mu        sync.Mutex
	seq       int
	cache     map[string]domain.Account
	txs       []domain.Transaction
	positions map[string]domain.Position
	rewards   []domain.Reward
	treasury  domain.TreasuryStats
	processed map[string]bool
}

// New builds an empty Store.
func New() *Store {
	return &Store{
		cache:     map[string]domain.Account{},
		positions: map[string]domain.Position{},
		treasury: domain.TreasuryStats{
			TotalSupply: big.NewInt(0), MaxSupply: big.NewInt(0), TotalBurned: big.NewInt(0),
			FeesCollected: big.NewInt(0), ValidatorRewards: big.NewInt(0), TreasuryAmount: big.NewInt(0),
		},
		processed: map[string]bool{},
	}
}

func (s *Store) Ping(context.Context) error { return nil }

func bi(x *big.Int) *big.Int {
	if x == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(x)
}

func maxBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) >= 0 {
		return new(big.Int).Set(a)
	}
	return new(big.Int).Set(b)
}

// once marks (scope,key) and reports whether the action should run (true = first time).
func (s *Store) once(scope, key string) bool {
	k := scope + "\x00" + key
	if s.processed[k] {
		return false
	}
	s.processed[k] = true
	return true
}

// nextID returns a monotonically increasing zero-padded id (stable ordering for tests).
func (s *Store) nextID() string {
	s.seq++
	return fmt.Sprintf("%020d", s.seq)
}

func (s *Store) addTx(addr, counterparty, direction, kind string, amount *big.Int, txHash string, bh uint64, at time.Time) {
	s.txs = append(s.txs, domain.Transaction{
		ID: s.nextID(), Addr: addr, Counterparty: counterparty, Direction: direction, Kind: kind,
		Amount: bi(amount), TxHash: txHash, BlockHeight: bh, OccurredAt: at,
	})
}

func (s *Store) ensurePosition(addr string, at time.Time) domain.Position {
	p, ok := s.positions[addr]
	if !ok {
		p = domain.Position{
			Addr: addr, StakedAmount: big.NewInt(0), PendingWithdrawal: big.NewInt(0),
			TotalRewards: big.NewInt(0), TotalSlashed: big.NewInt(0), UpdatedAt: at,
		}
	}
	return p
}

// ── projections ──

func (s *Store) RecordTransfer(_ context.Context, ev domain.TransferEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.once("chain.account.transfer", ev.TxHash+":"+ev.From+":"+ev.To) {
		return false, nil
	}
	s.addTx(ev.From, ev.To, "out", domain.KindTransfer, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	s.addTx(ev.To, ev.From, "in", domain.KindTransfer, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	return true, nil
}

func (s *Store) RecordStaked(_ context.Context, ev domain.StakedEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.once("chain.validator.staked", ev.TxHash+":"+ev.Addr) {
		return false, nil
	}
	p := s.ensurePosition(ev.Addr, ev.OccurredAt)
	p.StakedAmount = bi(ev.TotalStaked)
	p.IsActive = ev.IsActive
	p.UpdatedAt = ev.OccurredAt
	s.positions[ev.Addr] = p
	s.addTx(ev.Addr, "", "out", domain.KindStake, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	return true, nil
}

func (s *Store) RecordUnstakeStarted(_ context.Context, ev domain.UnstakeStartedEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.once("chain.validator.unstake_started", ev.TxHash+":"+ev.Addr) {
		return false, nil
	}
	p := s.ensurePosition(ev.Addr, ev.OccurredAt)
	p.PendingWithdrawal = new(big.Int).Add(p.PendingWithdrawal, bi(ev.Amount))
	p.StakedAmount = maxBig(big.NewInt(0), new(big.Int).Sub(p.StakedAmount, bi(ev.Amount)))
	p.WithdrawableAt = ev.WithdrawableAt
	p.UpdatedAt = ev.OccurredAt
	s.positions[ev.Addr] = p
	s.addTx(ev.Addr, "", "self", domain.KindUnstakeStart, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	return true, nil
}

func (s *Store) RecordUnstakeCompleted(_ context.Context, ev domain.UnstakeCompletedEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.once("chain.validator.unstake_completed", ev.TxHash+":"+ev.Addr) {
		return false, nil
	}
	p := s.ensurePosition(ev.Addr, ev.OccurredAt)
	p.PendingWithdrawal = maxBig(big.NewInt(0), new(big.Int).Sub(p.PendingWithdrawal, bi(ev.Amount)))
	p.UpdatedAt = ev.OccurredAt
	s.positions[ev.Addr] = p
	s.addTx(ev.Addr, "", "in", domain.KindUnstakeComplete, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	return true, nil
}

func (s *Store) RecordSlashed(_ context.Context, ev domain.SlashedEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.once("chain.validator.slashed", ev.TxHash+":"+ev.Addr) {
		return false, nil
	}
	p := s.ensurePosition(ev.Addr, ev.OccurredAt)
	p.TotalSlashed = new(big.Int).Add(p.TotalSlashed, bi(ev.Amount))
	p.StakedAmount = bi(ev.Remaining)
	p.UpdatedAt = ev.OccurredAt
	s.positions[ev.Addr] = p
	s.addTx(ev.Addr, "", "out", domain.KindSlash, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	return true, nil
}

func (s *Store) RecordRewarded(_ context.Context, ev domain.RewardedEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.once("chain.validator.rewarded", ev.TxHash+":"+ev.Addr) {
		return false, nil
	}
	p := s.ensurePosition(ev.Addr, ev.OccurredAt)
	p.TotalRewards = bi(ev.TotalRewards)
	p.UpdatedAt = ev.OccurredAt
	s.positions[ev.Addr] = p
	s.rewards = append(s.rewards, domain.Reward{
		ID: s.nextID(), Addr: ev.Addr, Amount: bi(ev.Amount), TotalRewards: bi(ev.TotalRewards),
		Source: domain.SourceValidatorReward, TxHash: ev.TxHash, BlockHeight: ev.BlockHeight, OccurredAt: ev.OccurredAt,
	})
	s.addTx(ev.Addr, "", "in", domain.KindReward, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	return true, nil
}

func (s *Store) RecordFeeCollected(_ context.Context, ev domain.FeeCollectedEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := ev.TxHash
	if key == "" {
		key = "fee:" + ev.OccurredAt.UTC().Format(time.RFC3339Nano)
	}
	if !s.once("chain.fee.collected", key) {
		return false, nil
	}
	s.treasury.FeesCollected = new(big.Int).Add(s.treasury.FeesCollected, bi(ev.TotalFee))
	s.treasury.ValidatorRewards = new(big.Int).Add(s.treasury.ValidatorRewards, bi(ev.ValidatorShare))
	s.treasury.TreasuryAmount = new(big.Int).Add(s.treasury.TreasuryAmount, bi(ev.TreasuryShare))
	if ev.BlockHeight > s.treasury.ChainHeight {
		s.treasury.ChainHeight = ev.BlockHeight
	}
	s.treasury.UpdatedAt = ev.OccurredAt
	return true, nil
}

func (s *Store) RecordFeeDistributed(_ context.Context, ev domain.FeeDistributedEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.once("chain.fee.distributed", ev.TxHash+":"+ev.Validator) {
		return false, nil
	}
	s.rewards = append(s.rewards, domain.Reward{
		ID: s.nextID(), Addr: ev.Validator, Amount: bi(ev.Amount), TotalRewards: big.NewInt(0),
		Source: domain.SourceFeeShare, TxHash: ev.TxHash, BlockHeight: ev.BlockHeight, OccurredAt: ev.OccurredAt,
	})
	return true, nil
}

func (s *Store) RecordTokenBurned(_ context.Context, ev domain.TokenBurnedEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := ev.TxHash
	if key == "" {
		key = "burn:" + ev.OccurredAt.UTC().Format(time.RFC3339Nano)
	}
	if !s.once("chain.token.burned", key) {
		return false, nil
	}
	s.treasury.TotalBurned = bi(ev.TotalBurned)
	if ev.BlockHeight > s.treasury.ChainHeight {
		s.treasury.ChainHeight = ev.BlockHeight
	}
	s.treasury.UpdatedAt = ev.OccurredAt
	return true, nil
}

// ── reads ──

func (s *Store) GetAccountCache(_ context.Context, addr string) (domain.Account, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.cache[addr]
	return a, ok, nil
}

func (s *Store) UpsertAccountCache(_ context.Context, a domain.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := a
	cp.Balance = bi(a.Balance)
	s.cache[a.Addr] = cp
	return nil
}

func (s *Store) ListTransactions(_ context.Context, addr string, limit int, cursor string) ([]domain.Transaction, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit = domain.ClampLimit(limit)
	var rows []domain.Transaction
	for _, t := range s.txs {
		if t.Addr == addr {
			rows = append(rows, t)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].BlockHeight != rows[j].BlockHeight {
			return rows[i].BlockHeight > rows[j].BlockHeight
		}
		return rows[i].ID > rows[j].ID
	})
	rows = afterCursor(rows, cursor, func(t domain.Transaction) (uint64, string) { return t.BlockHeight, t.ID })
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
	}
	if len(rows) == limit {
		last := rows[len(rows)-1]
		next = domain.EncodeCursor(last.BlockHeight, last.ID)
	}
	return rows, next, nil
}

func (s *Store) ListRewards(_ context.Context, addr string, limit int, cursor string) ([]domain.Reward, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit = domain.ClampLimit(limit)
	var rows []domain.Reward
	for _, r := range s.rewards {
		if r.Addr == addr {
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].BlockHeight != rows[j].BlockHeight {
			return rows[i].BlockHeight > rows[j].BlockHeight
		}
		return rows[i].ID > rows[j].ID
	})
	rows = afterCursor(rows, cursor, func(r domain.Reward) (uint64, string) { return r.BlockHeight, r.ID })
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
	}
	if len(rows) == limit {
		last := rows[len(rows)-1]
		next = domain.EncodeCursor(last.BlockHeight, last.ID)
	}
	return rows, next, nil
}

// afterCursor drops rows up to and including the keyset cursor (rows must be sorted newest-first).
func afterCursor[T any](rows []T, cursor string, key func(T) (uint64, string)) []T {
	bh, id, ok := domain.DecodeCursor(cursor)
	if !ok {
		return rows
	}
	out := rows[:0:0]
	for _, r := range rows {
		rbh, rid := key(r)
		if rbh < bh || (rbh == bh && rid < id) {
			out = append(out, r)
		}
	}
	return out
}

func (s *Store) GetPosition(_ context.Context, addr string) (domain.Position, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.positions[addr]
	return p, ok, nil
}

func (s *Store) GetTreasuryStats(context.Context) (domain.TreasuryStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.treasury
	t.TotalSupply = bi(s.treasury.TotalSupply)
	t.MaxSupply = bi(s.treasury.MaxSupply)
	t.TotalBurned = bi(s.treasury.TotalBurned)
	t.FeesCollected = bi(s.treasury.FeesCollected)
	t.ValidatorRewards = bi(s.treasury.ValidatorRewards)
	t.TreasuryAmount = bi(s.treasury.TreasuryAmount)
	return t, nil
}
