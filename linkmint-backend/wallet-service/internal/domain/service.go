package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"regexp"
	"strings"
	"time"

	lvm "github.com/paylink/paylink-chain/pkg/lvm"

	"github.com/paylink/wallet-service/internal/chainrpc"
)

// addrRe matches a 0x-prefixed 20-byte hex address.
var addrRe = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

// Service is the wallet-service read orchestration. It owns no mutating state: reads come from the
// Store (the indexed read-side) with a read-through to the chain for live balance/nonce, and the
// intent builder is a pure function of (addr, action, amount, nonce, chainID).
type Service struct {
	store    Store
	chain    ChainReader
	m        Recorder
	log      *slog.Logger
	chainID  string
	cacheTTL time.Duration
	now      func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithChainID sets the fallback chain id used by BuildIntent when the live id is unknown.
func WithChainID(id string) Option { return func(s *Service) { s.chainID = id } }

// WithMetrics attaches a metrics recorder.
func WithMetrics(m Recorder) Option { return func(s *Service) { s.m = m } }

// WithBalanceCacheTTL sets how long a cached balance row is served before re-hitting the chain.
func WithBalanceCacheTTL(d time.Duration) Option { return func(s *Service) { s.cacheTTL = d } }

// WithNowFunc overrides the clock (used by tests to drive the balance-cache TTL deterministically).
func WithNowFunc(fn func() time.Time) Option { return func(s *Service) { s.now = fn } }

// NewService builds a Service.
func NewService(store Store, chain ChainReader, log *slog.Logger, opts ...Option) *Service {
	if log == nil {
		log = slog.Default()
	}
	s := &Service{store: store, chain: chain, log: log, cacheTTL: 5 * time.Second, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// NormalizeAddr validates and lower-cases a 0x-address. Returns ErrInvalidAddress on a bad format.
func NormalizeAddr(addr string) (string, error) {
	a := strings.TrimSpace(addr)
	if !addrRe.MatchString(a) {
		return "", ErrInvalidAddress
	}
	return strings.ToLower(a), nil
}

// GetWallet returns the balance + nonce for addr, read-through cached from chain truth. Within the
// cache TTL the stored row is served; otherwise the chain is consulted and the cache refreshed. When
// the chain is unreachable, a cached row is served with Stale=true; with no cached row, the call
// fails with ErrChainUnavailable.
func (s *Service) GetWallet(ctx context.Context, addr string) (Account, error) {
	a, err := NormalizeAddr(addr)
	if err != nil {
		return Account{}, err
	}

	cached, hasCache, cerr := s.store.GetAccountCache(ctx, a)
	if cerr != nil {
		return Account{}, cerr
	}
	if hasCache && s.now().Sub(cached.FetchedAt) < s.cacheTTL {
		cached.Stale = false
		return cached, nil
	}

	acc, rerr := s.chain.GetAccount(ctx, a)
	if rerr != nil {
		if errors.Is(rerr, chainrpc.ErrUnavailable) {
			if hasCache {
				cached.Stale = true
				return cached, nil
			}
			return Account{}, ErrChainUnavailable
		}
		return Account{}, rerr
	}

	fresh := Account{
		Addr:      a,
		Balance:   new(big.Int).SetUint64(acc.Balance),
		Nonce:     acc.Nonce,
		FetchedAt: s.now().UTC(),
	}
	if uerr := s.store.UpsertAccountCache(ctx, fresh); uerr != nil {
		// A cache-write failure must not fail the read — the value is still chain-fresh.
		s.log.Warn("balance_cache_upsert_failed", "addr", a, "err", uerr.Error())
	}
	return fresh, nil
}

// ListTransactions returns a page of an address's transaction history (newest first).
func (s *Service) ListTransactions(ctx context.Context, addr string, limit int, cursor string) ([]Transaction, string, error) {
	a, err := NormalizeAddr(addr)
	if err != nil {
		return nil, "", err
	}
	return s.store.ListTransactions(ctx, a, limit, cursor)
}

// GetPosition returns an address's staking position, or ErrNotFound when it has never staked.
func (s *Service) GetPosition(ctx context.Context, addr string) (Position, error) {
	a, err := NormalizeAddr(addr)
	if err != nil {
		return Position{}, err
	}
	p, ok, perr := s.store.GetPosition(ctx, a)
	if perr != nil {
		return Position{}, perr
	}
	if !ok {
		return Position{}, ErrNotFound
	}
	return p, nil
}

// ListRewards returns a page of an address's reward history (newest first).
func (s *Service) ListRewards(ctx context.Context, addr string, limit int, cursor string) ([]Reward, string, error) {
	a, err := NormalizeAddr(addr)
	if err != nil {
		return nil, "", err
	}
	return s.store.ListRewards(ctx, a, limit, cursor)
}

// GetTreasuryStats returns the public treasury aggregate, enriching supply figures live from the
// chain when reachable (best-effort; falls back to the projected snapshot).
func (s *Service) GetTreasuryStats(ctx context.Context) (TreasuryStats, error) {
	stats, err := s.store.GetTreasuryStats(ctx)
	if err != nil {
		return TreasuryStats{}, err
	}
	if ts, terr := s.chain.TokenStats(ctx); terr == nil {
		stats.TotalSupply = new(big.Int).SetUint64(ts.TotalSupply)
		stats.MaxSupply = new(big.Int).SetUint64(ts.MaxSupply)
	}
	return stats, nil
}

// BuildIntent assembles an UNSIGNED stake/unstake transaction plus the bytes the client signs and a
// fee estimate. It holds NO keys (A.1): the signature/pubkey/hash are left zero for the client to
// fill. The live nonce comes from the chain (the one read-side endpoint that needs the chain up).
func (s *Service) BuildIntent(ctx context.Context, req IntentRequest) (Intent, error) {
	a, err := NormalizeAddr(req.Addr)
	if err != nil {
		return Intent{}, err
	}
	if req.Amount == nil || req.Amount.Sign() <= 0 {
		return Intent{}, ErrInvalidAmount
	}
	if req.Amount.Cmp(new(big.Int).SetUint64(math.MaxUint64)) > 0 {
		return Intent{}, fmt.Errorf("%w: exceeds uint64 range", ErrInvalidAmount)
	}
	amt := req.Amount.Uint64()

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "stake" && action != "unstake" {
		return Intent{}, ErrInvalidAction
	}

	nonce, nerr := s.chain.GetNonce(ctx, a)
	if nerr != nil {
		if errors.Is(nerr, chainrpc.ErrUnavailable) {
			return Intent{}, ErrChainUnavailable
		}
		return Intent{}, nerr
	}

	chainID := s.chainID
	if ci, cerr := s.chain.ChainInfo(ctx); cerr == nil && ci.ChainID != "" {
		chainID = ci.ChainID
	}

	from := lvm.HexToAddress(a)
	var tx *lvm.Transaction
	var berr error
	switch action {
	case "stake":
		tx, berr = lvm.BuildStakeTx(from, nonce, amt)
	case "unstake":
		tx, berr = lvm.BuildInitiateUnstakeTx(from, nonce, amt)
	}
	if berr != nil {
		return Intent{}, berr
	}

	if s.m != nil {
		s.m.IntentBuilt(action)
	}
	return Intent{
		Tx:            tx,
		SignableBytes: tx.SignableBytes(),
		Nonce:         nonce,
		ChainID:       chainID,
		FeeEstimate: FeeEstimate{
			Amount:   big.NewInt(0),
			Currency: "PLN",
			Policy:   "no-protocol-fee-on-stake-txs",
		},
	}, nil
}

// ── Consumer-facing handlers (thin wrappers over the store projections) ──

func result(ran bool) string {
	if ran {
		return ResultProcessed
	}
	return ResultDuplicate
}

func (s *Service) HandleTransfer(ctx context.Context, ev TransferEvent) (string, error) {
	ran, err := s.store.RecordTransfer(ctx, ev)
	return result(ran), err
}

func (s *Service) HandleStaked(ctx context.Context, ev StakedEvent) (string, error) {
	ran, err := s.store.RecordStaked(ctx, ev)
	return result(ran), err
}

func (s *Service) HandleUnstakeStarted(ctx context.Context, ev UnstakeStartedEvent) (string, error) {
	ran, err := s.store.RecordUnstakeStarted(ctx, ev)
	return result(ran), err
}

func (s *Service) HandleUnstakeCompleted(ctx context.Context, ev UnstakeCompletedEvent) (string, error) {
	ran, err := s.store.RecordUnstakeCompleted(ctx, ev)
	return result(ran), err
}

func (s *Service) HandleSlashed(ctx context.Context, ev SlashedEvent) (string, error) {
	ran, err := s.store.RecordSlashed(ctx, ev)
	return result(ran), err
}

func (s *Service) HandleRewarded(ctx context.Context, ev RewardedEvent) (string, error) {
	ran, err := s.store.RecordRewarded(ctx, ev)
	return result(ran), err
}

func (s *Service) HandleFeeCollected(ctx context.Context, ev FeeCollectedEvent) (string, error) {
	ran, err := s.store.RecordFeeCollected(ctx, ev)
	return result(ran), err
}

func (s *Service) HandleFeeDistributed(ctx context.Context, ev FeeDistributedEvent) (string, error) {
	ran, err := s.store.RecordFeeDistributed(ctx, ev)
	return result(ran), err
}

func (s *Service) HandleTokenBurned(ctx context.Context, ev TokenBurnedEvent) (string, error) {
	ran, err := s.store.RecordTokenBurned(ctx, ev)
	return result(ran), err
}
