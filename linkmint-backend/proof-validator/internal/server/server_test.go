package server_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/pkg/lvm"

	"github.com/paylink/proof-validator/internal/chain"
	"github.com/paylink/proof-validator/internal/domain"
	"github.com/paylink/proof-validator/internal/httpx"
	"github.com/paylink/proof-validator/internal/idempotency"
	"github.com/paylink/proof-validator/internal/metrics"
	"github.com/paylink/proof-validator/internal/proof"
	"github.com/paylink/proof-validator/internal/server"
	"github.com/paylink/proof-validator/internal/store/memory"
)

func httpxAppError() error { return httpx.NewError(httpx.CodeInvalidProofSignature, "bad sig", nil) }
func chainDownErr() error  { return httpx.NewError(httpx.CodeChainUnavailable, "chain down", nil) }

// ── fakes ──

type stubChain struct {
	used    bool
	usedErr error
	sendErr error
	sent    int
}

func (s *stubChain) IsProofUsed(context.Context, string) (bool, error) { return s.used, s.usedErr }
func (s *stubChain) GetPayLink(context.Context, string) (*chain.PayLinkState, bool, error) {
	return nil, false, nil // cross-check is disabled in these tests
}
func (s *stubChain) SendTransaction(context.Context, *lvm.Transaction) (string, error) {
	if s.sendErr != nil {
		return "", s.sendErr
	}
	s.sent++
	return "0xtx", nil
}

type stubVerifier struct{ err error }

func (v stubVerifier) Verify(proof.Proof) error { return v.err }

type stubSigner struct{}

func (stubSigner) Address() lvm.Address {
	return lvm.HexToAddress("0x00000000000000000000000000000000000000aa")
}
func (stubSigner) SignTx(tx *lvm.Transaction) error {
	tx.Hash = lvm.SHA256Hash(tx.SignableBytes())
	tx.Signature = []byte{1}
	return nil
}

type stubNonce struct{}

func (stubNonce) Reserve(context.Context, string) (uint64, func(bool), error) {
	return 0, func(bool) {}, nil
}

type fakeRedis struct {
	mu sync.Mutex
	m  map[string]string
}

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
func (f *fakeRedis) Ping(context.Context) error { return nil }

// ── harness ──

func newServer(t *testing.T, ch domain.ChainClient, v domain.ProofVerifier, ready ...server.ReadyCheck) http.Handler {
	t.Helper()
	svc := domain.NewService(memory.New(), ch, v, stubSigner{}, stubNonce{}, nil, nil, domain.WithCrossCheck(false))
	idem := idempotency.New(&fakeRedis{m: map[string]string{}}, time.Hour)
	if ready == nil {
		ready = []server.ReadyCheck{{Name: "ok", Check: func(context.Context) error { return nil }}}
	}
	return server.New(svc, idem, metrics.New(), nil, ready).Handler()
}

func proofBody(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"pl_id":           "0x" + strings.Repeat("ab", 32),
		"rail":            "mpesa",
		"tx_id":           "MP-1",
		"amount":          1500,
		"timestamp":       1730000000,
		"sender":          "254700000000",
		"receiver":        "254711111111",
		"proof_signature": base64.StdEncoding.EncodeToString(make([]byte, 64)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func post(t *testing.T, h http.Handler, body []byte, idemKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/proofs", bytes.NewReader(body))
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ── tests ──

func TestSubmitProof_Accepted(t *testing.T) {
	ch := &stubChain{}
	h := newServer(t, ch, stubVerifier{})
	rec := post(t, h, proofBody(t), "idem-1")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rec.Code, rec.Body)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("missing X-Request-Id")
	}
	var v map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v["status"] != "broadcast" || v["proof_hash"] == "" || v["tx_hash"] != "0xtx" {
		t.Fatalf("unexpected body: %v", v)
	}
	if ch.sent != 1 {
		t.Fatalf("expected 1 broadcast, got %d", ch.sent)
	}
}

func TestSubmitProof_MissingIdempotencyKey(t *testing.T) {
	rec := post(t, newServer(t, &stubChain{}, stubVerifier{}), proofBody(t), "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSubmitProof_ReplayIsCached(t *testing.T) {
	ch := &stubChain{}
	h := newServer(t, ch, stubVerifier{})
	body := proofBody(t)
	_ = post(t, h, body, "same-key")
	rec := post(t, h, body, "same-key")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("replay status = %d, want 202", rec.Code)
	}
	if ch.sent != 1 {
		t.Fatalf("replay must not re-broadcast; sent=%d", ch.sent)
	}
}

func TestSubmitProof_KeyConflict(t *testing.T) {
	h := newServer(t, &stubChain{}, stubVerifier{})
	_ = post(t, h, proofBody(t), "k")
	// Same key, different body.
	other := bytes.Replace(proofBody(t), []byte("MP-1"), []byte("MP-2"), 1)
	rec := post(t, h, other, "k")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestSubmitProof_BadShape(t *testing.T) {
	bad := bytes.Replace(proofBody(t), []byte("0x"+strings.Repeat("ab", 32)), []byte("nope"), 1)
	rec := post(t, newServer(t, &stubChain{}, stubVerifier{}), bad, "k")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSubmitProof_BadSignature(t *testing.T) {
	v := stubVerifier{err: httpxAppError()}
	rec := post(t, newServer(t, &stubChain{}, v), proofBody(t), "k")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", rec.Code, rec.Body)
	}
}

func TestSubmitProof_ChainUnavailable(t *testing.T) {
	ch := &stubChain{usedErr: chainDownErr()}
	rec := post(t, newServer(t, ch, stubVerifier{}), proofBody(t), "k")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestGetProof(t *testing.T) {
	ch := &stubChain{}
	h := newServer(t, ch, stubVerifier{})
	rec := post(t, h, proofBody(t), "k")
	var v map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	ph, _ := v["proof_hash"].(string)

	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/proofs/"+ph, nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200; body=%s", get.Code, get.Body)
	}

	miss := httptest.NewRecorder()
	h.ServeHTTP(miss, httptest.NewRequest(http.MethodGet, "/v1/proofs/0xunknown", nil))
	if miss.Code != http.StatusNotFound {
		t.Fatalf("GET unknown status = %d, want 404", miss.Code)
	}
}

func TestHealthReadyMetrics(t *testing.T) {
	okReady := server.ReadyCheck{Name: "dep", Check: func(context.Context) error { return nil }}
	h := newServer(t, &stubChain{}, stubVerifier{}, okReady)

	for path, want := range map[string]int{"/internal/healthz": 200, "/internal/readyz": 200, "/metrics": 200} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("%s = %d, want %d", path, rec.Code, want)
		}
	}

	failReady := server.ReadyCheck{Name: "dep", Check: func(context.Context) error { return chainDownErr() }}
	hf := newServer(t, &stubChain{}, stubVerifier{}, failReady)
	rec := httptest.NewRecorder()
	hf.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/internal/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz with a failing dep = %d, want 503", rec.Code)
	}
}
