package domain_test

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/paylink/settlement-service/internal/domain"
	"github.com/paylink/settlement-service/internal/store/memory"
)

// fakePub records published events.
type fakePub struct {
	mu     sync.Mutex
	events []pubEvent
}

type pubEvent struct {
	name, key string
	payload   any
}

func (f *fakePub) Publish(_ context.Context, name, key string, payload any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, pubEvent{name, key, payload})
	return nil
}

func (f *fakePub) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, e := range f.events {
		if e.name == name {
			n++
		}
	}
	return n
}

const (
	payee = "0x00000000000000000000000000000000000000aa"
	plA   = "PLK_A"
	plB   = "PLK_B"
)

// fixedClock returns a clock pinned to t.
func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// newSvc builds a service over a fresh memory store with UTC tz and the given options applied last.
func newSvc(t *testing.T, pub domain.Publisher, opts ...domain.Option) (*domain.Service, *memory.Store) {
	t.Helper()
	st := memory.New()
	base := []domain.Option{
		domain.WithCurrency("KES"),
		domain.WithTimezone("UTC"),
		domain.WithMinPayout(func(string) *big.Int { return big.NewInt(0) }),
		domain.WithDefaultRail("mpesa"),
	}
	return domain.NewService(st, pub, nil, append(base, opts...)...), st
}

func verifiedAt(pl string, amount uint64, ts time.Time) domain.VerifiedEvent {
	return domain.VerifiedEvent{PLID: pl, Payee: payee, Amount: new(big.Int).SetUint64(amount), TxHash: "0xtx", OccurredAt: ts}
}

func TestHandleVerifiedOpensSettlementAndAggregates(t *testing.T) {
	ctx := context.Background()
	pub := &fakePub{}
	svc, _ := newSvc(t, pub)

	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	res, err := svc.HandleVerified(ctx, verifiedAt(plA, 1500, day))
	if err != nil || res != domain.ResultSettled {
		t.Fatalf("verified A = (%q,%v), want settled", res, err)
	}
	// Second PayLink, same payee + day → same settlement, no new batch_created.
	if res, err := svc.HandleVerified(ctx, verifiedAt(plB, 500, day)); err != nil || res != domain.ResultSettled {
		t.Fatalf("verified B = (%q,%v)", res, err)
	}

	if got := pub.count(domain.EventSettlementBatchCreated); got != 1 {
		t.Fatalf("batch_created published %d times, want 1", got)
	}
	list, _ := svc.ListSettlements(ctx, payee, "", 10)
	if len(list) != 1 {
		t.Fatalf("settlements = %d, want 1", len(list))
	}
	st := list[0]
	if st.Gross.Cmp(big.NewInt(2000)) != 0 || st.Net.Cmp(big.NewInt(2000)) != 0 {
		t.Fatalf("gross=%s net=%s, want 2000/2000", st.Gross, st.Net)
	}
	if st.SettlementDate != "2026-06-10" {
		t.Fatalf("settlement_date=%s, want 2026-06-10", st.SettlementDate)
	}
}

func TestHandleVerifiedDuplicateIsNoop(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, &fakePub{})
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, verifiedAt(plA, 1500, day))
	res, err := svc.HandleVerified(ctx, verifiedAt(plA, 1500, day))
	if err != nil || res != domain.ResultDuplicate {
		t.Fatalf("duplicate = (%q,%v), want duplicate", res, err)
	}
	list, _ := svc.ListSettlements(ctx, payee, "", 10)
	if list[0].Gross.Cmp(big.NewInt(1500)) != 0 {
		t.Fatalf("gross=%s, want 1500 (no double count)", list[0].Gross)
	}
}

func TestHandleVerifiedIncompleteIgnored(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, &fakePub{})
	res, _ := svc.HandleVerified(ctx, domain.VerifiedEvent{PLID: "", Payee: payee, Amount: big.NewInt(10)})
	if res != domain.ResultIgnored {
		t.Fatalf("empty pl_id = %q, want ignored", res)
	}
	res, _ = svc.HandleVerified(ctx, domain.VerifiedEvent{PLID: plA, Payee: "", Amount: big.NewInt(10)})
	if res != domain.ResultIgnored {
		t.Fatalf("empty payee = %q, want ignored", res)
	}
	res, _ = svc.HandleVerified(ctx, domain.VerifiedEvent{PLID: plA, Payee: payee, Amount: big.NewInt(0)})
	if res != domain.ResultIgnored {
		t.Fatalf("zero amount = %q, want ignored", res)
	}
}

func TestPlatformFeeAndChainFeeNet(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, &fakePub{}, domain.WithPlatformFeeBps(100)) // 1%
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	if _, err := svc.HandleVerified(ctx, verifiedAt(plA, 1500, day)); err != nil {
		t.Fatal(err)
	}
	// platform fee = floor(1500*100/10000) = 15 → net after verified = 1485
	res, err := svc.HandleFee(ctx, domain.FeeEvent{PLID: plA, ChainFee: big.NewInt(7)})
	if err != nil || res != domain.ResultFee {
		t.Fatalf("fee = (%q,%v), want fee", res, err)
	}
	list, _ := svc.ListSettlements(ctx, payee, "", 10)
	st := list[0]
	if st.PlatformFee.Cmp(big.NewInt(15)) != 0 || st.ChainFee.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("platform=%s chain=%s, want 15/7", st.PlatformFee, st.ChainFee)
	}
	if st.Net.Cmp(big.NewInt(1478)) != 0 { // 1500 - 15 - 7
		t.Fatalf("net=%s, want 1478", st.Net)
	}
}

func TestHandleFeeNoItemIgnored(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, &fakePub{})
	res, err := svc.HandleFee(ctx, domain.FeeEvent{PLID: "PLK_unknown", ChainFee: big.NewInt(5)})
	if err != nil || res != domain.ResultIgnored {
		t.Fatalf("fee with no item = (%q,%v), want ignored", res, err)
	}
}

func TestHandleFeeDuplicate(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, &fakePub{})
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, verifiedAt(plA, 1500, day))
	_, _ = svc.HandleFee(ctx, domain.FeeEvent{PLID: plA, ChainFee: big.NewInt(7)})
	res, _ := svc.HandleFee(ctx, domain.FeeEvent{PLID: plA, ChainFee: big.NewInt(7)})
	if res != domain.ResultDuplicate {
		t.Fatalf("duplicate fee = %q, want duplicate", res)
	}
	list, _ := svc.ListSettlements(ctx, payee, "", 10)
	if list[0].ChainFee.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("chain_fee=%s, want 7 (no double count)", list[0].ChainFee)
	}
}

func TestClawbackOffsetsNet(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, &fakePub{})
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, verifiedAt(plA, 1500, day))

	res, err := svc.HandleClawback(ctx, domain.ClawbackEvent{RefundID: "rf1", PLID: plA, Amount: big.NewInt(400), OccurredAt: day})
	if err != nil || res != domain.ResultClawback {
		t.Fatalf("clawback = (%q,%v), want clawback", res, err)
	}
	list, _ := svc.ListSettlements(ctx, payee, "", 10)
	if list[0].Net.Cmp(big.NewInt(1100)) != 0 { // 1500 - 400
		t.Fatalf("net=%s, want 1100", list[0].Net)
	}
	// Duplicate refund id → no double offset.
	if res, _ := svc.HandleClawback(ctx, domain.ClawbackEvent{RefundID: "rf1", PLID: plA, Amount: big.NewInt(400), OccurredAt: day}); res != domain.ResultDuplicate {
		t.Fatalf("duplicate clawback = %q, want duplicate", res)
	}
}

func TestClawbackNoItemIgnored(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, &fakePub{})
	res, _ := svc.HandleClawback(ctx, domain.ClawbackEvent{RefundID: "rf1", PLID: "PLK_unknown", Amount: big.NewInt(400)})
	if res != domain.ResultIgnored {
		t.Fatalf("clawback no item = %q, want ignored", res)
	}
}

func TestMerchantAndBankProjections(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, &fakePub{})
	if res, _ := svc.HandleMerchantOnboarded(ctx, domain.MerchantOnboardedEvent{MerchantID: "m1", Status: "ACTIVE"}); res != domain.ResultMerchant {
		t.Fatalf("merchant = %q, want merchant", res)
	}
	if res, _ := svc.HandleMerchantOnboarded(ctx, domain.MerchantOnboardedEvent{MerchantID: "m1"}); res != domain.ResultDuplicate {
		t.Fatalf("merchant dup = %q, want duplicate", res)
	}
	if res, _ := svc.HandleBankAccountVerified(ctx, domain.BankAccountVerifiedEvent{BankAccountID: "b1", MerchantID: "m1", Status: "VERIFIED"}); res != domain.ResultBank {
		t.Fatalf("bank = %q, want bank", res)
	}
	if res, _ := svc.HandleMerchantOnboarded(ctx, domain.MerchantOnboardedEvent{MerchantID: ""}); res != domain.ResultIgnored {
		t.Fatalf("empty merchant = %q, want ignored", res)
	}
}

func TestScheduleClosesAndInstructsPayout(t *testing.T) {
	ctx := context.Background()
	pub := &fakePub{}
	// Clock is well after the verified day's T+1 cutoff so the settlement closes.
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	svc, _ := newSvc(t, pub, domain.WithClock(fixedClock(now)),
		domain.WithMinPayout(func(string) *big.Int { return big.NewInt(1000) }))

	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, verifiedAt(plA, 1500, day)) // net 1500 >= min 1000

	svc.Schedule(ctx)

	payouts, _ := svc.ListPayouts(ctx, payee, "", 10)
	if len(payouts) != 1 {
		t.Fatalf("payouts = %d, want 1", len(payouts))
	}
	p := payouts[0]
	if p.Status != domain.PayoutInstructed || p.Amount.Cmp(big.NewInt(1500)) != 0 {
		t.Fatalf("payout status=%s amount=%s, want INSTRUCTED/1500", p.Status, p.Amount)
	}
	if pub.count(domain.EventPayoutScheduled) != 1 || pub.count(domain.EventPayoutInstructed) != 1 {
		t.Fatalf("expected one payout.scheduled + one payout.instructed")
	}
	// A second schedule pass must not create a duplicate payout.
	svc.Schedule(ctx)
	if payouts, _ := svc.ListPayouts(ctx, payee, "", 10); len(payouts) != 1 {
		t.Fatalf("payouts after re-schedule = %d, want 1", len(payouts))
	}
}

func TestScheduleBelowMinimumCarriesForward(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	svc, _ := newSvc(t, &fakePub{}, domain.WithClock(fixedClock(now)),
		domain.WithMinPayout(func(string) *big.Int { return big.NewInt(1000) }))
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, verifiedAt(plA, 500, day)) // below min 1000

	svc.Schedule(ctx)
	if payouts, _ := svc.ListPayouts(ctx, payee, "", 10); len(payouts) != 0 {
		t.Fatalf("payouts = %d, want 0 (below minimum)", len(payouts))
	}
}

func TestCreatePayoutManual(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	svc, _ := newSvc(t, &fakePub{}, domain.WithClock(fixedClock(now)))
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, verifiedAt(plA, 1500, day))

	// Not closed yet → invalid state.
	list, _ := svc.ListSettlements(ctx, payee, "", 10)
	if _, err := svc.CreatePayout(ctx, list[0].ID, payee); err == nil {
		t.Fatal("expected error creating payout on OPEN settlement")
	}
	// Close it, then pay out.
	if _, err := svc.ListSettlements(ctx, payee, "", 10); err != nil {
		t.Fatal(err)
	}
	svc.Schedule(ctx) // closes + instructs (min=0 default) → payout exists already
	// Wrong caller → not found.
	if _, err := svc.CreatePayout(ctx, list[0].ID, "0xother"); err == nil {
		t.Fatal("expected not-found for wrong caller")
	}
}

func TestIngestMatchesPayoutAndCompletes(t *testing.T) {
	ctx := context.Background()
	pub := &fakePub{}
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	svc, _ := newSvc(t, pub, domain.WithClock(fixedClock(now)))
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, verifiedAt(plA, 1500, day))
	svc.Schedule(ctx)
	payouts, _ := svc.ListPayouts(ctx, payee, "", 10)
	ref := payouts[0].Reference

	res, err := svc.IngestRailFile(ctx, domain.RailFileInput{
		Rail: "mpesa", FileID: "file1",
		Lines: []domain.RailFileLine{
			{Reference: ref, Amount: big.NewInt(1500), Currency: "KES"},
			{Reference: "PO-unknown", Amount: big.NewInt(999), Currency: "KES"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched != 1 || res.Unmatched != 1 {
		t.Fatalf("matched=%d unmatched=%d, want 1/1", res.Matched, res.Unmatched)
	}
	if pub.count(domain.EventPayoutCompleted) != 1 || pub.count(domain.EventSettlementCompleted) != 1 {
		t.Fatal("expected payout.completed + settlement.completed")
	}
	p, _ := svc.GetPayout(ctx, payouts[0].ID, payee)
	if p.Status != domain.PayoutPaid {
		t.Fatalf("payout status=%s, want PAID", p.Status)
	}
}
