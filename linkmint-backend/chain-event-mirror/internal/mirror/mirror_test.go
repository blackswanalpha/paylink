package mirror

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/paylink/chain-event-mirror/internal/chain"
	"github.com/paylink/chain-event-mirror/internal/metrics"
)

type fakePublisher struct {
	name    string
	key     string
	payload any
	err     error
	calls   int
}

func (f *fakePublisher) Publish(_ context.Context, name, key string, payload any) error {
	f.calls++
	f.name, f.key, f.payload = name, key, payload
	return f.err
}

func TestHandle_MapsChainEventToChainTopicEnvelope(t *testing.T) {
	fp := &fakePublisher{}
	mr := New(fp, metrics.New(), nil)
	ev := chain.Event{
		Kind:        "paylink.verified",
		EntityType:  "paylink",
		EntityID:    "PLK_1",
		BlockHeight: 42,
		Timestamp:   1234,
		TxHash:      "0xabc",
		ToState:     "VERIFIED",
		Transition:  "settle",
		Data:        json.RawMessage(`{"proofHash":"0xdef"}`),
	}
	if err := mr.Handle(context.Background(), ev); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if fp.name != "chain.paylink.verified" {
		t.Errorf("name = %q, want chain.paylink.verified", fp.name)
	}
	if fp.key != "PLK_1" {
		t.Errorf("key = %q, want PLK_1", fp.key)
	}
	p, ok := fp.payload.(eventPayload)
	if !ok {
		t.Fatalf("payload type = %T", fp.payload)
	}
	if p.EntityID != "PLK_1" || p.ToState != "VERIFIED" || p.BlockHeight != 42 || p.TxHash != "0xabc" {
		t.Errorf("payload = %+v", p)
	}
	if string(p.Data) != `{"proofHash":"0xdef"}` {
		t.Errorf("data = %s", p.Data)
	}
}

func TestHandle_EmptyKindIsSkipped(t *testing.T) {
	fp := &fakePublisher{}
	mr := New(fp, nil, nil)
	if err := mr.Handle(context.Background(), chain.Event{EntityID: "X"}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if fp.calls != 0 {
		t.Errorf("expected no publish, got %d calls", fp.calls)
	}
}

func TestHandle_PublishErrorPropagates(t *testing.T) {
	fp := &fakePublisher{err: errors.New("broker down")}
	mr := New(fp, metrics.New(), nil)
	err := mr.Handle(context.Background(), chain.Event{Kind: "paylink.failed", EntityID: "PLK_2"})
	if err == nil {
		t.Fatal("expected the publish error to propagate")
	}
}
