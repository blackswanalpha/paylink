package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/lifecycle"
)

func seed(t *testing.T) (*Store, domain.Payment) {
	t.Helper()
	s := New()
	p := domain.Payment{
		ID:        "pay-1",
		PayLinkID: "0xabc",
		Rail:      "mpesa",
		Status:    lifecycle.StateAwaitingPayment,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreatePayment(context.Background(), p); err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	return s, p
}

func verifiedProject(cur lifecycle.State) (lifecycle.State, bool, error) {
	return lifecycle.Project(cur, "VERIFIED")
}

func TestCreateAndGet(t *testing.T) {
	s, p := seed(t)
	ctx := context.Background()

	got, err := s.GetPayment(ctx, p.ID)
	if err != nil || got.ID != p.ID {
		t.Fatalf("GetPayment: %v / %+v", err, got)
	}
	byPL, err := s.GetPaymentByPayLink(ctx, p.PayLinkID)
	if err != nil || byPL.ID != p.ID {
		t.Fatalf("GetPaymentByPayLink: %v / %+v", err, byPL)
	}

	if _, err := s.GetPayment(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := s.GetPaymentByPayLink(ctx, "0xmissing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCreateDuplicatePayLink(t *testing.T) {
	s, p := seed(t)
	err := s.CreatePayment(context.Background(), domain.Payment{ID: "pay-2", PayLinkID: p.PayLinkID, Status: lifecycle.StateAwaitingPayment})
	if !errors.Is(err, domain.ErrPaymentExists) {
		t.Fatalf("want ErrPaymentExists, got %v", err)
	}
}

func TestApplyChainEventIdempotent(t *testing.T) {
	s, p := seed(t)
	ctx := context.Background()
	ref := domain.ChainEventRef{PayLinkID: p.PayLinkID, Seq: 5, Kind: "paylink.verified", TxHash: "0xtx"}

	got, changed, err := s.ApplyChainEvent(ctx, ref, verifiedProject)
	if err != nil || !changed || got.Status != lifecycle.StateSettled {
		t.Fatalf("first apply: changed=%v status=%v err=%v", changed, got.Status, err)
	}
	if got.LastEventSeq != 5 {
		t.Fatalf("last_event_seq = %d, want 5", got.LastEventSeq)
	}

	// Exact duplicate event (same seq) -> no-op.
	got, changed, err = s.ApplyChainEvent(ctx, ref, verifiedProject)
	if err != nil || changed {
		t.Fatalf("duplicate apply should be no-op: changed=%v err=%v", changed, err)
	}

	// A different-seq replay of the same VERIFIED status -> FSM no-op (already settled).
	got, changed, err = s.ApplyChainEvent(ctx, domain.ChainEventRef{PayLinkID: p.PayLinkID, Seq: 6}, verifiedProject)
	if err != nil || changed || got.Status != lifecycle.StateSettled {
		t.Fatalf("settled replay should be no-op: changed=%v status=%v err=%v", changed, got.Status, err)
	}
}

func TestApplyChainEventNotFound(t *testing.T) {
	s := New()
	_, _, err := s.ApplyChainEvent(context.Background(), domain.ChainEventRef{PayLinkID: "0xnope", Seq: 1}, verifiedProject)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestApplyChainEventIllegalTransition(t *testing.T) {
	s, p := seed(t)
	ctx := context.Background()
	// settle first
	if _, _, err := s.ApplyChainEvent(ctx, domain.ChainEventRef{PayLinkID: p.PayLinkID, Seq: 1}, verifiedProject); err != nil {
		t.Fatal(err)
	}
	// then a cancel event: illegal from settled
	cancelProject := func(cur lifecycle.State) (lifecycle.State, bool, error) {
		return lifecycle.Project(cur, "CANCELLED")
	}
	_, changed, err := s.ApplyChainEvent(ctx, domain.ChainEventRef{PayLinkID: p.PayLinkID, Seq: 2}, cancelProject)
	if changed || !errors.Is(err, lifecycle.ErrIllegalTransition) {
		t.Fatalf("want illegal transition no-op: changed=%v err=%v", changed, err)
	}
}

func TestReconcile(t *testing.T) {
	s, p := seed(t)
	ctx := context.Background()
	got, changed, err := s.Reconcile(ctx, p.PayLinkID, verifiedProject)
	if err != nil || !changed || got.Status != lifecycle.StateSettled {
		t.Fatalf("reconcile: changed=%v status=%v err=%v", changed, got.Status, err)
	}
	// idempotent second reconcile
	_, changed, err = s.Reconcile(ctx, p.PayLinkID, verifiedProject)
	if err != nil || changed {
		t.Fatalf("reconcile noop: changed=%v err=%v", changed, err)
	}
	if _, _, err := s.Reconcile(ctx, "0xmissing", verifiedProject); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPing(t *testing.T) {
	if err := New().Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestSearchPayments(t *testing.T) {
	s := New()
	ctx := context.Background()
	base := time.Now().UTC()
	mk := func(id, pl string, status lifecycle.State, age time.Duration) {
		if err := s.CreatePayment(ctx, domain.Payment{
			ID: id, PayLinkID: pl, Rail: "mpesa", Status: status,
			CreatedAt: base.Add(-age), UpdatedAt: base.Add(-age),
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	mk("pay-1", "0xaaa", lifecycle.StateAwaitingPayment, 2*time.Hour)
	mk("pay-2", "0xbbb", lifecycle.StateSettled, time.Hour)
	mk("pay-3", "0xccc", lifecycle.StateAwaitingPayment, 30*time.Minute)

	if got, _ := s.SearchPayments(ctx, "pay-2", 20); len(got) != 1 || got[0].ID != "pay-2" {
		t.Fatalf("by id: %+v", got)
	}
	if got, _ := s.SearchPayments(ctx, "0xccc", 20); len(got) != 1 || got[0].ID != "pay-3" {
		t.Fatalf("by paylink id: %+v", got)
	}
	// by status, case-insensitive, most-recent-first
	awaiting := strings.ToLower(string(lifecycle.StateAwaitingPayment))
	got, _ := s.SearchPayments(ctx, awaiting, 20)
	if len(got) != 2 || got[0].ID != "pay-3" || got[1].ID != "pay-1" {
		t.Fatalf("by status order: %+v", got)
	}
	if got, _ := s.SearchPayments(ctx, awaiting, 1); len(got) != 1 || got[0].ID != "pay-3" {
		t.Fatalf("limit clamp: %+v", got)
	}
	if got, _ := s.SearchPayments(ctx, "no-such-thing", 20); len(got) != 0 {
		t.Fatalf("no match: %+v", got)
	}
}
