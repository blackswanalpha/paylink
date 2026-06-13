//go:build integration

// Integration tests for the pgx-backed store. Run with: go test -tags=integration ./...
// Requires a Docker daemon (testcontainers spins an ephemeral postgres:16). These assert the
// double-entry ledger balance (A.6) end-to-end, which the in-memory unit tests cannot.
package postgres

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	ledger "github.com/paylink/ledger-go"

	"github.com/paylink/settlement-service/internal/domain"
)

const payee = "0x00000000000000000000000000000000000000aa"

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

func cutoff() time.Time { return time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC) }

func verified(pl string, gross, platformFee int64) domain.VerifiedRecord {
	return domain.VerifiedRecord{
		PLID: pl, MerchantKey: payee, Currency: "KES", SettlementDate: "2026-06-10",
		CutoffAt: cutoff(), Gross: big.NewInt(gross), PlatformFee: big.NewInt(platformFee), TxHash: "0xtx",
	}
}

func assertBalanced(t *testing.T, s *Store, ccy string) {
	t.Helper()
	ok, err := ledger.IsBalanced(context.Background(), s.pool, ccy)
	if err != nil {
		t.Fatalf("IsBalanced: %v", err)
	}
	if !ok {
		t.Fatalf("ledger is not balanced for %s (A.6 violated)", ccy)
	}
}

func TestVerifiedFeeAggregatesAndBalances(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	out, err := s.RecordVerified(ctx, verified("PLK_A", 1500, 15))
	if err != nil {
		t.Fatalf("RecordVerified: %v", err)
	}
	if !out.Applied || !out.Opened {
		t.Fatalf("outcome = %+v, want applied+opened", out)
	}
	// Duplicate verified is a no-op.
	if dup, err := s.RecordVerified(ctx, verified("PLK_A", 1500, 15)); err != nil || dup.Applied {
		t.Fatalf("duplicate verified = (%+v,%v)", dup, err)
	}

	fee, err := s.RecordFee(ctx, domain.FeeRecord{PLID: "PLK_A", ChainFee: big.NewInt(7)})
	if err != nil || !fee.Applied || !fee.Found {
		t.Fatalf("RecordFee = (%+v,%v)", fee, err)
	}
	// Duplicate fee is a no-op.
	if dup, _ := s.RecordFee(ctx, domain.FeeRecord{PLID: "PLK_A", ChainFee: big.NewInt(7)}); dup.Applied {
		t.Fatal("duplicate fee applied")
	}

	st, items, err := s.GetSettlement(ctx, out.Settlement.ID, payee)
	if err != nil {
		t.Fatalf("GetSettlement: %v", err)
	}
	if st.Gross.Cmp(big.NewInt(1500)) != 0 || st.PlatformFee.Cmp(big.NewInt(15)) != 0 ||
		st.ChainFee.Cmp(big.NewInt(7)) != 0 || st.Net.Cmp(big.NewInt(1478)) != 0 {
		t.Fatalf("settlement totals = %s/%s/%s/%s, want 1500/15/7/1478", st.Gross, st.PlatformFee, st.ChainFee, st.Net)
	}
	if len(items) != 1 {
		t.Fatalf("items=%d, want 1", len(items))
	}
	assertBalanced(t, s, "KES")
}

func TestFeeWithoutItemNotFound(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	out, err := s.RecordFee(ctx, domain.FeeRecord{PLID: "PLK_unknown", ChainFee: big.NewInt(7)})
	if err != nil || out.Found {
		t.Fatalf("fee with no item = (%+v,%v), want Found=false", out, err)
	}
}

func TestScheduleClosesAndInstructsAndIngestPays(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	out, _ := s.RecordVerified(ctx, verified("PLK_A", 1500, 0))
	_, _ = s.RecordFee(ctx, domain.FeeRecord{PLID: "PLK_A", ChainFee: big.NewInt(10)})

	// Before cutoff: nothing closes.
	if closed, _ := s.CloseDueSettlements(ctx, cutoff().Add(-time.Hour)); len(closed) != 0 {
		t.Fatalf("closed before cutoff = %d, want 0", len(closed))
	}
	// After cutoff: closes.
	closed, err := s.CloseDueSettlements(ctx, cutoff().Add(time.Hour))
	if err != nil || len(closed) != 1 {
		t.Fatalf("close = (%d,%v), want 1", len(closed), err)
	}
	payouts, err := s.SchedulePayouts(ctx, cutoff().Add(time.Hour), domain.ScheduleOpts{
		MinPayoutFor: func(string) *big.Int { return big.NewInt(0) }, DefaultRail: "mpesa",
	})
	if err != nil || len(payouts) != 1 {
		t.Fatalf("schedule = (%d,%v), want 1", len(payouts), err)
	}
	p := payouts[0]
	if p.Status != domain.PayoutInstructed || p.Amount.Cmp(big.NewInt(1490)) != 0 {
		t.Fatalf("payout = %s/%s, want INSTRUCTED/1490", p.Status, p.Amount)
	}
	// Re-schedule must not duplicate.
	if again, _ := s.SchedulePayouts(ctx, cutoff().Add(time.Hour), domain.ScheduleOpts{
		MinPayoutFor: func(string) *big.Int { return big.NewInt(0) }, DefaultRail: "mpesa",
	}); len(again) != 0 {
		t.Fatalf("re-schedule = %d, want 0", len(again))
	}

	// Ingest a matching rail file → payout PAID, settlement PAID, ledger balanced.
	res, err := s.IngestRailFile(ctx, domain.RailFileInput{
		Rail: "mpesa", FileID: "f1",
		Lines: []domain.RailFileLine{
			{Reference: p.Reference, Amount: big.NewInt(1490), Currency: "KES"},
			{Reference: "PO-unknown", Amount: big.NewInt(1), Currency: "KES"},
		},
	})
	if err != nil || res.Matched != 1 || res.Unmatched != 1 {
		t.Fatalf("ingest = (%+v,%v)", res, err)
	}
	paid, _ := s.GetPayout(ctx, p.ID, payee)
	if paid.Status != domain.PayoutPaid {
		t.Fatalf("payout status=%s, want PAID", paid.Status)
	}
	st, _, _ := s.GetSettlement(ctx, out.Settlement.ID, payee)
	if st.Status != domain.StatusPaid {
		t.Fatalf("settlement status=%s, want PAID", st.Status)
	}
	// Re-ingest same file id is idempotent.
	if again, _ := s.IngestRailFile(ctx, domain.RailFileInput{Rail: "mpesa", FileID: "f1"}); len(again.PaidPayouts) != 0 {
		t.Fatalf("re-ingest paid=%d, want 0", len(again.PaidPayouts))
	}
	assertBalanced(t, s, "KES")
}

func TestClawbackOffsetsAndBalances(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_, _ = s.RecordVerified(ctx, verified("PLK_A", 1500, 0))

	out, err := s.RecordClawback(ctx, domain.ClawbackRecord{
		RefundID: "rf1", PLID: "PLK_A", Amount: big.NewInt(400),
		SettlementDate: "2026-06-10", CutoffAt: cutoff(),
	})
	if err != nil || !out.Applied || !out.Found {
		t.Fatalf("clawback = (%+v,%v)", out, err)
	}
	// Unknown pl_id → not found.
	if u, _ := s.RecordClawback(ctx, domain.ClawbackRecord{RefundID: "rf2", PLID: "PLK_x", Amount: big.NewInt(1), SettlementDate: "2026-06-10", CutoffAt: cutoff()}); u.Found {
		t.Fatal("clawback for unknown pl_id should be Found=false")
	}
	// Duplicate refund id → no-op.
	if dup, _ := s.RecordClawback(ctx, domain.ClawbackRecord{RefundID: "rf1", PLID: "PLK_A", Amount: big.NewInt(400), SettlementDate: "2026-06-10", CutoffAt: cutoff()}); dup.Applied {
		t.Fatal("duplicate clawback applied")
	}

	list, _ := s.ListSettlements(ctx, payee, "", 10)
	if len(list) != 1 || list[0].Net.Cmp(big.NewInt(1100)) != 0 {
		t.Fatalf("net after clawback = %s, want 1100", list[0].Net)
	}
	assertBalanced(t, s, "KES")
}

func TestProjectionsUpsert(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if ok, err := s.UpsertMerchant(ctx, domain.Merchant{MerchantID: "m1", Status: "ACTIVE"}); err != nil || !ok {
		t.Fatalf("merchant upsert = (%v,%v)", ok, err)
	}
	if ok, _ := s.UpsertMerchant(ctx, domain.Merchant{MerchantID: "m1"}); ok {
		t.Fatal("duplicate merchant should be no-op")
	}
	if ok, err := s.UpsertBankAccount(ctx, domain.BankAccount{BankAccountID: "b1", MerchantID: "m1", Rail: "mpesa", Currency: "KES", Status: "VERIFIED"}); err != nil || !ok {
		t.Fatalf("bank upsert = (%v,%v)", ok, err)
	}
}

func TestCreatePayoutManualStates(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	out, _ := s.RecordVerified(ctx, verified("PLK_A", 1500, 0))
	sid := out.Settlement.ID

	// OPEN → invalid state.
	if _, err := s.CreatePayout(ctx, sid, payee, "mpesa"); err == nil {
		t.Fatal("payout on OPEN settlement should fail")
	}
	// Wrong owner → not found.
	if _, err := s.CreatePayout(ctx, sid, "0xother", "mpesa"); err != domain.ErrNotFound {
		t.Fatalf("wrong owner err=%v, want ErrNotFound", err)
	}
	// Close then pay.
	_, _ = s.CloseDueSettlements(ctx, cutoff().Add(time.Hour))
	p, err := s.CreatePayout(ctx, sid, payee, "mpesa")
	if err != nil || p.Status != domain.PayoutInstructed {
		t.Fatalf("create payout = (%+v,%v)", p, err)
	}
	// Second create → already exists.
	if _, err := s.CreatePayout(ctx, sid, payee, "mpesa"); err == nil {
		t.Fatal("second payout should fail")
	}
}
