package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/paylink/mpesa-adapter/internal/broadcast"
	"github.com/paylink/mpesa-adapter/internal/correlation"
	"github.com/paylink/mpesa-adapter/internal/daraja"
	"github.com/paylink/mpesa-adapter/internal/domain"
	"github.com/paylink/mpesa-adapter/internal/httpx"
	"github.com/paylink/mpesa-adapter/internal/proof"
)

var plID = "0x" + strings.Repeat("ab", 32)

type fakeSigner struct{}

func (fakeSigner) Sign(proof.Proof) (string, error) { return "c2ln", nil }

type fakeBroadcaster struct {
	last     proof.Proof
	lastIdem string
	calls    int
	res      broadcast.Result
	err      error
}

func (f *fakeBroadcaster) Broadcast(_ context.Context, p proof.Proof, idemKey string) (broadcast.Result, error) {
	f.calls++
	f.last = p
	f.lastIdem = idemKey
	return f.res, f.err
}

func newSvc(t *testing.T, rail domain.RailClient, corr correlation.Store, b domain.Broadcaster) *domain.Service {
	t.Helper()
	clock := func() time.Time { return time.Unix(1730000000, 0) }
	return domain.NewService(rail, corr, fakeSigner{}, b, "174379", nil, domain.WithClock(clock))
}

func successCallback(checkout string) daraja.CallbackResult {
	return daraja.CallbackResult{
		CheckoutRequestID:  checkout,
		ResultCode:         0,
		Amount:             1500,
		MpesaReceiptNumber: "NLJ7RT61SV",
		PhoneNumber:        "254700000000",
	}
}

func TestInitiateCharge_Success(t *testing.T) {
	rail := &daraja.FakeClient{Result: daraja.STKPushResult{CheckoutRequestID: "ws_CO_9", MerchantRequestID: "m9"}}
	corr := correlation.NewMemory()
	svc := newSvc(t, rail, corr, &fakeBroadcaster{})

	res, err := svc.InitiateCharge(context.Background(), domain.ChargeInput{PayLinkID: plID, Amount: 1500, PayerPhone: "254700000000"})
	if err != nil {
		t.Fatalf("InitiateCharge: %v", err)
	}
	if res.CheckoutRequestID != "ws_CO_9" || res.Status != "pending" {
		t.Fatalf("result = %+v", res)
	}
	// A.1: the STK push went to the default RECEIVER shortcode (no LinkMint account).
	if len(rail.Calls) != 1 || rail.Calls[0].ShortCode != "174379" || rail.Calls[0].Amount != 1500 {
		t.Fatalf("rail call = %+v", rail.Calls)
	}
	// Correlation recorded for the callback.
	rec, gerr := corr.Get(context.Background(), "ws_CO_9")
	if gerr != nil || rec.PayLinkID != plID || rec.Receiver != "174379" {
		t.Fatalf("correlation = %+v err=%v", rec, gerr)
	}
}

func TestInitiateCharge_InvalidPayLink(t *testing.T) {
	svc := newSvc(t, &daraja.FakeClient{}, correlation.NewMemory(), &fakeBroadcaster{})
	_, err := svc.InitiateCharge(context.Background(), domain.ChargeInput{PayLinkID: "0xnope", Amount: 1, PayerPhone: "254"})
	if c := codeOf(t, err); c != httpx.CodeInvalidPayload {
		t.Fatalf("code = %s, want INVALID_PAYLOAD", c)
	}
}

func TestInitiateCharge_RailError(t *testing.T) {
	rail := &daraja.FakeClient{Err: httpx.NewError(httpx.CodeDarajaUnavailable, "down", nil)}
	svc := newSvc(t, rail, correlation.NewMemory(), &fakeBroadcaster{})
	_, err := svc.InitiateCharge(context.Background(), domain.ChargeInput{PayLinkID: plID, Amount: 1500, PayerPhone: "254700000000"})
	if c := codeOf(t, err); c != httpx.CodeDarajaUnavailable {
		t.Fatalf("code = %s, want DARAJA_UNAVAILABLE", c)
	}
}

func TestHandleCallback_Success_NormalizesAndBroadcasts(t *testing.T) {
	corr := correlation.NewMemory()
	_ = corr.Put(context.Background(), "ws_CO_1", correlation.Record{PayLinkID: plID, Amount: 1500, Receiver: "174379", PayerPhone: "254700000000"})
	b := &fakeBroadcaster{res: broadcast.Result{ProofHash: "0xph", TxHash: "0xtx", Status: "broadcast"}}
	svc := newSvc(t, &daraja.FakeClient{}, corr, b)

	out, err := svc.HandleCallback(context.Background(), successCallback("ws_CO_1"))
	if err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	if out.Status != "broadcast" || out.ProofHash != "0xph" || out.TxHash != "0xtx" {
		t.Fatalf("outcome = %+v", out)
	}
	if b.calls != 1 {
		t.Fatalf("broadcast calls = %d, want 1", b.calls)
	}
	// A.4: exactly the normalized shape, rail=mpesa, receiver=shortcode, sender=phone, tx=receipt.
	p := b.last
	if p.Rail != "mpesa" || p.PayLinkID != plID || p.Amount != 1500 ||
		p.TxID != "NLJ7RT61SV" || p.Sender != "254700000000" || p.Receiver != "174379" {
		t.Fatalf("normalized proof leaked/wrong: %+v", p)
	}
	if b.lastIdem != "mpesa:NLJ7RT61SV" {
		t.Fatalf("idem key = %q, want mpesa:NLJ7RT61SV", b.lastIdem)
	}
}

func TestHandleCallback_NoCorrelation(t *testing.T) {
	b := &fakeBroadcaster{}
	svc := newSvc(t, &daraja.FakeClient{}, correlation.NewMemory(), b)
	out, err := svc.HandleCallback(context.Background(), successCallback("unknown"))
	if err != nil {
		t.Fatalf("err = %v, want nil (ack)", err)
	}
	if out.Status != "ignored_no_correlation" || b.calls != 0 {
		t.Fatalf("outcome = %+v calls=%d", out, b.calls)
	}
}

func TestHandleCallback_FailedPayment(t *testing.T) {
	corr := correlation.NewMemory()
	_ = corr.Put(context.Background(), "ws_CO_2", correlation.Record{PayLinkID: plID, Amount: 1500, Receiver: "174379"})
	b := &fakeBroadcaster{}
	svc := newSvc(t, &daraja.FakeClient{}, corr, b)

	cb := daraja.CallbackResult{CheckoutRequestID: "ws_CO_2", ResultCode: 1032, ResultDesc: "cancelled"}
	out, err := svc.HandleCallback(context.Background(), cb)
	if err != nil || out.Status != "ignored_failed_payment" || b.calls != 0 {
		t.Fatalf("outcome = %+v err=%v calls=%d", out, err, b.calls)
	}
}

func TestHandleCallback_AmountMismatch(t *testing.T) {
	corr := correlation.NewMemory()
	_ = corr.Put(context.Background(), "ws_CO_3", correlation.Record{PayLinkID: plID, Amount: 1500, Receiver: "174379"})
	b := &fakeBroadcaster{}
	svc := newSvc(t, &daraja.FakeClient{}, corr, b)

	cb := successCallback("ws_CO_3")
	cb.Amount = 999 // paid less than the PayLink requires
	out, err := svc.HandleCallback(context.Background(), cb)
	if err != nil || out.Status != "ignored_amount_mismatch" || b.calls != 0 {
		t.Fatalf("outcome = %+v err=%v calls=%d", out, err, b.calls)
	}
}

func TestHandleCallback_AlreadySettled(t *testing.T) {
	corr := correlation.NewMemory()
	_ = corr.Put(context.Background(), "ws_CO_4", correlation.Record{PayLinkID: plID, Amount: 1500, Receiver: "174379"})
	b := &fakeBroadcaster{res: broadcast.Result{ProofHash: "0xph", Status: "already_settled"}}
	svc := newSvc(t, &daraja.FakeClient{}, corr, b)

	out, err := svc.HandleCallback(context.Background(), successCallback("ws_CO_4"))
	if err != nil || out.Status != "already_settled" {
		t.Fatalf("outcome = %+v err=%v", out, err)
	}
}

func TestHandleCallback_ValidatorUnavailable_Retryable(t *testing.T) {
	corr := correlation.NewMemory()
	_ = corr.Put(context.Background(), "ws_CO_5", correlation.Record{PayLinkID: plID, Amount: 1500, Receiver: "174379"})
	b := &fakeBroadcaster{err: httpx.NewError(httpx.CodeValidatorUnavailable, "down", nil)}
	svc := newSvc(t, &daraja.FakeClient{}, corr, b)

	_, err := svc.HandleCallback(context.Background(), successCallback("ws_CO_5"))
	if c := codeOf(t, err); c != httpx.CodeValidatorUnavailable {
		t.Fatalf("want retryable VALIDATOR_UNAVAILABLE, got %v", err)
	}
}

func TestHandleCallback_Rejected_Terminal(t *testing.T) {
	corr := correlation.NewMemory()
	_ = corr.Put(context.Background(), "ws_CO_6", correlation.Record{PayLinkID: plID, Amount: 1500, Receiver: "174379"})
	b := &fakeBroadcaster{err: httpx.NewError(httpx.CodeProofRejected, "bad sig", nil)}
	svc := newSvc(t, &daraja.FakeClient{}, corr, b)

	out, err := svc.HandleCallback(context.Background(), successCallback("ws_CO_6"))
	if err != nil || out.Status != "rejected" {
		t.Fatalf("outcome = %+v err=%v (want rejected, nil)", out, err)
	}
}

func codeOf(t *testing.T, err error) httpx.ErrorCode {
	t.Helper()
	var ae *httpx.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("error %v is not an AppError", err)
	}
	return ae.Code
}

// guard against accidental rail-specific leakage in the wire body shape.
func TestProofWireHasNoRailFields(t *testing.T) {
	b, _ := proof.MarshalWire(proof.Proof{PayLinkID: plID, Rail: "mpesa", TxID: "x", Amount: 1, Timestamp: 1, Sender: "s", Receiver: "r", Signature: "z"})
	for _, leak := range []string{"CheckoutRequestID", "MerchantRequest", "stkCallback", "MpesaReceipt"} {
		if strings.Contains(string(b), leak) {
			t.Fatalf("wire body leaked rail field %q: %s", leak, b)
		}
	}
}
