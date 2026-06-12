package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/paylink/escrow-manager/internal/domain"
)

type fakeSvc struct {
	plID, txHash string
	calls        int
	result       string
	err          error
}

func (f *fakeSvc) HandlePaylinkVerified(_ context.Context, plID, txHash string) (string, error) {
	f.calls++
	f.plID, f.txHash = plID, txHash
	return f.result, f.err
}

type fakeRecorder struct {
	results []string
}

func (f *fakeRecorder) EventConsumed(result string) { f.results = append(f.results, result) }

func TestHandleFiltersOtherEvents(t *testing.T) {
	svc := &fakeSvc{result: domain.ResultFunded}
	rec := &fakeRecorder{}
	h := New(svc, rec, nil)
	if err := h.Handle(context.Background(), "chain.paylink.created", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("other events must be nil-handled: %v", err)
	}
	if svc.calls != 0 || len(rec.results) != 0 {
		t.Fatalf("service must not be called: calls=%d results=%v", svc.calls, rec.results)
	}
}

func TestHandleDecodesEntityID(t *testing.T) {
	svc := &fakeSvc{result: domain.ResultReleased}
	rec := &fakeRecorder{}
	h := New(svc, rec, nil)
	payload := json.RawMessage(`{"entity_id":"PLK_1","entity_type":"paylink","kind":"paylink.verified","tx_hash":"0xtx"}`)
	if err := h.Handle(context.Background(), EventPaylinkVerified, payload); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if svc.plID != "PLK_1" || svc.txHash != "0xtx" {
		t.Fatalf("decoded %q/%q", svc.plID, svc.txHash)
	}
	if len(rec.results) != 1 || rec.results[0] != domain.ResultReleased {
		t.Fatalf("results = %v", rec.results)
	}
}

func TestHandleFallsBackToPLID(t *testing.T) {
	svc := &fakeSvc{result: domain.ResultFunded}
	h := New(svc, nil, nil) // nil recorder must be safe
	payload := json.RawMessage(`{"pl_id":"PLK_2","tx_hash":"0xtx2"}`)
	if err := h.Handle(context.Background(), EventPaylinkVerified, payload); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if svc.plID != "PLK_2" {
		t.Fatalf("pl_id fallback: %q", svc.plID)
	}
}

func TestHandlePoisonSafe(t *testing.T) {
	svc := &fakeSvc{}
	rec := &fakeRecorder{}
	h := New(svc, rec, nil)
	// Undecodable payload: nil (skip+commit), service untouched.
	if err := h.Handle(context.Background(), EventPaylinkVerified, json.RawMessage(`{not json`)); err != nil {
		t.Fatalf("poison payload must be nil-handled: %v", err)
	}
	// Missing paylink id: nil (skip+commit), service untouched.
	if err := h.Handle(context.Background(), EventPaylinkVerified, json.RawMessage(`{"tx_hash":"0x"}`)); err != nil {
		t.Fatalf("missing id must be nil-handled: %v", err)
	}
	if svc.calls != 0 {
		t.Fatalf("service must not be called: %d", svc.calls)
	}
	if len(rec.results) != 2 || rec.results[0] != domain.ResultIgnored || rec.results[1] != domain.ResultIgnored {
		t.Fatalf("results = %v", rec.results)
	}
}

func TestHandleErrorPropagates(t *testing.T) {
	boom := errors.New("db down")
	svc := &fakeSvc{err: boom}
	rec := &fakeRecorder{}
	h := New(svc, rec, nil)
	err := h.Handle(context.Background(), EventPaylinkVerified, json.RawMessage(`{"entity_id":"PLK_3"}`))
	if !errors.Is(err, boom) {
		t.Fatalf("service error must propagate (no offset commit → redelivery), got %v", err)
	}
	if len(rec.results) != 1 || rec.results[0] != "error" {
		t.Fatalf("results = %v", rec.results)
	}
}
