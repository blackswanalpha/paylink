package domain_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	lvm "github.com/paylink/paylink-chain/pkg/lvm"

	"github.com/paylink/wallet-service/internal/chainrpc"
	"github.com/paylink/wallet-service/internal/domain"
	"github.com/paylink/wallet-service/internal/store/memory"
)

const (
	addr1 = "0x" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	addr2 = "0x" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func newService(t *testing.T, chain domain.ChainReader, opts ...domain.Option) (*domain.Service, *memory.Store) {
	t.Helper()
	st := memory.New()
	svc := domain.NewService(st, chain, nil, opts...)
	return svc, st
}

func TestNormalizeAddr(t *testing.T) {
	if _, err := domain.NormalizeAddr("not-an-addr"); !errors.Is(err, domain.ErrInvalidAddress) {
		t.Fatalf("expected ErrInvalidAddress, got %v", err)
	}
	got, err := domain.NormalizeAddr("0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	if err != nil || got != addr1 {
		t.Fatalf("normalize = %q err %v", got, err)
	}
}

func TestGetWalletReadThroughAndCache(t *testing.T) {
	fake := chainrpc.NewFake()
	fake.Accounts[addr1] = chainrpc.Account{Address: addr1, Balance: 1000, Nonce: 4}

	now := time.Unix(1_000, 0).UTC()
	svc, _ := newService(t, fake,
		domain.WithBalanceCacheTTL(10*time.Second),
		domain.WithNowFunc(func() time.Time { return now }),
	)
	ctx := context.Background()

	// First read hits the chain and caches.
	acc, err := svc.GetWallet(ctx, addr1)
	if err != nil || acc.Balance.Cmp(big.NewInt(1000)) != 0 || acc.Nonce != 4 || acc.Stale {
		t.Fatalf("first GetWallet = %+v err %v", acc, err)
	}

	// Change the chain value; within TTL the cached (old) value is served.
	fake.Accounts[addr1] = chainrpc.Account{Address: addr1, Balance: 2000, Nonce: 5}
	acc, err = svc.GetWallet(ctx, addr1)
	if err != nil || acc.Balance.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("within-TTL GetWallet should serve cache, got %+v err %v", acc, err)
	}

	// Past the TTL it refreshes from the chain.
	now = now.Add(20 * time.Second)
	acc, err = svc.GetWallet(ctx, addr1)
	if err != nil || acc.Balance.Cmp(big.NewInt(2000)) != 0 {
		t.Fatalf("post-TTL GetWallet should refresh, got %+v err %v", acc, err)
	}
}

func TestGetWalletChainDownServesStaleOrFails(t *testing.T) {
	fake := chainrpc.NewFake()
	fake.Accounts[addr1] = chainrpc.Account{Address: addr1, Balance: 700}
	now := time.Unix(2_000, 0).UTC()
	svc, _ := newService(t, fake,
		domain.WithBalanceCacheTTL(1*time.Second),
		domain.WithNowFunc(func() time.Time { return now }),
	)
	ctx := context.Background()

	// Warm the cache.
	if _, err := svc.GetWallet(ctx, addr1); err != nil {
		t.Fatalf("warm: %v", err)
	}

	// Chain down + cache present + past TTL → serve stale.
	fake.Err = chainrpc.ErrUnavailable
	now = now.Add(5 * time.Second)
	acc, err := svc.GetWallet(ctx, addr1)
	if err != nil || !acc.Stale || acc.Balance.Cmp(big.NewInt(700)) != 0 {
		t.Fatalf("stale serve = %+v err %v", acc, err)
	}

	// Chain down + no cache → ErrChainUnavailable.
	acc, err = svc.GetWallet(ctx, addr2)
	if !errors.Is(err, domain.ErrChainUnavailable) {
		t.Fatalf("expected ErrChainUnavailable, got %+v err %v", acc, err)
	}
}

func TestGetWalletInvalidAddr(t *testing.T) {
	svc, _ := newService(t, chainrpc.NewFake())
	if _, err := svc.GetWallet(context.Background(), "0xZZ"); !errors.Is(err, domain.ErrInvalidAddress) {
		t.Fatalf("expected ErrInvalidAddress, got %v", err)
	}
}

func TestBuildIntentUnsigned(t *testing.T) {
	fake := chainrpc.NewFake()
	fake.Nonces[addr1] = 9
	fake.Info = chainrpc.ChainInfo{ChainID: "paylink-live"}
	svc, _ := newService(t, fake, domain.WithChainID("paylink-fallback"))
	ctx := context.Background()

	for _, action := range []string{"stake", "unstake"} {
		intent, err := svc.BuildIntent(ctx, domain.IntentRequest{Addr: addr1, Action: action, Amount: big.NewInt(50)})
		if err != nil {
			t.Fatalf("BuildIntent(%s): %v", action, err)
		}
		if intent.Nonce != 9 {
			t.Errorf("%s nonce = %d", action, intent.Nonce)
		}
		if intent.ChainID != "paylink-live" {
			t.Errorf("%s chainID = %q (want live)", action, intent.ChainID)
		}
		if intent.FeeEstimate.Amount.Sign() != 0 || intent.FeeEstimate.Currency != "PLN" {
			t.Errorf("%s fee = %+v", action, intent.FeeEstimate)
		}
		// A.1: the tx must be unsigned — no signature, pubkey, or hash.
		if len(intent.Tx.Signature) != 0 || len(intent.Tx.PubKey) != 0 || intent.Tx.Hash != (lvm.Hash{}) {
			t.Errorf("%s tx must be unsigned: %+v", action, intent.Tx)
		}
		// signable_bytes must equal Tx.SignableBytes().
		if string(intent.SignableBytes) != string(intent.Tx.SignableBytes()) {
			t.Errorf("%s signable mismatch", action)
		}
		raw, err := intent.UnsignedJSON()
		if err != nil {
			t.Fatalf("UnsignedJSON: %v", err)
		}
		var probe struct {
			Type    int             `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			t.Fatalf("decode unsigned: %v", err)
		}
		wantType := 6
		if action == "unstake" {
			wantType = 7
		}
		if probe.Type != wantType {
			t.Errorf("%s tx type = %d want %d", action, probe.Type, wantType)
		}
	}
}

func TestBuildIntentValidation(t *testing.T) {
	fake := chainrpc.NewFake()
	fake.Nonces[addr1] = 1
	svc, _ := newService(t, fake)
	ctx := context.Background()

	cases := []struct {
		name string
		req  domain.IntentRequest
		want error
	}{
		{"bad addr", domain.IntentRequest{Addr: "nope", Action: "stake", Amount: big.NewInt(1)}, domain.ErrInvalidAddress},
		{"zero amount", domain.IntentRequest{Addr: addr1, Action: "stake", Amount: big.NewInt(0)}, domain.ErrInvalidAmount},
		{"nil amount", domain.IntentRequest{Addr: addr1, Action: "stake", Amount: nil}, domain.ErrInvalidAmount},
		{"bad action", domain.IntentRequest{Addr: addr1, Action: "transfer", Amount: big.NewInt(1)}, domain.ErrInvalidAction},
	}
	for _, c := range cases {
		if _, err := svc.BuildIntent(ctx, c.req); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v want %v", c.name, err, c.want)
		}
	}

	// Overflow amount (> uint64).
	huge := new(big.Int).Lsh(big.NewInt(1), 65)
	if _, err := svc.BuildIntent(ctx, domain.IntentRequest{Addr: addr1, Action: "stake", Amount: huge}); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("overflow: got %v", err)
	}
}

func TestBuildIntentChainDown(t *testing.T) {
	fake := chainrpc.NewFake()
	fake.Err = chainrpc.ErrUnavailable
	svc, _ := newService(t, fake)
	if _, err := svc.BuildIntent(context.Background(), domain.IntentRequest{Addr: addr1, Action: "stake", Amount: big.NewInt(1)}); !errors.Is(err, domain.ErrChainUnavailable) {
		t.Fatalf("expected ErrChainUnavailable, got %v", err)
	}
}

func TestGetPositionNotFound(t *testing.T) {
	svc, _ := newService(t, chainrpc.NewFake())
	if _, err := svc.GetPosition(context.Background(), addr1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetTreasuryStatsEnrichesSupply(t *testing.T) {
	fake := chainrpc.NewFake()
	fake.Tokens = chainrpc.TokenStats{TotalSupply: 12345, MaxSupply: 99999}
	svc, _ := newService(t, fake)
	stats, err := svc.GetTreasuryStats(context.Background())
	if err != nil {
		t.Fatalf("GetTreasuryStats: %v", err)
	}
	if stats.TotalSupply.Cmp(big.NewInt(12345)) != 0 || stats.MaxSupply.Cmp(big.NewInt(99999)) != 0 {
		t.Fatalf("supply not enriched: %+v", stats)
	}
}

// TestProjectionEndToEnd drives the consumer-facing Handle* methods through the memory store and
// reads the result back via the read methods — exercising the projection math + dedupe.
func TestProjectionEndToEnd(t *testing.T) {
	svc, _ := newService(t, chainrpc.NewFake())
	ctx := context.Background()
	at := time.Unix(5_000, 0).UTC()

	// Stake → position active, staked=100; a stake tx row.
	if r, err := svc.HandleStaked(ctx, domain.StakedEvent{Addr: addr1, Amount: big.NewInt(100), TotalStaked: big.NewInt(100), IsActive: true, TxHash: "0xh1", BlockHeight: 1, OccurredAt: at}); err != nil || r != domain.ResultProcessed {
		t.Fatalf("staked = %s err %v", r, err)
	}
	// Duplicate redelivery is suppressed.
	if r, _ := svc.HandleStaked(ctx, domain.StakedEvent{Addr: addr1, Amount: big.NewInt(100), TotalStaked: big.NewInt(100), IsActive: true, TxHash: "0xh1", BlockHeight: 1, OccurredAt: at}); r != domain.ResultDuplicate {
		t.Fatalf("expected duplicate, got %s", r)
	}

	// Rewarded → total_rewards=10; reward history + reward tx.
	if _, err := svc.HandleRewarded(ctx, domain.RewardedEvent{Addr: addr1, Amount: big.NewInt(10), TotalRewards: big.NewInt(10), TxHash: "0xh2", BlockHeight: 2, OccurredAt: at}); err != nil {
		t.Fatalf("rewarded: %v", err)
	}
	// Unstake started → pending=40, staked=60.
	if _, err := svc.HandleUnstakeStarted(ctx, domain.UnstakeStartedEvent{Addr: addr1, Amount: big.NewInt(40), TxHash: "0xh3", BlockHeight: 3, OccurredAt: at}); err != nil {
		t.Fatalf("unstake started: %v", err)
	}

	pos, err := svc.GetPosition(ctx, addr1)
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if pos.StakedAmount.Cmp(big.NewInt(60)) != 0 || pos.PendingWithdrawal.Cmp(big.NewInt(40)) != 0 || pos.TotalRewards.Cmp(big.NewInt(10)) != 0 || !pos.IsActive {
		t.Fatalf("position = %+v", pos)
	}

	txs, next, err := svc.ListTransactions(ctx, addr1, 1, "")
	if err != nil || len(txs) != 1 {
		t.Fatalf("list txs page1 = %d err %v", len(txs), err)
	}
	if next == "" {
		t.Fatal("expected a next cursor with more rows")
	}
	// Newest-first: the unstake_start (block 3) comes first.
	if txs[0].Kind != domain.KindUnstakeStart {
		t.Fatalf("first tx kind = %s", txs[0].Kind)
	}
	// Page through the rest with the cursor.
	txs2, _, err := svc.ListTransactions(ctx, addr1, 10, next)
	if err != nil || len(txs2) != 2 {
		t.Fatalf("list txs page2 = %d err %v", len(txs2), err)
	}

	rewards, _, err := svc.ListRewards(ctx, addr1, 10, "")
	if err != nil || len(rewards) != 1 || rewards[0].Source != domain.SourceValidatorReward {
		t.Fatalf("rewards = %+v err %v", rewards, err)
	}

	// Fee collected + token burned → treasury aggregate.
	if _, err := svc.HandleFeeCollected(ctx, domain.FeeCollectedEvent{TotalFee: big.NewInt(100), ValidatorShare: big.NewInt(70), TreasuryShare: big.NewInt(20), BurnAmount: big.NewInt(10), TxHash: "0xf1", BlockHeight: 4, OccurredAt: at}); err != nil {
		t.Fatalf("fee collected: %v", err)
	}
	if _, err := svc.HandleTokenBurned(ctx, domain.TokenBurnedEvent{Amount: big.NewInt(10), TotalBurned: big.NewInt(10), TxHash: "0xb1", BlockHeight: 4, OccurredAt: at}); err != nil {
		t.Fatalf("token burned: %v", err)
	}
	if _, err := svc.HandleFeeDistributed(ctx, domain.FeeDistributedEvent{Validator: addr1, Amount: big.NewInt(70), TxHash: "0xd1", BlockHeight: 4, OccurredAt: at}); err != nil {
		t.Fatalf("fee distributed: %v", err)
	}

	stats, err := svc.GetTreasuryStats(ctx)
	if err != nil {
		t.Fatalf("treasury: %v", err)
	}
	if stats.FeesCollected.Cmp(big.NewInt(100)) != 0 || stats.ValidatorRewards.Cmp(big.NewInt(70)) != 0 ||
		stats.TreasuryAmount.Cmp(big.NewInt(20)) != 0 || stats.TotalBurned.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("treasury aggregate = %+v", stats)
	}

	// fee_share reward now present for the validator.
	rewards, _, _ = svc.ListRewards(ctx, addr1, 10, "")
	var sawFeeShare bool
	for _, r := range rewards {
		if r.Source == domain.SourceFeeShare {
			sawFeeShare = true
		}
	}
	if !sawFeeShare {
		t.Fatal("expected a fee_share reward row")
	}

	// Transfer → out-row on sender, in-row on receiver.
	if _, err := svc.HandleTransfer(ctx, domain.TransferEvent{From: addr1, To: addr2, Amount: big.NewInt(5), TxHash: "0xt1", BlockHeight: 5, OccurredAt: at}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	recv, _, _ := svc.ListTransactions(ctx, addr2, 10, "")
	if len(recv) != 1 || recv[0].Direction != "in" || recv[0].Counterparty != addr1 {
		t.Fatalf("receiver tx = %+v", recv)
	}

	// Slash + unstake complete exercise the remaining projections.
	if _, err := svc.HandleSlashed(ctx, domain.SlashedEvent{Addr: addr1, Amount: big.NewInt(5), Remaining: big.NewInt(55), TxHash: "0xs1", BlockHeight: 6, OccurredAt: at}); err != nil {
		t.Fatalf("slashed: %v", err)
	}
	if _, err := svc.HandleUnstakeCompleted(ctx, domain.UnstakeCompletedEvent{Addr: addr1, Amount: big.NewInt(40), TxHash: "0xu1", BlockHeight: 7, OccurredAt: at}); err != nil {
		t.Fatalf("unstake completed: %v", err)
	}
	pos, _ = svc.GetPosition(ctx, addr1)
	if pos.StakedAmount.Cmp(big.NewInt(55)) != 0 || pos.PendingWithdrawal.Sign() != 0 || pos.TotalSlashed.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("post slash/complete position = %+v", pos)
	}
}
