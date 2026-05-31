package broadcast_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paylink/mpesa-adapter/internal/broadcast"
	"github.com/paylink/mpesa-adapter/internal/httpx"
	"github.com/paylink/mpesa-adapter/internal/proof"
)

func sampleProof() proof.Proof {
	return proof.Proof{
		PayLinkID: "0x" + strings.Repeat("ab", 32),
		Rail:      "mpesa", TxID: "MP-1", Amount: 1500, Timestamp: 1730000000,
		Sender: "254700000000", Receiver: "174379", Signature: "c2ln",
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

func newClient(url string) *broadcast.Client {
	return broadcast.NewClient(url, &http.Client{Timeout: 2 * time.Second})
}

func TestBroadcast_Accepted(t *testing.T) {
	var gotIdem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdem = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"proof_hash":"0xph","tx_hash":"0xtx","status":"broadcast"}`))
	}))
	defer srv.Close()

	res, err := newClient(srv.URL).Broadcast(context.Background(), sampleProof(), "mpesa:MP-1")
	if err != nil {
		t.Fatalf("Broadcast: %v", err)
	}
	if res.Status != "broadcast" || res.ProofHash != "0xph" || res.TxHash != "0xtx" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if gotIdem != "mpesa:MP-1" {
		t.Fatalf("Idempotency-Key = %q, want mpesa:MP-1", gotIdem)
	}
}

func TestBroadcast_AlreadySettled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"proof_hash":"0xph","status":"already_settled"}`))
	}))
	defer srv.Close()

	res, err := newClient(srv.URL).Broadcast(context.Background(), sampleProof(), "k")
	if err != nil {
		t.Fatalf("Broadcast: %v", err)
	}
	if res.Status != "already_settled" {
		t.Fatalf("status = %q, want already_settled", res.Status)
	}
}

func TestBroadcast_RejectedByValidator(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"INVALID_PROOF_SIGNATURE","message":"nope","details":{}}}`))
	}))
	defer srv.Close()

	_, err := newClient(srv.URL).Broadcast(context.Background(), sampleProof(), "k")
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if c := codeOf(t, err); c != httpx.CodeProofRejected {
		t.Fatalf("code = %s, want PROOF_REJECTED", c)
	}
}

func TestBroadcast_ValidatorUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := newClient(srv.URL).Broadcast(context.Background(), sampleProof(), "k")
	if err == nil {
		t.Fatal("expected error for 502")
	}
	if c := codeOf(t, err); c != httpx.CodeValidatorUnavailable {
		t.Fatalf("code = %s, want VALIDATOR_UNAVAILABLE", c)
	}
}
