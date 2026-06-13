// Package memory is an in-memory domain.Store for unit tests. It mirrors the postgres store's state
// bookkeeping and aggregation arithmetic (gross/fee/net, settlement lifecycle, payout scheduling,
// rail-file matching, clawback offsets) but does NOT post ledger entries — ledger balance (A.6) is
// asserted against the postgres store in integration tests.
package memory

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/paylink/settlement-service/internal/domain"
)

// Store is a concurrency-safe in-memory domain.Store.
type Store struct {
	mu          sync.Mutex
	settlements map[string]*domain.Settlement
	periodIndex map[string]string // merchant_key|currency|date -> settlement id
	items       map[string][]domain.SettlementItem
	paylinkItem map[string]string // pl_id -> settlement id (paylink items only)
	payouts     map[string]*domain.Payout
	payoutBySet map[string]string // settlement id -> payout id
	merchants   map[string]domain.Merchant
	bankAccts   map[string]domain.BankAccount
	railFiles   map[string]domain.IngestResult
	processed   map[string]bool // dedupe: scope|key
}

// New builds an empty Store.
func New() *Store {
	return &Store{
		settlements: map[string]*domain.Settlement{},
		periodIndex: map[string]string{},
		items:       map[string][]domain.SettlementItem{},
		paylinkItem: map[string]string{},
		payouts:     map[string]*domain.Payout{},
		payoutBySet: map[string]string{},
		merchants:   map[string]domain.Merchant{},
		bankAccts:   map[string]domain.BankAccount{},
		railFiles:   map[string]domain.IngestResult{},
		processed:   map[string]bool{},
	}
}

func (s *Store) Ping(context.Context) error { return nil }
func (s *Store) Close()                     {}

// seen marks (scope,key) and reports whether it was already present (duplicate).
func (s *Store) seen(scope, key string) bool {
	k := scope + "|" + key
	if s.processed[k] {
		return true
	}
	s.processed[k] = true
	return false
}

func (s *Store) upsertOpen(merchantKey, currency, date string, cutoff time.Time) (string, bool) {
	idxKey := merchantKey + "|" + currency + "|" + date
	if id, ok := s.periodIndex[idxKey]; ok {
		return id, false
	}
	id := uuid.NewString()
	s.settlements[id] = &domain.Settlement{
		ID: id, MerchantKey: merchantKey, Currency: currency, SettlementDate: date,
		Status: domain.StatusOpen, Gross: big.NewInt(0), PlatformFee: big.NewInt(0),
		ChainFee: big.NewInt(0), Net: big.NewInt(0), CutoffAt: cutoff, OpenedAt: time.Now().UTC(),
	}
	s.periodIndex[idxKey] = id
	return id, true
}

// RecordVerified implements domain.Store.
func (s *Store) RecordVerified(_ context.Context, in domain.VerifiedRecord) (domain.VerifiedOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen("chain.paylink.verified", in.PLID) {
		return domain.VerifiedOutcome{Applied: false}, nil
	}
	sID, opened := s.upsertOpen(in.MerchantKey, in.Currency, in.SettlementDate, in.CutoffAt)
	net := new(big.Int).Sub(in.Gross, in.PlatformFee)
	s.items[sID] = append(s.items[sID], domain.SettlementItem{
		ID: uuid.NewString(), SettlementID: sID, PLID: in.PLID, Kind: domain.ItemPayLink,
		Gross: clone(in.Gross), PlatformFee: clone(in.PlatformFee), ChainFee: big.NewInt(0),
		Net: clone(net), VerifiedTxHash: in.TxHash, CreatedAt: time.Now().UTC(),
	})
	s.paylinkItem[in.PLID] = sID
	st := s.settlements[sID]
	st.Gross.Add(st.Gross, in.Gross)
	st.PlatformFee.Add(st.PlatformFee, in.PlatformFee)
	st.Net.Add(st.Net, net)
	return domain.VerifiedOutcome{Applied: true, Opened: opened, Settlement: *st}, nil
}

// RecordFee implements domain.Store.
func (s *Store) RecordFee(_ context.Context, in domain.FeeRecord) (domain.FeeOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sID, ok := s.paylinkItem[in.PLID]
	if !ok {
		return domain.FeeOutcome{Found: false}, nil
	}
	if s.seen("chain.fee.collected", in.PLID) {
		return domain.FeeOutcome{Applied: false, Found: true}, nil
	}
	for i := range s.items[sID] {
		it := &s.items[sID][i]
		if it.PLID == in.PLID && it.Kind == domain.ItemPayLink {
			it.ChainFee.Add(it.ChainFee, in.ChainFee)
			it.Net.Sub(it.Net, in.ChainFee)
			break
		}
	}
	st := s.settlements[sID]
	st.ChainFee.Add(st.ChainFee, in.ChainFee)
	st.Net.Sub(st.Net, in.ChainFee)
	return domain.FeeOutcome{Applied: true, Found: true}, nil
}

// RecordClawback implements domain.Store.
func (s *Store) RecordClawback(_ context.Context, in domain.ClawbackRecord) (domain.ClawbackOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	origID, ok := s.paylinkItem[in.PLID]
	if !ok {
		return domain.ClawbackOutcome{Found: false}, nil
	}
	if s.seen("refund.clawback.requested", in.RefundID) {
		return domain.ClawbackOutcome{Applied: false, Found: true}, nil
	}
	merchantKey := s.settlements[origID].MerchantKey
	currency := s.settlements[origID].Currency
	sID, _ := s.upsertOpen(merchantKey, currency, in.SettlementDate, in.CutoffAt)
	negNet := new(big.Int).Neg(in.Amount)
	s.items[sID] = append(s.items[sID], domain.SettlementItem{
		ID: uuid.NewString(), SettlementID: sID, PLID: in.PLID, Kind: domain.ItemClawback,
		Gross: big.NewInt(0), PlatformFee: big.NewInt(0), ChainFee: big.NewInt(0),
		Net: clone(negNet), CreatedAt: time.Now().UTC(),
	})
	st := s.settlements[sID]
	st.Net.Add(st.Net, negNet)
	return domain.ClawbackOutcome{Applied: true, Found: true}, nil
}

// UpsertMerchant implements domain.Store.
func (s *Store) UpsertMerchant(_ context.Context, m domain.Merchant) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen("merchant.onboarded", m.MerchantID) {
		return false, nil
	}
	s.merchants[m.MerchantID] = m
	return true, nil
}

// UpsertBankAccount implements domain.Store.
func (s *Store) UpsertBankAccount(_ context.Context, b domain.BankAccount) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen("merchant.bank_account.verified", b.BankAccountID) {
		return false, nil
	}
	s.bankAccts[b.BankAccountID] = b
	return true, nil
}

// CloseDueSettlements implements domain.Store.
func (s *Store) CloseDueSettlements(_ context.Context, now time.Time) ([]domain.Settlement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var closed []domain.Settlement
	for _, st := range s.settlements {
		if st.Status == domain.StatusOpen && !st.CutoffAt.After(now) {
			st.Status = domain.StatusClosed
			t := now.UTC()
			st.ClosedAt = &t
			closed = append(closed, *st)
		}
	}
	sort.Slice(closed, func(i, j int) bool { return closed[i].ID < closed[j].ID })
	return closed, nil
}

// SchedulePayouts implements domain.Store.
func (s *Store) SchedulePayouts(_ context.Context, now time.Time, opts domain.ScheduleOpts) ([]domain.Payout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rail := opts.DefaultRail
	if rail == "" {
		rail = "unknown"
	}
	var ids []string
	for id, st := range s.settlements {
		if st.Status != domain.StatusClosed {
			continue
		}
		if _, has := s.payoutBySet[id]; has {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []domain.Payout
	for _, id := range ids {
		st := s.settlements[id]
		min := big.NewInt(0)
		if opts.MinPayoutFor != nil {
			if m := opts.MinPayoutFor(st.Currency); m != nil {
				min = m
			}
		}
		if st.Net.Sign() <= 0 || st.Net.Cmp(min) < 0 {
			continue
		}
		out = append(out, s.createPayout(st, rail, now))
	}
	return out, nil
}

// CreatePayout implements domain.Store.
func (s *Store) CreatePayout(_ context.Context, settlementID, merchantKey, defaultRail string) (domain.Payout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.settlements[settlementID]
	if !ok || st.MerchantKey != merchantKey {
		return domain.Payout{}, domain.ErrNotFound
	}
	if st.Status != domain.StatusClosed {
		return domain.Payout{}, fmt.Errorf("%w: settlement must be CLOSED to pay out (is %s)", domain.ErrInvalidState, st.Status)
	}
	if st.Net.Sign() <= 0 {
		return domain.Payout{}, fmt.Errorf("%w: settlement net must be positive", domain.ErrInvalidAmount)
	}
	if _, has := s.payoutBySet[settlementID]; has {
		return domain.Payout{}, fmt.Errorf("%w: a payout already exists for this settlement", domain.ErrInvalidState)
	}
	rail := defaultRail
	if rail == "" {
		rail = "unknown"
	}
	return s.createPayout(st, rail, now()), nil
}

func (s *Store) createPayout(st *domain.Settlement, rail string, when time.Time) domain.Payout {
	id := uuid.NewString()
	instructed := when.UTC()
	p := domain.Payout{
		ID: id, SettlementID: st.ID, MerchantKey: st.MerchantKey, Rail: rail,
		Currency: st.Currency, Amount: clone(st.Net), Status: domain.PayoutInstructed,
		Reference: "PO-" + st.ID, ScheduledFor: st.CutoffAt, InstructedAt: &instructed,
	}
	cp := p
	s.payouts[id] = &cp
	s.payoutBySet[st.ID] = id
	return p
}

// IngestRailFile implements domain.Store.
func (s *Store) IngestRailFile(_ context.Context, in domain.RailFileInput) (domain.IngestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if res, ok := s.railFiles[in.FileID]; ok {
		res.PaidPayouts = nil
		return res, nil
	}
	var paid []domain.Payout
	matched := 0
	for _, line := range in.Lines {
		amt := line.Amount
		if amt == nil {
			amt = big.NewInt(0)
		}
		for _, p := range s.payouts {
			if p.Reference == line.Reference && p.Currency == line.Currency &&
				p.Amount.Cmp(amt) == 0 && (p.Status == domain.PayoutInstructed || p.Status == domain.PayoutScheduled) {
				p.Status = domain.PayoutPaid
				t := time.Now().UTC()
				p.PaidAt = &t
				if st, ok := s.settlements[p.SettlementID]; ok {
					st.Status = domain.StatusPaid
				}
				matched++
				paid = append(paid, *p)
				break
			}
		}
	}
	res := domain.IngestResult{
		FileID: in.FileID, Rail: in.Rail, LineCount: len(in.Lines),
		Matched: matched, Unmatched: len(in.Lines) - matched, PaidPayouts: paid,
	}
	stored := res
	stored.PaidPayouts = nil
	s.railFiles[in.FileID] = stored
	return res, nil
}

// GetSettlement implements domain.Store.
func (s *Store) GetSettlement(_ context.Context, id, merchantKey string) (domain.Settlement, []domain.SettlementItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.settlements[id]
	if !ok || st.MerchantKey != merchantKey {
		return domain.Settlement{}, nil, domain.ErrNotFound
	}
	items := append([]domain.SettlementItem(nil), s.items[id]...)
	return *st, items, nil
}

// ListSettlements implements domain.Store.
func (s *Store) ListSettlements(_ context.Context, merchantKey, status string, limit int) ([]domain.Settlement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Settlement
	for _, st := range s.settlements {
		if st.MerchantKey != merchantKey {
			continue
		}
		if status != "" && st.Status != status {
			continue
		}
		out = append(out, *st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenedAt.After(out[j].OpenedAt) })
	return limitSlice(out, limit), nil
}

// GetPayout implements domain.Store.
func (s *Store) GetPayout(_ context.Context, id, merchantKey string) (domain.Payout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.payouts[id]
	if !ok || p.MerchantKey != merchantKey {
		return domain.Payout{}, domain.ErrNotFound
	}
	return *p, nil
}

// ListPayouts implements domain.Store.
func (s *Store) ListPayouts(_ context.Context, merchantKey, status string, limit int) ([]domain.Payout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Payout
	for _, p := range s.payouts {
		if p.MerchantKey != merchantKey {
			continue
		}
		if status != "" && p.Status != status {
			continue
		}
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return limitSlice(out, limit), nil
}

func clone(v *big.Int) *big.Int {
	if v == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(v)
}

func limitSlice[T any](in []T, limit int) []T {
	if limit > 0 && len(in) > limit {
		return in[:limit]
	}
	return in
}

func now() time.Time { return time.Now() }
