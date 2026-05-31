package domain_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/httpx"
	"github.com/paylink/payment-orchestrator/internal/lifecycle"
	"github.com/paylink/payment-orchestrator/internal/store/memory"
)

var (
	plID  = "0x" + strings.Repeat("a", 64)
	clock = time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
)

type fakePayLinks struct {
	rec *domain.PayLinkRecord
	err error
}

func (f *fakePayLinks) GetPayLink(_ context.Context, _ string) (*domain.PayLinkRecord, error) {
	return f.rec, f.err
}

type fakeChain struct {
	status string
	found  bool
	err    error
	calls  int
}

func (f *fakeChain) PayLinkStatus(_ context.Context, _ string) (string, bool, error) {
	f.calls++
	return f.status, f.found, f.err
}

type capturePublisher struct{ events []string }

func (c *capturePublisher) Publish(_ context.Context, name, _ string, _ any) error {
	c.events = append(c.events, name)
	return nil
}

func newSvc(store domain.Store, pl domain.PayLinkLookup, ch domain.ChainReader, pub *capturePublisher) *domain.Service {
	var n int
	idgen := func() string { n++; return "pay-" + strconv.Itoa(n) }
	return domain.NewService(store, pl, ch, pub, nil,
		domain.WithClock(func() time.Time { return clock }),
		domain.WithIDGen(idgen),
	)
}

func mustAppErr(t *testing.T, err error) *httpx.AppError {
	t.Helper()
	var ae *httpx.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *httpx.AppError, got %v", err)
	}
	return ae
}

func createdRecord() *domain.PayLinkRecord {
	return &domain.PayLinkRecord{ID: plID, Status: "CREATED", Expiry: clock.Add(time.Hour)}
}

func TestInitiateHappy(t *testing.T) {
	store := memory.New()
	pub := &capturePublisher{}
	svc := newSvc(store, &fakePayLinks{rec: createdRecord()}, &fakeChain{}, pub)

	p, err := svc.Initiate(context.Background(), domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"})
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	if p.Status != lifecycle.StateAwaitingPayment || p.PayLinkID != plID || p.Rail != "mpesa" {
		t.Fatalf("unexpected payment: %+v", p)
	}
	if len(pub.events) != 1 || pub.events[0] != domain.EventPaymentInitiated {
		t.Fatalf("expected payment.initiated event, got %v", pub.events)
	}
}

func TestInitiateValidation(t *testing.T) {
	svc := newSvc(memory.New(), &fakePayLinks{rec: createdRecord()}, &fakeChain{}, &capturePublisher{})
	ctx := context.Background()

	if _, err := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: "0xshort", Rail: "mpesa"}); mustAppErr(t, err).Code != httpx.CodeInvalidPayload {
		t.Error("short hash should be INVALID_PAYLOAD")
	}
	if _, err := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "paypal"}); mustAppErr(t, err).Code != httpx.CodeInvalidPayload {
		t.Error("bad rail should be INVALID_PAYLOAD")
	}
}

func TestInitiatePayLinkNotFound(t *testing.T) {
	svc := newSvc(memory.New(), &fakePayLinks{rec: nil}, &fakeChain{}, &capturePublisher{})
	_, err := svc.Initiate(context.Background(), domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"})
	if mustAppErr(t, err).Code != httpx.CodePayLinkNotFound {
		t.Fatalf("want PAYLINK_NOT_FOUND, got %v", err)
	}
}

func TestInitiateNotPayable(t *testing.T) {
	rec := &domain.PayLinkRecord{ID: plID, Status: "VERIFIED", Expiry: clock.Add(time.Hour)}
	svc := newSvc(memory.New(), &fakePayLinks{rec: rec}, &fakeChain{}, &capturePublisher{})
	_, err := svc.Initiate(context.Background(), domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"})
	if mustAppErr(t, err).Code != httpx.CodePayLinkNotPayable {
		t.Fatalf("want PAYLINK_NOT_PAYABLE, got %v", err)
	}
}

func TestInitiateExpired(t *testing.T) {
	rec := &domain.PayLinkRecord{ID: plID, Status: "CREATED", Expiry: clock.Add(-time.Minute)}
	svc := newSvc(memory.New(), &fakePayLinks{rec: rec}, &fakeChain{}, &capturePublisher{})
	_, err := svc.Initiate(context.Background(), domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"})
	if mustAppErr(t, err).Code != httpx.CodePayLinkExpired {
		t.Fatalf("want PAYLINK_EXPIRED, got %v", err)
	}
}

func TestInitiatePayLinkServiceError(t *testing.T) {
	svc := newSvc(memory.New(), &fakePayLinks{err: httpx.NewError(httpx.CodePayLinkSvcUnavail, "down", nil)}, &fakeChain{}, &capturePublisher{})
	_, err := svc.Initiate(context.Background(), domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"})
	if mustAppErr(t, err).Code != httpx.CodePayLinkSvcUnavail {
		t.Fatalf("want PAYLINK_SERVICE_UNAVAILABLE, got %v", err)
	}
}

func TestInitiateDuplicate(t *testing.T) {
	store := memory.New()
	svc := newSvc(store, &fakePayLinks{rec: createdRecord()}, &fakeChain{}, &capturePublisher{})
	ctx := context.Background()
	if _, err := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "card"})
	ae := mustAppErr(t, err)
	if ae.Code != httpx.CodePaymentExists {
		t.Fatalf("want PAYMENT_EXISTS, got %v", ae.Code)
	}
	if ae.Details["payment_id"] == nil {
		t.Error("PAYMENT_EXISTS should include existing payment_id")
	}
}

func TestGetNotFound(t *testing.T) {
	svc := newSvc(memory.New(), &fakePayLinks{}, &fakeChain{}, &capturePublisher{})
	_, err := svc.Get(context.Background(), "missing")
	if mustAppErr(t, err).Code != httpx.CodePaymentNotFound {
		t.Fatalf("want PAYMENT_NOT_FOUND, got %v", err)
	}
}

func TestGetReconcilesToSettled(t *testing.T) {
	store := memory.New()
	pub := &capturePublisher{}
	ch := &fakeChain{status: "VERIFIED", found: true}
	svc := newSvc(store, &fakePayLinks{rec: createdRecord()}, ch, pub)
	ctx := context.Background()

	p, err := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != lifecycle.StateSettled {
		t.Fatalf("Get should reconcile to SETTLED, got %v", got.Status)
	}
	if !contains(pub.events, domain.EventPaymentSettled) {
		t.Fatalf("expected payment.settled published, got %v", pub.events)
	}
}

func TestGetChainUnavailableDegradesGracefully(t *testing.T) {
	store := memory.New()
	ch := &fakeChain{err: httpx.NewError(httpx.CodeChainUnavailable, "boom", nil)}
	svc := newSvc(store, &fakePayLinks{rec: createdRecord()}, ch, &capturePublisher{})
	ctx := context.Background()
	p, _ := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"})

	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get should not fail on chain hiccup: %v", err)
	}
	if got.Status != lifecycle.StateAwaitingPayment {
		t.Fatalf("status should remain AWAITING_PAYMENT, got %v", got.Status)
	}
}

func TestGetTerminalSkipsChain(t *testing.T) {
	store := memory.New()
	ch := &fakeChain{status: "VERIFIED", found: true}
	svc := newSvc(store, &fakePayLinks{rec: createdRecord()}, ch, &capturePublisher{})
	ctx := context.Background()
	p, _ := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"})

	// settle it
	if _, _, err := svc.ApplyChainEvent(ctx, domain.ChainEventInput{PayLinkID: plID, Seq: 1, ChainStatus: "VERIFIED", Kind: "paylink.verified"}); err != nil {
		t.Fatal(err)
	}
	callsBefore := ch.calls
	if _, err := svc.Get(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if ch.calls != callsBefore {
		t.Error("Get on a terminal payment must not call the chain")
	}
}

func TestApplyChainEventIdempotent(t *testing.T) {
	store := memory.New()
	pub := &capturePublisher{}
	svc := newSvc(store, &fakePayLinks{rec: createdRecord()}, &fakeChain{}, pub)
	ctx := context.Background()
	if _, err := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"}); err != nil {
		t.Fatal(err)
	}

	in := domain.ChainEventInput{PayLinkID: plID, Seq: 7, ChainStatus: "VERIFIED", Kind: "paylink.verified", TxHash: "0xtx"}
	_, changed, err := svc.ApplyChainEvent(ctx, in)
	if err != nil || !changed {
		t.Fatalf("first apply: changed=%v err=%v", changed, err)
	}
	// duplicate (same seq) -> no double advance
	_, changed, err = svc.ApplyChainEvent(ctx, in)
	if err != nil || changed {
		t.Fatalf("duplicate apply must be no-op: changed=%v err=%v", changed, err)
	}
	settledCount := 0
	for _, e := range pub.events {
		if e == domain.EventPaymentSettled {
			settledCount++
		}
	}
	if settledCount != 1 {
		t.Fatalf("payment.settled should be published exactly once, got %d", settledCount)
	}
}

func TestApplyChainEventNotOrchestrating(t *testing.T) {
	svc := newSvc(memory.New(), &fakePayLinks{}, &fakeChain{}, &capturePublisher{})
	_, changed, err := svc.ApplyChainEvent(context.Background(), domain.ChainEventInput{PayLinkID: plID, Seq: 1, ChainStatus: "VERIFIED"})
	if changed || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("event for un-orchestrated paylink should return ErrNotFound: changed=%v err=%v", changed, err)
	}
}

func TestApplyChainEventIllegalSwallowed(t *testing.T) {
	store := memory.New()
	svc := newSvc(store, &fakePayLinks{rec: createdRecord()}, &fakeChain{}, &capturePublisher{})
	ctx := context.Background()
	if _, err := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ApplyChainEvent(ctx, domain.ChainEventInput{PayLinkID: plID, Seq: 1, ChainStatus: "VERIFIED"}); err != nil {
		t.Fatal(err)
	}
	// cancel after settled: illegal, must be swallowed (no error, no change)
	_, changed, err := svc.ApplyChainEvent(ctx, domain.ChainEventInput{PayLinkID: plID, Seq: 2, ChainStatus: "CANCELLED"})
	if changed || err != nil {
		t.Fatalf("illegal transition should be a silent no-op: changed=%v err=%v", changed, err)
	}
}

func TestReady(t *testing.T) {
	svc := newSvc(memory.New(), &fakePayLinks{}, &fakeChain{}, &capturePublisher{})
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("Ready: %v", err)
	}
}

type fakeRecorder struct{ transitions [][2]string }

func (f *fakeRecorder) Transition(from, to string) {
	f.transitions = append(f.transitions, [2]string{from, to})
}

func TestApplyChainEventCancelAndFail(t *testing.T) {
	cases := []struct {
		chainStatus string
		wantState   lifecycle.State
		wantEvent   string
	}{
		{"CANCELLED", lifecycle.StateCancelled, domain.EventPaymentCancelled},
		{"FAILED", lifecycle.StateFailed, domain.EventPaymentFailed},
	}
	for _, c := range cases {
		t.Run(c.chainStatus, func(t *testing.T) {
			store := memory.New()
			pub := &capturePublisher{}
			svc := newSvc(store, &fakePayLinks{rec: createdRecord()}, &fakeChain{}, pub)
			ctx := context.Background()
			if _, err := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"}); err != nil {
				t.Fatal(err)
			}
			_, changed, err := svc.ApplyChainEvent(ctx, domain.ChainEventInput{PayLinkID: plID, Seq: 1, ChainStatus: c.chainStatus, Kind: "paylink." + strings.ToLower(c.chainStatus)})
			if err != nil || !changed {
				t.Fatalf("apply %s: changed=%v err=%v", c.chainStatus, changed, err)
			}
			if !contains(pub.events, c.wantEvent) {
				t.Fatalf("expected %s published, got %v", c.wantEvent, pub.events)
			}
		})
	}
}

func TestWithMetricsRecordsTransition(t *testing.T) {
	store := memory.New()
	rec := &fakeRecorder{}
	svc := domain.NewService(store, &fakePayLinks{rec: createdRecord()}, &fakeChain{}, &capturePublisher{}, nil,
		domain.WithClock(func() time.Time { return clock }),
		domain.WithIDGen(func() string { return "pay-x" }),
		domain.WithMetrics(rec),
	)
	ctx := context.Background()
	if _, err := svc.Initiate(ctx, domain.InitiateInput{PayLinkID: plID, Rail: "mpesa"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ApplyChainEvent(ctx, domain.ChainEventInput{PayLinkID: plID, Seq: 1, ChainStatus: "VERIFIED"}); err != nil {
		t.Fatal(err)
	}
	if len(rec.transitions) != 1 || rec.transitions[0] != [2]string{"AWAITING_PAYMENT", "SETTLED"} {
		t.Fatalf("expected one AWAITING_PAYMENT->SETTLED transition, got %v", rec.transitions)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
