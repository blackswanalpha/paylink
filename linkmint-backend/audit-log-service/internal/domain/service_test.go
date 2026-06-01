package domain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/paylink/audit-log-service/internal/hashing"
	"github.com/paylink/audit-log-service/internal/httpx"
)

type fakeStore struct {
	entries      []Entry
	appendErr    error
	verifyResult VerifyResult
	verifyErr    error
}

func (f *fakeStore) Append(_ context.Context, in AppendInput) (Entry, error) {
	if f.appendErr != nil {
		return Entry{}, f.appendErr
	}
	prev := GenesisHash()
	if n := len(f.entries); n > 0 {
		prev = f.entries[n-1].EntryHash
	}
	e, err := BuildEntry(in, prev)
	if err != nil {
		return Entry{}, err
	}
	e.EntryID = int64(len(f.entries) + 1)
	f.entries = append(f.entries, e)
	return e, nil
}

func (f *fakeStore) GetByID(_ context.Context, id int64) (Entry, error) {
	for _, e := range f.entries {
		if e.EntryID == id {
			return e, nil
		}
	}
	return Entry{}, ErrNotFound
}
func (f *fakeStore) Query(_ context.Context, _ QueryFilter) (Page, error) {
	return Page{Items: f.entries}, nil
}
func (f *fakeStore) VerifyRange(_ context.Context, _, _ *time.Time) (VerifyResult, error) {
	return f.verifyResult, f.verifyErr
}
func (f *fakeStore) Tail(_ context.Context) ([]byte, int64, error) {
	return GenesisHash(), int64(len(f.entries)), nil
}
func (f *fakeStore) Ping(_ context.Context) error { return nil }

type fakePub struct{ events []string }

func (p *fakePub) Publish(_ context.Context, name, _ string, _ any) error {
	p.events = append(p.events, name)
	return nil
}

func validInput() AppendInput {
	return AppendInput{
		Actor:    Actor{Kind: ActorService},
		Action:   "merchant.suspend",
		Resource: "merchant:abc",
		Context:  json.RawMessage(`{"trace_id":"t1"}`),
	}
}

func TestAppendStampsAndEmits(t *testing.T) {
	st := &fakeStore{}
	pub := &fakePub{}
	svc := NewService(st, pub, nil)

	e, err := svc.Append(context.Background(), validInput())
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if e.EntryID != 1 || len(e.EntryHash) != hashing.HashLen {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if e.OccurredAt.IsZero() {
		t.Fatal("occurred_at should be stamped when absent")
	}
	if len(pub.events) != 1 || pub.events[0] != EventEntryAdded {
		t.Fatalf("expected one %s event, got %v", EventEntryAdded, pub.events)
	}
}

func TestAppendValidationRejects(t *testing.T) {
	cases := map[string]func(*AppendInput){
		"bad kind":        func(in *AppendInput) { in.Actor.Kind = "robot" },
		"empty action":    func(in *AppendInput) { in.Action = "  " },
		"empty resource":  func(in *AppendInput) { in.Resource = "" },
		"context array":   func(in *AppendInput) { in.Context = json.RawMessage(`[1,2]`) },
		"context missing": func(in *AppendInput) { in.Context = nil },
		"bad before json": func(in *AppendInput) { in.Before = json.RawMessage(`{bad`) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			st := &fakeStore{}
			pub := &fakePub{}
			svc := NewService(st, pub, nil)
			in := validInput()
			mutate(&in)
			_, err := svc.Append(context.Background(), in)
			var ae *httpx.AppError
			if !errors.As(err, &ae) || ae.Code != httpx.CodeInvalidPayload {
				t.Fatalf("want INVALID_PAYLOAD, got %v", err)
			}
			if len(st.entries) != 0 || len(pub.events) != 0 {
				t.Fatal("invalid input must not append or emit")
			}
		})
	}
}

func TestGetNotFoundAndProof(t *testing.T) {
	st := &fakeStore{}
	svc := NewService(st, &fakePub{}, nil)

	_, _, err := svc.Get(context.Background(), 1)
	var ae *httpx.AppError
	if !errors.As(err, &ae) || ae.Code != httpx.CodeEntryNotFound {
		t.Fatalf("want ENTRY_NOT_FOUND, got %v", err)
	}

	e, _ := svc.Append(context.Background(), validInput())
	got, proof, err := svc.Get(context.Background(), e.EntryID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.EntryID != e.EntryID || !proof.Valid || proof.ChainType != "linear" {
		t.Fatalf("unexpected proof: %+v", proof)
	}
}

func TestVerifyEmitsOnBreak(t *testing.T) {
	broken := int64(7)
	st := &fakeStore{verifyResult: VerifyResult{OK: false, BrokenAt: &broken}}
	pub := &fakePub{}
	svc := NewService(st, pub, nil)

	res, err := svc.Verify(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || *res.BrokenAt != 7 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(pub.events) != 1 || pub.events[0] != EventVerificationFailed {
		t.Fatalf("expected %s event, got %v", EventVerificationFailed, pub.events)
	}
}

func TestVerifyOKNoEvent(t *testing.T) {
	st := &fakeStore{verifyResult: VerifyResult{OK: true}}
	pub := &fakePub{}
	svc := NewService(st, pub, nil)
	res, err := svc.Verify(context.Background(), nil, nil)
	if err != nil || !res.OK {
		t.Fatalf("want ok, got %+v err=%v", res, err)
	}
	if len(pub.events) != 0 {
		t.Fatalf("ok verify must not emit, got %v", pub.events)
	}
}

func TestCheckEntryDetectsTamper(t *testing.T) {
	st := &fakeStore{}
	svc := NewService(st, &fakePub{}, nil, WithClock(func() time.Time { return time.Unix(1700000000, 0).UTC() }))
	e, _ := svc.Append(context.Background(), validInput())

	selfOK, linkOK := CheckEntry(e, GenesisHash())
	if !selfOK || !linkOK {
		t.Fatalf("clean entry should verify: self=%v link=%v", selfOK, linkOK)
	}
	// tamper the canonical (hashed) bytes: stored entry_hash no longer matches the recompute
	e.Canonical = []byte(`{"tampered":true}`)
	selfOK, _ = CheckEntry(e, GenesisHash())
	if selfOK {
		t.Fatal("canonical tamper must fail the self-hash check")
	}
	// tamper the linkage: a second entry's prev_hash is not genesis
	e2, _ := svc.Append(context.Background(), validInput())
	_, linkOK = CheckEntry(e2, GenesisHash())
	if linkOK {
		t.Fatal("wrong predecessor must fail the linkage check")
	}
}

func TestActorKindValid(t *testing.T) {
	for _, k := range []ActorKind{ActorUser, ActorService, ActorSystem} {
		if !k.Valid() {
			t.Fatalf("%s should be valid", k)
		}
	}
	if ActorKind("nope").Valid() {
		t.Fatal("unknown kind must be invalid")
	}
}

func TestActorIDRoundTripsThroughHashInput(t *testing.T) {
	id := uuid.New()
	e := Entry{Actor: Actor{ID: &id, Kind: ActorUser}, Context: json.RawMessage(`{}`)}
	if e.HashInput().ActorID != id.String() {
		t.Fatal("actor id must flow into the hash input")
	}
}
