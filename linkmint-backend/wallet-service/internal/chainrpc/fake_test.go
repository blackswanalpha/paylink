package chainrpc

import (
	"context"
	"errors"
	"testing"
)

func TestFakeClientReturnsConfigured(t *testing.T) {
	f := NewFake()
	f.Accounts["0xabc"] = Account{Address: "0xabc", Balance: 5, Nonce: 2}
	f.Nonces["0xabc"] = 2
	f.Validators["0xabc"] = Validator{Address: "0xabc", StakedAmount: 9, IsActive: true}
	f.Tokens = TokenStats{TotalSupply: 100, MaxSupply: 200}
	f.Info = ChainInfo{ChainID: "paylink-test", Height: 3}
	f.Stats = StakingStats{TotalStaked: 9, MinimumStake: 1}
	f.Height = 3
	ctx := context.Background()

	if a, err := f.GetAccount(ctx, "0xabc"); err != nil || a.Balance != 5 {
		t.Fatalf("GetAccount = %+v err %v", a, err)
	}
	// Unknown address yields zeros, not an error (mirrors the chain).
	if a, err := f.GetAccount(ctx, "0xother"); err != nil || a.Balance != 0 {
		t.Fatalf("unknown GetAccount = %+v err %v", a, err)
	}
	if n, err := f.GetNonce(ctx, "0xabc"); err != nil || n != 2 {
		t.Fatalf("GetNonce = %d err %v", n, err)
	}
	if v, ok, err := f.GetValidator(ctx, "0xabc"); err != nil || !ok || v.StakedAmount != 9 {
		t.Fatalf("GetValidator = %+v ok %v err %v", v, ok, err)
	}
	if _, ok, _ := f.GetValidator(ctx, "0xnope"); ok {
		t.Fatal("expected validator not-found")
	}
	if s, err := f.StakingStats(ctx); err != nil || s.TotalStaked != 9 {
		t.Fatalf("StakingStats = %+v err %v", s, err)
	}
	if ts, err := f.TokenStats(ctx); err != nil || ts.MaxSupply != 200 {
		t.Fatalf("TokenStats = %+v err %v", ts, err)
	}
	if ci, err := f.ChainInfo(ctx); err != nil || ci.ChainID != "paylink-test" {
		t.Fatalf("ChainInfo = %+v err %v", ci, err)
	}
	if h, err := f.ChainHeight(ctx); err != nil || h != 3 {
		t.Fatalf("ChainHeight = %d err %v", h, err)
	}
	if err := f.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestFakeClientErrPropagates(t *testing.T) {
	f := NewFake()
	f.Err = ErrUnavailable
	ctx := context.Background()

	if _, err := f.GetAccount(ctx, "x"); !errors.Is(err, ErrUnavailable) {
		t.Error("GetAccount should propagate Err")
	}
	if _, err := f.GetNonce(ctx, "x"); !errors.Is(err, ErrUnavailable) {
		t.Error("GetNonce should propagate Err")
	}
	if _, _, err := f.GetValidator(ctx, "x"); !errors.Is(err, ErrUnavailable) {
		t.Error("GetValidator should propagate Err")
	}
	if _, err := f.StakingStats(ctx); !errors.Is(err, ErrUnavailable) {
		t.Error("StakingStats should propagate Err")
	}
	if _, err := f.TokenStats(ctx); !errors.Is(err, ErrUnavailable) {
		t.Error("TokenStats should propagate Err")
	}
	if _, err := f.ChainInfo(ctx); !errors.Is(err, ErrUnavailable) {
		t.Error("ChainInfo should propagate Err")
	}
	if _, err := f.ChainHeight(ctx); !errors.Is(err, ErrUnavailable) {
		t.Error("ChainHeight should propagate Err")
	}
	if err := f.Ping(ctx); !errors.Is(err, ErrUnavailable) {
		t.Error("Ping should propagate Err")
	}
}
