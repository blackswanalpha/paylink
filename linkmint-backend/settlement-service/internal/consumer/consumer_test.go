package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/paylink/settlement-service/internal/domain"
)

type fakeSvc struct {
	verified []domain.VerifiedEvent
	fee      []domain.FeeEvent
	merchant []domain.MerchantOnboardedEvent
	bank     []domain.BankAccountVerifiedEvent
	clawback []domain.ClawbackEvent
	err      error
}

func (f *fakeSvc) HandleVerified(_ context.Context, ev domain.VerifiedEvent) (string, error) {
	f.verified = append(f.verified, ev)
	return domain.ResultSettled, f.err
}
func (f *fakeSvc) HandleFee(_ context.Context, ev domain.FeeEvent) (string, error) {
	f.fee = append(f.fee, ev)
	return domain.ResultFee, f.err
}
func (f *fakeSvc) HandleMerchantOnboarded(_ context.Context, ev domain.MerchantOnboardedEvent) (string, error) {
	f.merchant = append(f.merchant, ev)
	return domain.ResultMerchant, f.err
}
func (f *fakeSvc) HandleBankAccountVerified(_ context.Context, ev domain.BankAccountVerifiedEvent) (string, error) {
	f.bank = append(f.bank, ev)
	return domain.ResultBank, f.err
}
func (f *fakeSvc) HandleClawback(_ context.Context, ev domain.ClawbackEvent) (string, error) {
	f.clawback = append(f.clawback, ev)
	return domain.ResultClawback, f.err
}

type fakeRec struct{ results []string }

func (r *fakeRec) EventConsumed(result string) { r.results = append(r.results, result) }

func TestHandleVerifiedDecodes(t *testing.T) {
	svc := &fakeSvc{}
	rec := &fakeRec{}
	h := New(svc, rec, nil)
	payload := json.RawMessage(`{"entity_id":"PLK_A","tx_hash":"0xtx","timestamp":1718000000000,"data":{"payee":"0xaa","amount":1500}}`)
	if err := h.Handle(context.Background(), EventPaylinkVerified, payload); err != nil {
		t.Fatal(err)
	}
	if len(svc.verified) != 1 {
		t.Fatalf("verified calls=%d", len(svc.verified))
	}
	ev := svc.verified[0]
	if ev.PLID != "PLK_A" || ev.Payee != "0xaa" || ev.Amount.Int64() != 1500 || ev.TxHash != "0xtx" {
		t.Fatalf("decoded = %+v", ev)
	}
	if ev.OccurredAt.IsZero() {
		t.Fatal("OccurredAt should be set from timestamp")
	}
	if len(rec.results) != 1 || rec.results[0] != domain.ResultSettled {
		t.Fatalf("metrics = %v", rec.results)
	}
}

func TestHandleFeeDecodes(t *testing.T) {
	svc := &fakeSvc{}
	h := New(svc, nil, nil)
	payload := json.RawMessage(`{"entity_id":"PLK_A","data":{"paylinkId":"PLK_A","amount":1500,"totalFee":7}}`)
	if err := h.Handle(context.Background(), EventFeeCollected, payload); err != nil {
		t.Fatal(err)
	}
	if len(svc.fee) != 1 || svc.fee[0].PLID != "PLK_A" || svc.fee[0].ChainFee.Int64() != 7 {
		t.Fatalf("fee = %+v", svc.fee)
	}
}

func TestHandleMerchantBankClawback(t *testing.T) {
	svc := &fakeSvc{}
	h := New(svc, nil, nil)
	ctx := context.Background()
	_ = h.Handle(ctx, EventMerchantOnboard, json.RawMessage(`{"merchant_id":"m1","status":"ACTIVE"}`))
	_ = h.Handle(ctx, EventBankVerified, json.RawMessage(`{"bank_account_id":"b1","merchant_id":"m1","rail":"mpesa","currency":"KES","status":"VERIFIED"}`))
	_ = h.Handle(ctx, EventClawbackRequest, json.RawMessage(`{"dispute_id":"d1","paylink_id":"PLK_A","amount_minor":400,"currency":"KES"}`))

	if len(svc.merchant) != 1 || svc.merchant[0].MerchantID != "m1" {
		t.Fatalf("merchant = %+v", svc.merchant)
	}
	if len(svc.bank) != 1 || svc.bank[0].Rail != "mpesa" {
		t.Fatalf("bank = %+v", svc.bank)
	}
	if len(svc.clawback) != 1 || svc.clawback[0].RefundID != "d1" || svc.clawback[0].PLID != "PLK_A" || svc.clawback[0].Amount.Int64() != 400 {
		t.Fatalf("clawback = %+v", svc.clawback)
	}
}

func TestHandleUnknownNameIsNoop(t *testing.T) {
	svc := &fakeSvc{}
	h := New(svc, nil, nil)
	if err := h.Handle(context.Background(), "paylink.created", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if len(svc.verified)+len(svc.fee)+len(svc.merchant) != 0 {
		t.Fatal("unknown event should not dispatch")
	}
}

func TestHandlePoisonIsCommitted(t *testing.T) {
	svc := &fakeSvc{}
	rec := &fakeRec{}
	h := New(svc, rec, nil)
	if err := h.Handle(context.Background(), EventPaylinkVerified, json.RawMessage(`{not json`)); err != nil {
		t.Fatalf("poison payload should be committed (nil), got %v", err)
	}
	if len(rec.results) != 1 || rec.results[0] != domain.ResultIgnored {
		t.Fatalf("metrics = %v, want [ignored]", rec.results)
	}
}

func TestHandleServiceErrorRedelivers(t *testing.T) {
	svc := &fakeSvc{err: errors.New("boom")}
	rec := &fakeRec{}
	h := New(svc, rec, nil)
	payload := json.RawMessage(`{"entity_id":"PLK_A","data":{"payee":"0xaa","amount":1500}}`)
	if err := h.Handle(context.Background(), EventPaylinkVerified, payload); err == nil {
		t.Fatal("service error should propagate (no commit → redelivery)")
	}
	if len(rec.results) != 1 || rec.results[0] != domain.ResultError {
		t.Fatalf("metrics = %v, want [error]", rec.results)
	}
}
