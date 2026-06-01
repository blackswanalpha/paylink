//go:build integration

// Integration tests for the pgx-backed store. Run with: go test -tags=integration ./...
// Requires a Docker daemon (testcontainers spins an ephemeral postgres:16).
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/lifecycle"
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
	// idempotent re-run
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	return s
}

func awaiting(id, pl string) domain.Payment {
	return domain.Payment{
		ID: id, PayLinkID: pl, Rail: "mpesa", Status: lifecycle.StateAwaitingPayment,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
}

func verifiedProject(cur lifecycle.State) (lifecycle.State, bool, error) {
	return lifecycle.Project(cur, "VERIFIED")
}

func TestPostgresCRUDAndDuplicate(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	p := awaiting("11111111-1111-1111-1111-111111111111", "0xabc")
	if err := s.CreatePayment(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePayment(ctx, awaiting("22222222-2222-2222-2222-222222222222", "0xabc")); !errors.Is(err, domain.ErrPaymentExists) {
		t.Fatalf("want ErrPaymentExists, got %v", err)
	}
	got, err := s.GetPayment(ctx, p.ID)
	if err != nil || got.PayLinkID != "0xabc" {
		t.Fatalf("GetPayment: %v / %+v", err, got)
	}
	byPL, err := s.GetPaymentByPayLink(ctx, "0xabc")
	if err != nil || byPL.ID != p.ID {
		t.Fatalf("GetPaymentByPayLink: %v / %+v", err, byPL)
	}
	if _, err := s.GetPayment(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPostgresApplyChainEventIdempotent(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	p := awaiting("33333333-3333-3333-3333-333333333333", "0xdef")
	if err := s.CreatePayment(ctx, p); err != nil {
		t.Fatal(err)
	}
	ref := domain.ChainEventRef{PayLinkID: "0xdef", Seq: 9, Kind: "paylink.verified", TxHash: "0xtx"}

	got, changed, err := s.ApplyChainEvent(ctx, ref, verifiedProject)
	if err != nil || !changed || got.Status != lifecycle.StateSettled || got.LastEventSeq != 9 {
		t.Fatalf("first apply: changed=%v status=%v seq=%d err=%v", changed, got.Status, got.LastEventSeq, err)
	}
	// duplicate (same seq) -> no double advance
	_, changed, err = s.ApplyChainEvent(ctx, ref, verifiedProject)
	if err != nil || changed {
		t.Fatalf("duplicate apply must be no-op: changed=%v err=%v", changed, err)
	}
	// different-seq replay of same status -> FSM no-op
	_, changed, err = s.ApplyChainEvent(ctx, domain.ChainEventRef{PayLinkID: "0xdef", Seq: 10}, verifiedProject)
	if err != nil || changed {
		t.Fatalf("settled replay must be no-op: changed=%v err=%v", changed, err)
	}
	// event for unknown paylink
	if _, _, err := s.ApplyChainEvent(ctx, domain.ChainEventRef{PayLinkID: "0xmissing", Seq: 1}, verifiedProject); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPostgresApplyChainEventIllegal(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.CreatePayment(ctx, awaiting("44444444-4444-4444-4444-444444444444", "0xghi")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ApplyChainEvent(ctx, domain.ChainEventRef{PayLinkID: "0xghi", Seq: 1}, verifiedProject); err != nil {
		t.Fatal(err)
	}
	cancel := func(cur lifecycle.State) (lifecycle.State, bool, error) { return lifecycle.Project(cur, "CANCELLED") }
	_, changed, err := s.ApplyChainEvent(ctx, domain.ChainEventRef{PayLinkID: "0xghi", Seq: 2}, cancel)
	if changed || !errors.Is(err, lifecycle.ErrIllegalTransition) {
		t.Fatalf("want illegal transition: changed=%v err=%v", changed, err)
	}
}

func TestPostgresReconcile(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.CreatePayment(ctx, awaiting("55555555-5555-5555-5555-555555555555", "0xjkl")); err != nil {
		t.Fatal(err)
	}
	got, changed, err := s.Reconcile(ctx, "0xjkl", verifiedProject)
	if err != nil || !changed || got.Status != lifecycle.StateSettled {
		t.Fatalf("reconcile: changed=%v status=%v err=%v", changed, got.Status, err)
	}
	_, changed, err = s.Reconcile(ctx, "0xjkl", verifiedProject)
	if err != nil || changed {
		t.Fatalf("reconcile noop: changed=%v err=%v", changed, err)
	}
	if _, _, err := s.Reconcile(ctx, "0xmissing", verifiedProject); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPostgresSearchPayments(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a := awaiting("66666666-6666-6666-6666-666666666666", "0xsearch1")
	b := awaiting("77777777-7777-7777-7777-777777777777", "0xsearch2")
	for _, p := range []domain.Payment{a, b} {
		if err := s.CreatePayment(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	// exact payment id
	if got, err := s.SearchPayments(ctx, a.ID, 20); err != nil || len(got) != 1 || got[0].ID != a.ID {
		t.Fatalf("by id: %+v err=%v", got, err)
	}
	// exact paylink id
	if got, err := s.SearchPayments(ctx, "0xsearch2", 20); err != nil || len(got) != 1 || got[0].ID != b.ID {
		t.Fatalf("by paylink: %+v err=%v", got, err)
	}
	// by status (case-insensitive) returns both
	if got, err := s.SearchPayments(ctx, "awaiting_payment", 20); err != nil || len(got) != 2 {
		t.Fatalf("by status: %+v err=%v", got, err)
	}
	// limit clamps
	if got, err := s.SearchPayments(ctx, "AWAITING_PAYMENT", 1); err != nil || len(got) != 1 {
		t.Fatalf("limit: %+v err=%v", got, err)
	}
	// no match (and a non-uuid q must not error via id::text cast)
	if got, err := s.SearchPayments(ctx, "not-a-uuid", 20); err != nil || len(got) != 0 {
		t.Fatalf("no match: %+v err=%v", got, err)
	}
}

func TestPostgresPing(t *testing.T) {
	if err := newStore(t).Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
