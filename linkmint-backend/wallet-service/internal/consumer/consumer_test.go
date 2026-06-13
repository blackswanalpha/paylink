package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/paylink/wallet-service/internal/domain"
)

// stubService records the calls the consumer makes and returns canned results/errors.
type stubService struct {
	transfer  *domain.TransferEvent
	staked    *domain.StakedEvent
	unstakeS  *domain.UnstakeStartedEvent
	unstakeC  *domain.UnstakeCompletedEvent
	slashed   *domain.SlashedEvent
	rewarded  *domain.RewardedEvent
	feeColl   *domain.FeeCollectedEvent
	feeDist   *domain.FeeDistributedEvent
	burned    *domain.TokenBurnedEvent
	result    string
	returnErr error
}

func (s *stubService) ret() (string, error) {
	if s.returnErr != nil {
		return "", s.returnErr
	}
	r := s.result
	if r == "" {
		r = domain.ResultProcessed
	}
	return r, nil
}

func (s *stubService) HandleTransfer(_ context.Context, ev domain.TransferEvent) (string, error) {
	s.transfer = &ev
	return s.ret()
}
func (s *stubService) HandleStaked(_ context.Context, ev domain.StakedEvent) (string, error) {
	s.staked = &ev
	return s.ret()
}
func (s *stubService) HandleUnstakeStarted(_ context.Context, ev domain.UnstakeStartedEvent) (string, error) {
	s.unstakeS = &ev
	return s.ret()
}
func (s *stubService) HandleUnstakeCompleted(_ context.Context, ev domain.UnstakeCompletedEvent) (string, error) {
	s.unstakeC = &ev
	return s.ret()
}
func (s *stubService) HandleSlashed(_ context.Context, ev domain.SlashedEvent) (string, error) {
	s.slashed = &ev
	return s.ret()
}
func (s *stubService) HandleRewarded(_ context.Context, ev domain.RewardedEvent) (string, error) {
	s.rewarded = &ev
	return s.ret()
}
func (s *stubService) HandleFeeCollected(_ context.Context, ev domain.FeeCollectedEvent) (string, error) {
	s.feeColl = &ev
	return s.ret()
}
func (s *stubService) HandleFeeDistributed(_ context.Context, ev domain.FeeDistributedEvent) (string, error) {
	s.feeDist = &ev
	return s.ret()
}
func (s *stubService) HandleTokenBurned(_ context.Context, ev domain.TokenBurnedEvent) (string, error) {
	s.burned = &ev
	return s.ret()
}

type stubRecorder struct{ results []string }

func (r *stubRecorder) EventConsumed(result string) { r.results = append(r.results, result) }

// envelope builds a chain-event-mirror projected payload for name's data blob.
func envelope(t *testing.T, entityID, txHash string, height uint64, ts int64, data any) json.RawMessage {
	t.Helper()
	d, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	p, err := json.Marshal(map[string]any{
		"entity_id": entityID, "tx_hash": txHash, "block_height": height, "timestamp": ts, "data": json.RawMessage(d),
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return p
}

func TestHandleDispatchesEachEvent(t *testing.T) {
	svc := &stubService{}
	rec := &stubRecorder{}
	h := New(svc, rec, nil)
	ctx := context.Background()

	mustHandle := func(name string, payload json.RawMessage) {
		if err := h.Handle(ctx, name, payload); err != nil {
			t.Fatalf("Handle(%s): %v", name, err)
		}
	}

	mustHandle(EventTransfer, envelope(t, "", "0xt", 5, 1000, map[string]any{"from": "0xfrom", "to": "0xto", "amount": 9}))
	if svc.transfer == nil || svc.transfer.From != "0xfrom" || svc.transfer.To != "0xto" || svc.transfer.Amount.Uint64() != 9 || svc.transfer.BlockHeight != 5 {
		t.Fatalf("transfer = %+v", svc.transfer)
	}
	if svc.transfer.OccurredAt.IsZero() {
		t.Fatal("transfer time not decoded")
	}

	mustHandle(EventStaked, envelope(t, "0xval", "0xs", 6, 2000, map[string]any{"amount": 50, "totalStaked": 150, "isActive": true}))
	if svc.staked == nil || svc.staked.Addr != "0xval" || svc.staked.TotalStaked.Uint64() != 150 || !svc.staked.IsActive {
		t.Fatalf("staked = %+v", svc.staked)
	}

	mustHandle(EventUnstakeStarted, envelope(t, "0xval", "0xus", 7, 3000, map[string]any{"amount": 20, "withdrawableAt": 9999}))
	if svc.unstakeS == nil || svc.unstakeS.WithdrawableAt == nil || svc.unstakeS.WithdrawableAt.Unix() != 9999 {
		t.Fatalf("unstake started = %+v", svc.unstakeS)
	}

	mustHandle(EventUnstakeCompleted, envelope(t, "0xval", "0xuc", 8, 4000, map[string]any{"amount": 20}))
	if svc.unstakeC == nil || svc.unstakeC.Amount.Uint64() != 20 {
		t.Fatalf("unstake completed = %+v", svc.unstakeC)
	}

	mustHandle(EventSlashed, envelope(t, "0xval", "0xsl", 9, 5000, map[string]any{"amount": 5, "reason": "dbl", "remaining": 145}))
	if svc.slashed == nil || svc.slashed.Reason != "dbl" || svc.slashed.Remaining.Uint64() != 145 {
		t.Fatalf("slashed = %+v", svc.slashed)
	}

	mustHandle(EventRewarded, envelope(t, "0xval", "0xr", 10, 6000, map[string]any{"amount": 3, "totalRewards": 33}))
	if svc.rewarded == nil || svc.rewarded.TotalRewards.Uint64() != 33 {
		t.Fatalf("rewarded = %+v", svc.rewarded)
	}

	mustHandle(EventFeeCollected, envelope(t, "0xpl", "0xfc", 11, 7000, map[string]any{"totalFee": 100, "validatorShare": 70, "treasuryShare": 20, "burnAmount": 10}))
	if svc.feeColl == nil || svc.feeColl.TotalFee.Uint64() != 100 || svc.feeColl.TreasuryShare.Uint64() != 20 {
		t.Fatalf("fee collected = %+v", svc.feeColl)
	}

	mustHandle(EventFeeDistributed, envelope(t, "0xfallback", "0xfd", 12, 8000, map[string]any{"amount": 70}))
	if svc.feeDist == nil || svc.feeDist.Validator != "0xfallback" || svc.feeDist.Amount.Uint64() != 70 {
		t.Fatalf("fee distributed (entity fallback) = %+v", svc.feeDist)
	}

	mustHandle(EventTokenBurned, envelope(t, "", "0xtb", 13, 9000, map[string]any{"amount": 10, "totalBurned": 999}))
	if svc.burned == nil || svc.burned.TotalBurned.Uint64() != 999 {
		t.Fatalf("token burned = %+v", svc.burned)
	}

	// Nine processed results recorded.
	processed := 0
	for _, r := range rec.results {
		if r == domain.ResultProcessed {
			processed++
		}
	}
	if processed != 9 {
		t.Fatalf("processed count = %d, want 9", processed)
	}
}

func TestHandleUnknownNameIsNoOp(t *testing.T) {
	svc := &stubService{}
	rec := &stubRecorder{}
	h := New(svc, rec, nil)
	if err := h.Handle(context.Background(), "merchant.onboarded", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("unknown name should be no-op, got %v", err)
	}
	if len(rec.results) != 0 {
		t.Fatalf("unknown name should record nothing, got %v", rec.results)
	}
}

func TestHandlePoisonPayloadIsCommitted(t *testing.T) {
	svc := &stubService{}
	rec := &stubRecorder{}
	h := New(svc, rec, nil)
	// Malformed envelope (not an object) → poison-safe: logged, committed (nil), recorded ignored.
	if err := h.Handle(context.Background(), EventStaked, json.RawMessage(`not-json`)); err != nil {
		t.Fatalf("poison should commit (nil), got %v", err)
	}
	if svc.staked != nil {
		t.Fatal("service should not be called on poison payload")
	}
	if len(rec.results) != 1 || rec.results[0] != domain.ResultIgnored {
		t.Fatalf("expected one ignored result, got %v", rec.results)
	}
}

func TestHandleServiceErrorRedelivers(t *testing.T) {
	svc := &stubService{returnErr: errors.New("db down")}
	rec := &stubRecorder{}
	h := New(svc, rec, nil)
	err := h.Handle(context.Background(), EventTokenBurned, envelope(t, "", "0x", 1, 1000, map[string]any{"totalBurned": 1}))
	if err == nil {
		t.Fatal("service error should propagate (→ redelivery)")
	}
	if len(rec.results) != 1 || rec.results[0] != domain.ResultError {
		t.Fatalf("expected one error result, got %v", rec.results)
	}
}

func TestHandleDuplicateResult(t *testing.T) {
	svc := &stubService{result: domain.ResultDuplicate}
	rec := &stubRecorder{}
	h := New(svc, rec, nil)
	if err := h.Handle(context.Background(), EventStaked, envelope(t, "0xval", "0xs", 1, 1000, map[string]any{"amount": 1, "totalStaked": 1})); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(rec.results) != 1 || rec.results[0] != domain.ResultDuplicate {
		t.Fatalf("expected duplicate result, got %v", rec.results)
	}
}
