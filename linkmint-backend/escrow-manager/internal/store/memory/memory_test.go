package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paylink/escrow-manager/internal/domain"
	"github.com/paylink/escrow-manager/internal/fsm"
)

var base = time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

func waiting(id, pl string) domain.Escrow {
	return domain.Escrow{
		ID: id, PLID: pl, CreatorAddr: "0xc", PayeeAddr: "0xp", RefundTo: "0xr",
		Amount: "100", Currency: "KES", ConditionType: domain.ConditionDeliveryConfirmation,
		State: fsm.StateWaiting, TimeoutAt: base.Add(time.Hour), CreatedAt: base, UpdatedAt: base,
	}
}

func TestCreateGetDuplicate(t *testing.T) {
	s := New()
	ctx := context.Background()
	e := waiting("ESC_1", "PLK_1")
	if err := s.CreateEscrow(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateEscrow(ctx, waiting("ESC_2", "PLK_1")); !errors.Is(err, domain.ErrEscrowExists) {
		t.Fatalf("want ErrEscrowExists, got %v", err)
	}
	got, err := s.GetEscrow(ctx, "ESC_1")
	if err != nil || got.PLID != "PLK_1" {
		t.Fatalf("GetEscrow: %v / %+v", err, got)
	}
	if _, err := s.GetEscrow(ctx, "ESC_missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestListEscrows(t *testing.T) {
	s := New()
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
	got, _ = s.ListEscrows(ctx, "0xc", "", 1)
	if len(got) != 1 {
		t.Fatalf("limit: %+v", got)
	}
	got, _ = s.ListEscrows(ctx, "0xc", string(fsm.StateReleased), 0)
	if len(got) != 0 {
		t.Fatalf("state filter: %+v", got)
	}
}

func TestMutateApprovalAndState(t *testing.T) {
	s := New()
	ctx := context.Background()
	if err := s.CreateEscrow(ctx, waiting("ESC_m", "PLK_m")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Mutate(ctx, "ESC_m", func(e domain.Escrow, approvals []string) (domain.Update, error) {
		if len(approvals) != 0 {
			t.Fatalf("approvals = %v", approvals)
		}
		return domain.Update{AddApproval: "0xc", SetState: fsm.StateReleased}, nil
	})
	if err != nil || got.State != fsm.StateReleased {
		t.Fatalf("mutate: %+v err=%v", got, err)
	}
	if got.UpdatedAt.Equal(base) {
		t.Fatal("updated_at must advance on a state change")
	}
	// Approvals visible on the next mutate; duplicate adds are no-ops.
	_, err = s.Mutate(ctx, "ESC_m", func(e domain.Escrow, approvals []string) (domain.Update, error) {
		if len(approvals) != 1 || approvals[0] != "0xc" {
			t.Fatalf("approvals = %v", approvals)
		}
		return domain.Update{AddApproval: "0xc"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(s.Approvals("ESC_m")); n != 1 {
		t.Fatalf("approvals = %d, want 1", n)
	}
	// fn error rolls back.
	boom := errors.New("boom")
	if _, err := s.Mutate(ctx, "ESC_m", func(domain.Escrow, []string) (domain.Update, error) {
		return domain.Update{}, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("want fn error, got %v", err)
	}
	if _, err := s.Mutate(ctx, "ESC_missing", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestApplyFundingDedupe(t *testing.T) {
	s := New()
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
	_, applied, err = s.ApplyFunding(ctx, "PLK_f", "0xtx", fund)
	if err != nil || applied {
		t.Fatalf("duplicate must skip: applied=%v err=%v", applied, err)
	}
	if _, _, err := s.ApplyFunding(ctx, "PLK_missing", "0xtx", fund); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	// fn error → not marked processed → retry runs fn again.
	boom := errors.New("boom")
	calls := 0
	failing := func(domain.Escrow, []string) (domain.Update, error) { calls++; return domain.Update{}, boom }
	if _, _, err := s.ApplyFunding(ctx, "PLK_f", "0xtx2", failing); !errors.Is(err, boom) {
		t.Fatalf("want fn error, got %v", err)
	}
	if _, _, err := s.ApplyFunding(ctx, "PLK_f", "0xtx2", failing); !errors.Is(err, boom) {
		t.Fatalf("retry must rerun fn, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("fn calls = %d, want 2 (no poison-lock)", calls)
	}
}

func TestSweepPaths(t *testing.T) {
	s := New()
	ctx := context.Background()
	rel := base.Add(time.Minute)

	due := waiting("ESC_s1", "PLK_s1")
	due.ConditionType = domain.ConditionTimeLock
	due.Funded = true
	due.ReleaseAt = &rel

	unfunded := waiting("ESC_s2", "PLK_s2")
	unfunded.ConditionType = domain.ConditionTimeLock
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
	// ESC_s2's timeout is base+1h — not due at base+2min, so only ESC_s3 refunds.
	refunded, err := s.RefundTimedOut(ctx, base.Add(2*time.Minute))
	if err != nil || len(refunded) != 1 || refunded[0].ID != "ESC_s3" || refunded[0].State != fsm.StateRefunded {
		t.Fatalf("refunded = %+v err=%v", refunded, err)
	}
	// DISPUTED untouched; second pass is a no-op (CAS).
	if got, _ := s.GetEscrow(ctx, "ESC_s4"); got.State != fsm.StateDisputed {
		t.Fatalf("disputed touched: %+v", got)
	}
	released, _ = s.ReleaseDueTimeLocks(ctx, base.Add(2*time.Minute))
	refunded, _ = s.RefundTimedOut(ctx, base.Add(2*time.Minute))
	if len(released) != 0 || len(refunded) != 0 {
		t.Fatalf("second sweep must be empty: %v %v", released, refunded)
	}
}

func TestPing(t *testing.T) {
	if err := New().Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}
