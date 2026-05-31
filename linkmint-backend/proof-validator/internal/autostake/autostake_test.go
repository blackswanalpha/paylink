package autostake_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/pkg/lvm"
	"github.com/paylink/proof-validator/internal/autostake"
	"github.com/paylink/proof-validator/internal/chain"
)

type fakeChain struct {
	getValidator func() (*chain.ValidatorState, bool, error)
	account      *chain.AccountState
	accountErr   error
	stats        *chain.StakingStats
	statsErr     error
	sendErr      error
	sent         int
}

func (f *fakeChain) GetValidator(context.Context, string) (*chain.ValidatorState, bool, error) {
	return f.getValidator()
}
func (f *fakeChain) GetAccount(context.Context, string) (*chain.AccountState, error) {
	return f.account, f.accountErr
}
func (f *fakeChain) StakingStats(context.Context) (*chain.StakingStats, error) {
	return f.stats, f.statsErr
}
func (f *fakeChain) SendTransaction(context.Context, *lvm.Transaction) (string, error) {
	if f.sendErr != nil {
		return "", f.sendErr
	}
	f.sent++
	return "0xstake", nil
}

type fakeSigner struct{}

func (fakeSigner) Address() lvm.Address {
	return lvm.HexToAddress("0x00000000000000000000000000000000000000aa")
}
func (fakeSigner) SignTx(tx *lvm.Transaction) error {
	tx.Hash = lvm.SHA256Hash(tx.SignableBytes())
	tx.Signature = []byte{1}
	return nil
}

type fakeNonce struct{}

func (fakeNonce) Reserve(context.Context, string) (uint64, func(bool), error) {
	return 0, func(bool) {}, nil
}

func newBoot(t *testing.T, fc *fakeChain, amount uint64, timeout time.Duration) *autostake.Bootstrapper {
	t.Helper()
	return autostake.New(fc, fakeSigner{}, fakeNonce{}, nil, amount, time.Millisecond, timeout)
}

func TestEnsureActive_AlreadyActive_NoStake(t *testing.T) {
	fc := &fakeChain{getValidator: func() (*chain.ValidatorState, bool, error) {
		return &chain.ValidatorState{IsActive: true, StakedAmount: 1000}, true, nil
	}}
	if err := newBoot(t, fc, 0, time.Second).EnsureActive(context.Background()); err != nil {
		t.Fatalf("EnsureActive: %v", err)
	}
	if fc.sent != 0 {
		t.Fatalf("an already-active validator must not stake again; sent=%d", fc.sent)
	}
}

func TestEnsureActive_InactiveThenStakesAndBecomesActive(t *testing.T) {
	calls := 0
	fc := &fakeChain{
		getValidator: func() (*chain.ValidatorState, bool, error) {
			calls++
			if calls == 1 {
				return &chain.ValidatorState{IsActive: false}, true, nil // initial check
			}
			return &chain.ValidatorState{IsActive: true}, true, nil // poll after stake
		},
		account: &chain.AccountState{Balance: 5000},
		stats:   &chain.StakingStats{MinimumStake: 1000},
	}
	if err := newBoot(t, fc, 0, 2*time.Second).EnsureActive(context.Background()); err != nil {
		t.Fatalf("EnsureActive: %v", err)
	}
	if fc.sent != 1 {
		t.Fatalf("expected exactly one stake tx, got %d", fc.sent)
	}
}

func TestEnsureActive_NotFoundThenStakes(t *testing.T) {
	calls := 0
	fc := &fakeChain{
		getValidator: func() (*chain.ValidatorState, bool, error) {
			calls++
			if calls == 1 {
				return nil, false, nil // not a validator yet
			}
			return &chain.ValidatorState{IsActive: true}, true, nil
		},
		account: &chain.AccountState{Balance: 5000},
		stats:   &chain.StakingStats{MinimumStake: 1000},
	}
	if err := newBoot(t, fc, 0, 2*time.Second).EnsureActive(context.Background()); err != nil {
		t.Fatalf("EnsureActive: %v", err)
	}
	if fc.sent != 1 {
		t.Fatalf("expected one stake tx, got %d", fc.sent)
	}
}

func TestEnsureActive_InsufficientBalance(t *testing.T) {
	fc := &fakeChain{
		getValidator: func() (*chain.ValidatorState, bool, error) { return &chain.ValidatorState{IsActive: false}, true, nil },
		account:      &chain.AccountState{Balance: 10}, // < minimumStake
		stats:        &chain.StakingStats{MinimumStake: 1000},
	}
	err := newBoot(t, fc, 0, time.Second).EnsureActive(context.Background())
	if err == nil {
		t.Fatal("expected an insufficient-balance error")
	}
	if fc.sent != 0 {
		t.Fatalf("must not stake without sufficient balance; sent=%d", fc.sent)
	}
}

func TestEnsureActive_ExplicitAmountSkipsStatsLookup(t *testing.T) {
	calls := 0
	fc := &fakeChain{
		getValidator: func() (*chain.ValidatorState, bool, error) {
			calls++
			if calls == 1 {
				return &chain.ValidatorState{IsActive: false}, true, nil
			}
			return &chain.ValidatorState{IsActive: true}, true, nil
		},
		account: &chain.AccountState{Balance: 5000},
		// stats intentionally nil — must not be consulted when an explicit amount is given.
	}
	if err := newBoot(t, fc, 2000, 2*time.Second).EnsureActive(context.Background()); err != nil {
		t.Fatalf("EnsureActive: %v", err)
	}
	if fc.sent != 1 {
		t.Fatalf("expected one stake tx, got %d", fc.sent)
	}
}

func TestEnsureActive_TimesOut(t *testing.T) {
	fc := &fakeChain{
		getValidator: func() (*chain.ValidatorState, bool, error) { return &chain.ValidatorState{IsActive: false}, true, nil },
		account:      &chain.AccountState{Balance: 5000},
		stats:        &chain.StakingStats{MinimumStake: 1000},
	}
	err := newBoot(t, fc, 0, 20*time.Millisecond).EnsureActive(context.Background())
	if err == nil {
		t.Fatal("expected a timeout error when the validator never becomes active")
	}
}

func TestEnsureActive_StakingStatsError(t *testing.T) {
	fc := &fakeChain{
		getValidator: func() (*chain.ValidatorState, bool, error) { return &chain.ValidatorState{IsActive: false}, true, nil },
		statsErr:     errors.New("rpc down"),
	}
	if err := newBoot(t, fc, 0, time.Second).EnsureActive(context.Background()); err == nil {
		t.Fatal("expected error when staking stats lookup fails")
	}
}
