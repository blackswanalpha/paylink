package domain_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/paylink/settlement-service/internal/domain"
)

func TestReadsNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, &fakePub{})
	if _, _, err := svc.GetSettlement(ctx, "missing", payee); err == nil {
		t.Fatal("expected not found for missing settlement")
	}
	if _, err := svc.GetPayout(ctx, "missing", payee); err == nil {
		t.Fatal("expected not found for missing payout")
	}
}

func TestIngestReingestIdempotent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	svc, _ := newSvc(t, &fakePub{}, domain.WithClock(fixedClock(now)))
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, verifiedAt(plA, 1500, day))
	svc.Schedule(ctx)
	payouts, _ := svc.ListPayouts(ctx, payee, "", 10)
	file := domain.RailFileInput{Rail: "mpesa", FileID: "f1", Lines: []domain.RailFileLine{
		{Reference: payouts[0].Reference, Amount: big.NewInt(1500), Currency: "KES"},
	}}
	if res, _ := svc.IngestRailFile(ctx, file); res.Matched != 1 {
		t.Fatalf("first ingest matched=%d, want 1", res.Matched)
	}
	// Re-ingest the same file id → no new matches (idempotent).
	res2, _ := svc.IngestRailFile(ctx, file)
	if len(res2.PaidPayouts) != 0 {
		t.Fatalf("re-ingest paid=%d, want 0", len(res2.PaidPayouts))
	}
}

func TestClawbackZeroOccurredUsesClock(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	svc, _ := newSvc(t, &fakePub{}, domain.WithClock(fixedClock(now)))
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, verifiedAt(plA, 1500, day))
	// OccurredAt zero → service falls back to the clock (2026-06-11) for the offset period.
	res, err := svc.HandleClawback(ctx, domain.ClawbackEvent{RefundID: "rf1", PLID: plA, Amount: big.NewInt(100)})
	if err != nil || res != domain.ResultClawback {
		t.Fatalf("clawback = (%q,%v)", res, err)
	}
}

func TestPublishNilPublisherIsSafe(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, nil) // nil publisher
	day := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	if res, err := svc.HandleVerified(ctx, verifiedAt(plA, 1500, day)); err != nil || res != domain.ResultSettled {
		t.Fatalf("verified with nil publisher = (%q,%v)", res, err)
	}
}
