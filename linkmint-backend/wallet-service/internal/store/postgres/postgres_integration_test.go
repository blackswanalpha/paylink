//go:build integration

// Integration tests for the pgx-backed store. Run with: go test -tags=integration ./...
// Requires a Docker daemon (testcontainers spins an ephemeral postgres:16). These assert the
// projection writes, DbDedupe exactly-once semantics, keyset pagination, and the treasury aggregate
// against a real database, which the in-memory unit tests approximate.
package postgres

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/paylink/wallet-service/internal/domain"
)

const (
	addr1 = "0x" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	addr2 = "0x" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("paylink"),
		tcpostgres.WithUsername("paylink"),
		tcpostgres.WithPassword("paylink"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	s, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := s.Migrate(ctx); err != nil { // idempotent re-run
		t.Fatalf("re-migrate: %v", err)
	}
	return s
}

func at(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

func TestAccountCacheRoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if _, ok, err := s.GetAccountCache(ctx, addr1); err != nil || ok {
		t.Fatalf("empty cache = ok %v err %v", ok, err)
	}
	want := domain.Account{Addr: addr1, Balance: big.NewInt(123456789), Nonce: 9, BlockHeight: 5, FetchedAt: at(1000)}
	if err := s.UpsertAccountCache(ctx, want); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, ok, err := s.GetAccountCache(ctx, addr1)
	if err != nil || !ok || got.Balance.Cmp(big.NewInt(123456789)) != 0 || got.Nonce != 9 {
		t.Fatalf("cache = %+v ok %v err %v", got, ok, err)
	}

	// Upsert again updates in place.
	want.Balance = big.NewInt(42)
	if err := s.UpsertAccountCache(ctx, want); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, _, _ = s.GetAccountCache(ctx, addr1)
	if got.Balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("re-upsert balance = %s", got.Balance)
	}
}

func TestStakingProjectionAndDedupe(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	// Stake applies; redelivery (same tx_hash+addr) is a no-op.
	ran, err := s.RecordStaked(ctx, domain.StakedEvent{Addr: addr1, Amount: big.NewInt(100), TotalStaked: big.NewInt(100), IsActive: true, TxHash: "0xh1", BlockHeight: 1, OccurredAt: at(1000)})
	if err != nil || !ran {
		t.Fatalf("staked ran=%v err=%v", ran, err)
	}
	ran, err = s.RecordStaked(ctx, domain.StakedEvent{Addr: addr1, Amount: big.NewInt(100), TotalStaked: big.NewInt(100), IsActive: true, TxHash: "0xh1", BlockHeight: 1, OccurredAt: at(1000)})
	if err != nil || ran {
		t.Fatalf("duplicate staked ran=%v err=%v (want ran=false)", ran, err)
	}

	if _, err := s.RecordRewarded(ctx, domain.RewardedEvent{Addr: addr1, Amount: big.NewInt(10), TotalRewards: big.NewInt(10), TxHash: "0xh2", BlockHeight: 2, OccurredAt: at(2000)}); err != nil {
		t.Fatalf("rewarded: %v", err)
	}
	if _, err := s.RecordUnstakeStarted(ctx, domain.UnstakeStartedEvent{Addr: addr1, Amount: big.NewInt(40), WithdrawableAt: ptr(at(9000)), TxHash: "0xh3", BlockHeight: 3, OccurredAt: at(3000)}); err != nil {
		t.Fatalf("unstake started: %v", err)
	}
	if _, err := s.RecordSlashed(ctx, domain.SlashedEvent{Addr: addr1, Amount: big.NewInt(5), Remaining: big.NewInt(55), TxHash: "0xh4", BlockHeight: 4, OccurredAt: at(4000)}); err != nil {
		t.Fatalf("slashed: %v", err)
	}
	if _, err := s.RecordUnstakeCompleted(ctx, domain.UnstakeCompletedEvent{Addr: addr1, Amount: big.NewInt(40), TxHash: "0xh5", BlockHeight: 5, OccurredAt: at(5000)}); err != nil {
		t.Fatalf("unstake completed: %v", err)
	}

	pos, ok, err := s.GetPosition(ctx, addr1)
	if err != nil || !ok {
		t.Fatalf("position ok=%v err=%v", ok, err)
	}
	// staked: 100 → -40 (unstake start) = 60 → slash sets remaining 55. pending: +40 → -40 = 0.
	if pos.StakedAmount.Cmp(big.NewInt(55)) != 0 || pos.PendingWithdrawal.Sign() != 0 ||
		pos.TotalRewards.Cmp(big.NewInt(10)) != 0 || pos.TotalSlashed.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("position = %+v", pos)
	}
	if pos.WithdrawableAt == nil {
		t.Fatal("withdrawable_at should be set")
	}
}

func TestTransactionPaginationKeyset(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	// Five staking events at increasing block heights → five 'stake' tx rows for addr1.
	for i := 1; i <= 5; i++ {
		if _, err := s.RecordStaked(ctx, domain.StakedEvent{
			Addr: addr1, Amount: big.NewInt(int64(i)), TotalStaked: big.NewInt(int64(i)), IsActive: true,
			TxHash: "0xh" + string(rune('0'+i)), BlockHeight: uint64(i), OccurredAt: at(int64(1000 * i)),
		}); err != nil {
			t.Fatalf("staked %d: %v", i, err)
		}
	}

	page1, next, err := s.ListTransactions(ctx, addr1, 2, "")
	if err != nil || len(page1) != 2 || next == "" {
		t.Fatalf("page1 len=%d next=%q err=%v", len(page1), next, err)
	}
	// Newest first: block 5 then 4.
	if page1[0].BlockHeight != 5 || page1[1].BlockHeight != 4 {
		t.Fatalf("page1 order = %d,%d", page1[0].BlockHeight, page1[1].BlockHeight)
	}
	page2, next2, err := s.ListTransactions(ctx, addr1, 2, next)
	if err != nil || len(page2) != 2 || page2[0].BlockHeight != 3 {
		t.Fatalf("page2 len=%d first=%d err=%v", len(page2), page2[0].BlockHeight, err)
	}
	page3, _, err := s.ListTransactions(ctx, addr1, 2, next2)
	if err != nil || len(page3) != 1 || page3[0].BlockHeight != 1 {
		t.Fatalf("page3 len=%d err=%v", len(page3), err)
	}
}

func TestTransferTwoSidedRows(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.RecordTransfer(ctx, domain.TransferEvent{From: addr1, To: addr2, Amount: big.NewInt(7), TxHash: "0xt1", BlockHeight: 1, OccurredAt: at(1000)}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	out, _, _ := s.ListTransactions(ctx, addr1, 10, "")
	if len(out) != 1 || out[0].Direction != "out" || out[0].Counterparty != addr2 {
		t.Fatalf("sender row = %+v", out)
	}
	in, _, _ := s.ListTransactions(ctx, addr2, 10, "")
	if len(in) != 1 || in[0].Direction != "in" || in[0].Counterparty != addr1 {
		t.Fatalf("receiver row = %+v", in)
	}
}

func TestTreasuryAggregateAndDedupe(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	// Two distinct fee.collected events accumulate; a redelivery of the first does not double count.
	fee := func(txh string, fee, val, treas, burn, bh int64) domain.FeeCollectedEvent {
		return domain.FeeCollectedEvent{
			TotalFee: big.NewInt(fee), ValidatorShare: big.NewInt(val), TreasuryShare: big.NewInt(treas),
			BurnAmount: big.NewInt(burn), TxHash: txh, BlockHeight: uint64(bh), OccurredAt: at(1000 + bh),
		}
	}
	if _, err := s.RecordFeeCollected(ctx, fee("0xf1", 100, 70, 20, 10, 1)); err != nil {
		t.Fatalf("fee1: %v", err)
	}
	if ran, _ := s.RecordFeeCollected(ctx, fee("0xf1", 100, 70, 20, 10, 1)); ran {
		t.Fatal("duplicate fee.collected should not re-apply")
	}
	if _, err := s.RecordFeeCollected(ctx, fee("0xf2", 50, 35, 10, 5, 2)); err != nil {
		t.Fatalf("fee2: %v", err)
	}
	// token.burned sets the authoritative cumulative burn.
	if _, err := s.RecordTokenBurned(ctx, domain.TokenBurnedEvent{Amount: big.NewInt(15), TotalBurned: big.NewInt(15), TxHash: "0xb1", BlockHeight: 2, OccurredAt: at(2000)}); err != nil {
		t.Fatalf("burn: %v", err)
	}

	stats, err := s.GetTreasuryStats(ctx)
	if err != nil {
		t.Fatalf("treasury: %v", err)
	}
	if stats.FeesCollected.Cmp(big.NewInt(150)) != 0 || stats.ValidatorRewards.Cmp(big.NewInt(105)) != 0 ||
		stats.TreasuryAmount.Cmp(big.NewInt(30)) != 0 || stats.TotalBurned.Cmp(big.NewInt(15)) != 0 {
		t.Fatalf("aggregate = %+v", stats)
	}
	if stats.ChainHeight != 2 {
		t.Fatalf("chain_height = %d", stats.ChainHeight)
	}
}

func TestRewardsHistorySources(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.RecordRewarded(ctx, domain.RewardedEvent{Addr: addr1, Amount: big.NewInt(10), TotalRewards: big.NewInt(10), TxHash: "0xr1", BlockHeight: 1, OccurredAt: at(1000)}); err != nil {
		t.Fatalf("rewarded: %v", err)
	}
	if _, err := s.RecordFeeDistributed(ctx, domain.FeeDistributedEvent{Validator: addr1, Amount: big.NewInt(5), TxHash: "0xd1", BlockHeight: 2, OccurredAt: at(2000)}); err != nil {
		t.Fatalf("fee distributed: %v", err)
	}
	rewards, _, err := s.ListRewards(ctx, addr1, 10, "")
	if err != nil || len(rewards) != 2 {
		t.Fatalf("rewards = %d err %v", len(rewards), err)
	}
	// Newest first: fee_share (block 2) then validator_reward (block 1).
	if rewards[0].Source != domain.SourceFeeShare || rewards[1].Source != domain.SourceValidatorReward {
		t.Fatalf("reward sources = %s,%s", rewards[0].Source, rewards[1].Source)
	}
}

func ptr(t time.Time) *time.Time { return &t }
