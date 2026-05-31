package subscriber

import (
	"context"
	"testing"
	"time"

	"github.com/paylink/payment-orchestrator/internal/chain"
	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/lifecycle"
	"github.com/paylink/payment-orchestrator/internal/metrics"
	"github.com/paylink/payment-orchestrator/internal/store/memory"
)

type fakeSource struct{ events []chain.Event }

func (f *fakeSource) Run(ctx context.Context, handle func(context.Context, chain.Event) error) error {
	for _, ev := range f.events {
		if err := handle(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

type nopPayLinks struct{}

func (nopPayLinks) GetPayLink(context.Context, string) (*domain.PayLinkRecord, error) {
	return nil, nil
}

type nopChain struct{}

func (nopChain) PayLinkStatus(context.Context, string) (string, bool, error) { return "", false, nil }

func setup(t *testing.T) (*memory.Store, *domain.Service) {
	t.Helper()
	store := memory.New()
	if err := store.CreatePayment(context.Background(), domain.Payment{
		ID: "p1", PayLinkID: "0xabc", Rail: "mpesa", Status: lifecycle.StateAwaitingPayment,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	svc := domain.NewService(store, nopPayLinks{}, nopChain{}, nil, nil)
	return store, svc
}

func TestSubscriberAdvancesOnVerified(t *testing.T) {
	store, svc := setup(t)
	src := &fakeSource{events: []chain.Event{
		{Sequence: 1, Kind: chain.KindPayLinkVerified, EntityType: chain.EntityPayLink, EntityID: "0xabc", ToState: "VERIFIED"},
	}}
	if err := New(src, svc, metrics.New(), nil).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, _ := store.GetPaymentByPayLink(context.Background(), "0xabc")
	if got.Status != lifecycle.StateSettled {
		t.Fatalf("status = %v, want SETTLED", got.Status)
	}
}

func TestSubscriberDuplicateIsNoop(t *testing.T) {
	store, svc := setup(t)
	ev := chain.Event{Sequence: 1, Kind: chain.KindPayLinkVerified, EntityType: chain.EntityPayLink, EntityID: "0xabc", ToState: "VERIFIED"}
	src := &fakeSource{events: []chain.Event{ev, ev}} // delivered twice
	if err := New(src, svc, metrics.New(), nil).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, _ := store.GetPaymentByPayLink(context.Background(), "0xabc")
	if got.Status != lifecycle.StateSettled {
		t.Fatalf("status = %v, want SETTLED", got.Status)
	}
}

func TestSubscriberIgnoresIrrelevant(t *testing.T) {
	store, svc := setup(t)
	src := &fakeSource{events: []chain.Event{
		{Kind: "validator.staked", EntityType: "validator", EntityID: "0xval"},                                         // non-paylink
		{Kind: "paylink.voted", EntityType: chain.EntityPayLink, EntityID: "0xabc"},                                    // non-settlement
		{Kind: chain.KindPayLinkVerified, EntityType: chain.EntityPayLink, EntityID: "0xunknown", ToState: "VERIFIED"}, // not orchestrated
		{EntityType: chain.EntityPayLink, EntityID: ""},                                                                // empty id
	}}
	if err := New(src, svc, metrics.New(), nil).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, _ := store.GetPaymentByPayLink(context.Background(), "0xabc")
	if got.Status != lifecycle.StateAwaitingPayment {
		t.Fatalf("irrelevant events must not advance state, got %v", got.Status)
	}
}
