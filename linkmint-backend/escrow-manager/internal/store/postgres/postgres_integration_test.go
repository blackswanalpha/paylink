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

	"github.com/paylink/escrow-manager/internal/domain"
	"github.com/paylink/escrow-manager/internal/fsm"
)

var base = time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

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

func waiting(id, pl string) domain.Escrow {
	return domain.Escrow{
		ID: id, PLID: pl, CreatorAddr: "0xc", PayeeAddr: "0xp", RefundTo: "0xr",
		Amount: "123456789012345678901234567890", Currency: "KES",
		ConditionType: domain.ConditionMultiPartyApproval,
		ConditionParams: domain.ConditionParams{
			Approvers: []string{"0xa1", "0xa2"}, Threshold: 2,
		},
		State: fsm.StateWaiting, TimeoutAt: base.Add(time.Hour),
		CreatedAt: base, UpdatedAt: base,
	}
}

func TestPostgresCRUDAndDuplicate(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	e := waiting("ESC_1", "PLK_1")
	if err := s.CreateEscrow(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateEscrow(ctx, waiting("ESC_2", "PLK_1")); !errors.Is(err, domain.ErrEscrowExists) {
		t.Fatalf("want ErrEscrowExists, got %v", err)
	}
	got, err := s.GetEscrow(ctx, "ESC_1")
	if err != nil {
		t.Fatalf("GetEscrow: %v", err)
	}
	// Round-trip fidelity: numeric(30,0) amount, jsonb params, state, times.
	if got.Amount != e.Amount || got.Currency != "KES" || got.State != fsm.StateWaiting {
		t.Fatalf("round-trip: %+v", got)
	}
	if len(got.ConditionParams.Approvers) != 2 || got.ConditionParams.Threshold != 2 {
		t.Fatalf("params round-trip: %+v", got.ConditionParams)
	}
	if !got.TimeoutAt.Equal(e.TimeoutAt) {
		t.Fatalf("timeout_at round-trip: %v != %v", got.TimeoutAt, e.TimeoutAt)
	}
	if _, err := s.GetEscrow(ctx, "ESC_missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPostgresListEscrows(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a := waiting("ESC_a", "PLK_a")
	b := waiting("ESC_b", "PLK_b")
	b.CreatedAt = base.Add(time.Minute)
	c := waiting("ESC_c", "PLK_c")
	c.CreatorAddr = "0xother"
	for _, e := range []domain.Escrow{a, b, c} {
		if err := s.CreateEscrow(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.ListEscrows(ctx, "0xc", "", 20)
	if err != nil || len(got) != 2 || got[0].ID != "ESC_b" {
		t.Fatalf("list: %+v err=%v", got, err)
	}
	got, _ = s.ListEscrows(ctx, "0xc", string(fsm.StateWaiting), 1)
	if len(got) != 1 {
		t.Fatalf("limit+state: %+v", got)
	}
	got, _ = s.ListEscrows(ctx, "0xc", string(fsm.StateReleased), 0)
	if len(got) != 0 {
		t.Fatalf("state filter: %+v", got)
	}
}

func TestPostgresMutateApprovalPKAndRelease(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.CreateEscrow(ctx, waiting("ESC_m", "PLK_m")); err != nil {
		t.Fatal(err)
	}
	// First approval.
	got, err := s.Mutate(ctx, "ESC_m", func(e domain.Escrow, approvals []string) (domain.Update, error) {
		if len(approvals) != 0 {
			t.Fatalf("approvals = %v", approvals)
		}
		return domain.Update{AddApproval: "0xa1"}, nil
	})
	if err != nil || got.State != fsm.StateWaiting {
		t.Fatalf("first approval: %+v err=%v", got, err)
	}
	// Duplicate approval is a PK no-op; second approver + release in ONE transaction.
	got, err = s.Mutate(ctx, "ESC_m", func(e domain.Escrow, approvals []string) (domain.Update, error) {
		if len(approvals) != 1 || approvals[0] != "0xa1" {
			t.Fatalf("approvals = %v", approvals)
		}
		return domain.Update{AddApproval: "0xa1"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.Mutate(ctx, "ESC_m", func(e domain.Escrow, approvals []string) (domain.Update, error) {
		if len(approvals) != 1 {
			t.Fatalf("duplicate approval must not add a row: %v", approvals)
		}
		return domain.Update{AddApproval: "0xa2", SetState: fsm.StateReleased}, nil
	})
	if err != nil || got.State != fsm.StateReleased {
		t.Fatalf("release: %+v err=%v", got, err)
	}
	if got.UpdatedAt.Equal(base) {
		t.Fatal("updated_at must advance")
	}
	// fn error aborts the transaction (no approval row is kept).
	boom := errors.New("boom")
	if _, err := s.Mutate(ctx, "ESC_m", func(domain.Escrow, []string) (domain.Update, error) {
		return domain.Update{AddApproval: "0xa9"}, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("want fn error, got %v", err)
	}
	_, err = s.Mutate(ctx, "ESC_m", func(e domain.Escrow, approvals []string) (domain.Update, error) {
		if len(approvals) != 2 {
			t.Fatalf("rolled-back approval leaked: %v", approvals)
		}
		return domain.Update{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(ctx, "ESC_missing", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPostgresApplyFundingDbDedupe(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.CreateEscrow(ctx, waiting("ESC_f", "PLK_f")); err != nil {
		t.Fatal(err)
	}
	fund := func(e domain.Escrow, _ []string) (domain.Update, error) {
		return domain.Update{SetFunded: true, FundedTxHash: "0xtx"}, nil
	}
	got, applied, err := s.ApplyFunding(ctx, "PLK_f", "0xtx", fund)
	if err != nil || !applied || !got.Funded || got.FundedTxHash != "0xtx" {
		t.Fatalf("first: applied=%v %+v err=%v", applied, got, err)
	}
	// Redelivery: the processed_events row makes it a no-op (RunOnce returns ran=false).
	calls := 0
	_, applied, err = s.ApplyFunding(ctx, "PLK_f", "0xtx", func(domain.Escrow, []string) (domain.Update, error) {
		calls++
		return domain.Update{}, nil
	})
	if err != nil || applied || calls != 0 {
		t.Fatalf("duplicate: applied=%v calls=%d err=%v", applied, calls, err)
	}
	// Unknown paylink.
	if _, _, err := s.ApplyFunding(ctx, "PLK_missing", "0xtx", fund); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	// fn error rolls back BOTH the dedupe mark and the update → a retry runs fn again.
	boom := errors.New("boom")
	if _, _, err := s.ApplyFunding(ctx, "PLK_f", "0xtx2", func(domain.Escrow, []string) (domain.Update, error) {
		return domain.Update{}, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("want fn error, got %v", err)
	}
	retried := 0
	_, applied, err = s.ApplyFunding(ctx, "PLK_f", "0xtx2", func(domain.Escrow, []string) (domain.Update, error) {
		retried++
		return domain.Update{}, nil
	})
	if err != nil || !applied || retried != 1 {
		t.Fatalf("retry after rollback: applied=%v retried=%d err=%v", applied, retried, err)
	}
	// The funded flag never unsets and the first tx hash is kept.
	got, _ = s.GetEscrow(ctx, "ESC_f")
	if !got.Funded || got.FundedTxHash != "0xtx" {
		t.Fatalf("funded flag drifted: %+v", got)
	}
}

func TestPostgresSweepCAS(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	rel := base.Add(time.Minute)

	due := waiting("ESC_s1", "PLK_s1")
	due.ConditionType = domain.ConditionTimeLock
	due.ConditionParams = domain.ConditionParams{ReleaseAt: &rel}
	due.Funded = true
	due.ReleaseAt = &rel

	unfunded := waiting("ESC_s2", "PLK_s2")
	unfunded.ConditionType = domain.ConditionTimeLock
	unfunded.ConditionParams = domain.ConditionParams{ReleaseAt: &rel}
	unfunded.ReleaseAt = &rel

	timedOut := waiting("ESC_s3", "PLK_s3")
	timedOut.TimeoutAt = base.Add(time.Minute)

	disputed := waiting("ESC_s4", "PLK_s4")
	disputed.State = fsm.StateDisputed
	disputed.TimeoutAt = base.Add(time.Minute)

	for _, e := range []domain.Escrow{due, unfunded, timedOut, disputed} {
		if err := s.CreateEscrow(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	released, err := s.ReleaseDueTimeLocks(ctx, base.Add(2*time.Minute))
	if err != nil || len(released) != 1 || released[0].ID != "ESC_s1" || released[0].State != fsm.StateReleased {
		t.Fatalf("released = %+v err=%v", released, err)
	}
	refunded, err := s.RefundTimedOut(ctx, base.Add(2*time.Minute))
	if err != nil || len(refunded) != 1 || refunded[0].ID != "ESC_s3" || refunded[0].State != fsm.StateRefunded {
		t.Fatalf("refunded = %+v err=%v", refunded, err)
	}
	if got, _ := s.GetEscrow(ctx, "ESC_s4"); got.State != fsm.StateDisputed {
		t.Fatalf("DISPUTED row touched: %+v", got)
	}
	// CAS: a second pass matches nothing.
	released, _ = s.ReleaseDueTimeLocks(ctx, base.Add(2*time.Minute))
	refunded, _ = s.RefundTimedOut(ctx, base.Add(2*time.Minute))
	if len(released) != 0 || len(refunded) != 0 {
		t.Fatalf("second sweep must be empty: %v %v", released, refunded)
	}
}

func TestPostgresPing(t *testing.T) {
	if err := newStore(t).Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

// TestPostgresClosedPoolErrors exercises the backend-error branches: every method must surface
// a transport failure (closed pool) instead of swallowing it or mis-mapping it to a domain error.
func TestPostgresClosedPoolErrors(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Close() // pgxpool.Close is idempotent; the t.Cleanup re-close is a no-op

	if err := s.Migrate(ctx); err == nil {
		t.Error("Migrate on a closed pool must error")
	}
	if err := s.Ping(ctx); err == nil {
		t.Error("Ping on a closed pool must error")
	}
	if err := s.CreateEscrow(ctx, waiting("ESC_z", "PLK_z")); err == nil || errors.Is(err, domain.ErrEscrowExists) {
		t.Errorf("CreateEscrow: %v", err)
	}
	if _, err := s.GetEscrow(ctx, "ESC_z"); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetEscrow: %v", err)
	}
	if _, err := s.ListEscrows(ctx, "0xc", "", 20); err == nil {
		t.Error("ListEscrows on a closed pool must error")
	}
	if _, err := s.Mutate(ctx, "ESC_z", nil); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Mutate: %v", err)
	}
	if _, _, err := s.ApplyFunding(ctx, "PLK_z", "0xtx", nil); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ApplyFunding: %v", err)
	}
	if _, err := s.ReleaseDueTimeLocks(ctx, base); err == nil {
		t.Error("ReleaseDueTimeLocks on a closed pool must error")
	}
	if _, err := s.RefundTimedOut(ctx, base); err == nil {
		t.Error("RefundTimedOut on a closed pool must error")
	}
}
