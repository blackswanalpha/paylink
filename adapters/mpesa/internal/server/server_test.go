package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paylink/mpesa-adapter/internal/broadcast"
	"github.com/paylink/mpesa-adapter/internal/correlation"
	"github.com/paylink/mpesa-adapter/internal/daraja"
	"github.com/paylink/mpesa-adapter/internal/domain"
	"github.com/paylink/mpesa-adapter/internal/idempotency"
	"github.com/paylink/mpesa-adapter/internal/metrics"
	"github.com/paylink/mpesa-adapter/internal/proof"
	"github.com/paylink/mpesa-adapter/internal/server"
	"github.com/paylink/mpesa-adapter/internal/signer"
	"github.com/paylink/paylink-chain/pkg/lvm"
)

const (
	devnetKey        = "3f7a1c0d9e8b6a5f4d3c2b1a09f8e7d6c5b4a3928170615243f5e6d7c8b9a0f1"
	trustedPubKeyHex = "04e63cbe3984eae5834516e4af2e8e7fa88ce497f68bdcacf95a8fdaf9db4b02efa0ebcb964a1a74ec3d8c748b1e32986788f6c9a4aac39f0b79ac359801a5317d"
	internalToken    = "secret"
)

var testPL = "0x" + strings.Repeat("cd", 32)

// fakeRedis is an in-memory idempotency backend for tests.
type fakeRedis struct {
	mu sync.Mutex
	m  map[string]string
}

func newFakeRedis() *fakeRedis { return &fakeRedis{m: map[string]string{}} }

func (f *fakeRedis) SetNX(_ context.Context, k, v string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.m[k]; ok {
		return false, nil
	}
	f.m[k] = v
	return true, nil
}
func (f *fakeRedis) Set(_ context.Context, k, v string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[k] = v
	return nil
}
func (f *fakeRedis) Get(_ context.Context, k string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.m[k]
	return v, ok, nil
}
func (f *fakeRedis) Del(_ context.Context, k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, k)
	return nil
}
func (f *fakeRedis) Ping(_ context.Context) error { return nil }

// wireProof mirrors the JSON the adapter POSTs to the validator (so the stub can verify it).
type wireProof struct {
	PayLinkID string `json:"pl_id"`
	Rail      string `json:"rail"`
	TxID      string `json:"tx_id"`
	Amount    uint64 `json:"amount"`
	Timestamp int64  `json:"timestamp"`
	Sender    string `json:"sender"`
	Receiver  string `json:"receiver"`
	Signature string `json:"proof_signature"`
}

// stubValidator stands in for the proof-validator: it VERIFIES the received proof's signature
// against the trusted pubkey (the real off-chain trust check) and records what it saw.
func stubValidator(t *testing.T, gotProof *wireProof, gotIdem *string) *httptest.Server {
	t.Helper()
	pub, err := lvm.PublicKeyFromHex(trustedPubKeyHex)
	if err != nil {
		t.Fatalf("parse trusted pubkey: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotIdem = r.Header.Get("Idempotency-Key")
		var wp wireProof
		_ = json.NewDecoder(r.Body).Decode(&wp)
		*gotProof = wp
		p := proof.Proof{
			PayLinkID: wp.PayLinkID, Rail: wp.Rail, TxID: wp.TxID, Amount: wp.Amount,
			Timestamp: wp.Timestamp, Sender: wp.Sender, Receiver: wp.Receiver, Signature: wp.Signature,
		}
		if !proof.Verify(p, pub) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"INVALID_PROOF_SIGNATURE","message":"bad","details":{}}}`))
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"proof_hash":"0xph","tx_hash":"0xtx","status":"broadcast"}`))
	}))
}

func newServer(t *testing.T, validatorURL string, rail domain.RailClient, corr correlation.Store) *server.Server {
	t.Helper()
	sg, _, err := signer.Load(devnetKey)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	bcast := broadcast.NewClient(validatorURL, &http.Client{Timeout: 2 * time.Second})
	svc := domain.NewService(rail, corr, sg, bcast, "174379", nil)
	idem := idempotency.New(newFakeRedis(), time.Hour)
	return server.New(svc, idem, metrics.New(), nil, internalToken, nil)
}

func do(t *testing.T, h http.Handler, method, path, idemKey, token, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	if token != "" {
		req.Header.Set("X-Internal-Token", token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestCharges_Success_AndIdempotentReplay(t *testing.T) {
	var gp wireProof
	var gi string
	v := stubValidator(t, &gp, &gi)
	defer v.Close()
	rail := &daraja.FakeClient{Result: daraja.STKPushResult{CheckoutRequestID: "ws_CO_7"}}
	srv := newServer(t, v.URL, rail, correlation.NewMemory())
	h := srv.Handler()

	body := `{"pl_id":"` + testPL + `","amount":1500,"payer_phone":"254700000000"}`
	rec, out := do(t, h, http.MethodPost, "/v1/charges", "idem-1", "", body)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%v", rec.Code, out)
	}
	if out["checkout_request_id"] != "ws_CO_7" {
		t.Fatalf("checkout_request_id = %v", out["checkout_request_id"])
	}
	// Replay with the same key+body: cached response, rail NOT called again.
	rec2, _ := do(t, h, http.MethodPost, "/v1/charges", "idem-1", "", body)
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("replay status = %d", rec2.Code)
	}
	if len(rail.Calls) != 1 {
		t.Fatalf("rail STKPush calls = %d, want 1 (idempotent)", len(rail.Calls))
	}
}

func TestCharges_MissingIdempotencyKey(t *testing.T) {
	v := stubValidator(t, &wireProof{}, new(string))
	defer v.Close()
	srv := newServer(t, v.URL, &daraja.FakeClient{}, correlation.NewMemory())
	rec, out := do(t, srv.Handler(), http.MethodPost, "/v1/charges", "", "", `{"pl_id":"`+testPL+`","amount":1,"payer_phone":"254"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; %v", rec.Code, out)
	}
}

// TestCallback_EndToEnd is the DoD "captured callback → settle" path on the Go side: a rail-neutral
// callback (as the Node rail service forwards it) flows receive→normalize→sign→broadcast, and the
// stub validator confirms the proof's signature verifies against the trusted key.
func TestCallback_EndToEnd_SignsAndBroadcasts(t *testing.T) {
	var gp wireProof
	var gi string
	v := stubValidator(t, &gp, &gi)
	defer v.Close()

	corr := correlation.NewMemory()
	_ = corr.Put(context.Background(), "ws_CO_8", correlation.Record{PayLinkID: testPL, Amount: 1500, Receiver: "174379", PayerPhone: "254700000000"})
	srv := newServer(t, v.URL, &daraja.FakeClient{}, corr)

	cb := `{"checkout_request_id":"ws_CO_8","result_code":0,"amount":1500,"mpesa_receipt_number":"NLJ7RT61SV","phone_number":"254700000000"}`
	rec, out := do(t, srv.Handler(), http.MethodPost, "/v1/callbacks/mpesa", "", internalToken, cb)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%v", rec.Code, out)
	}
	if out["status"] != "broadcast" {
		t.Fatalf("status = %v, want broadcast", out["status"])
	}
	// The validator received a signature-valid, correctly-normalized proof.
	if gp.Rail != "mpesa" || gp.PayLinkID != testPL || gp.Amount != 1500 || gp.TxID != "NLJ7RT61SV" ||
		gp.Sender != "254700000000" || gp.Receiver != "174379" {
		t.Fatalf("validator saw wrong proof: %+v", gp)
	}
	if gi != "mpesa:NLJ7RT61SV" {
		t.Fatalf("Idempotency-Key = %q", gi)
	}
}

func TestCallback_Unauthorized(t *testing.T) {
	v := stubValidator(t, &wireProof{}, new(string))
	defer v.Close()
	srv := newServer(t, v.URL, &daraja.FakeClient{}, correlation.NewMemory())
	cb := `{"checkout_request_id":"ws_CO_x","result_code":0,"amount":1,"mpesa_receipt_number":"R","phone_number":"254"}`
	rec, _ := do(t, srv.Handler(), http.MethodPost, "/v1/callbacks/mpesa", "", "wrong-token", cb)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	v := stubValidator(t, &wireProof{}, new(string))
	defer v.Close()
	srv := newServer(t, v.URL, &daraja.FakeClient{}, correlation.NewMemory())
	rec, out := do(t, srv.Handler(), http.MethodGet, "/internal/healthz", "", "", "")
	if rec.Code != http.StatusOK || out["status"] != "ok" {
		t.Fatalf("healthz = %d %v", rec.Code, out)
	}
}
