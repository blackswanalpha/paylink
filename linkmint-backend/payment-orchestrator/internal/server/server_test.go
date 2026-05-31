package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/events"
	"github.com/paylink/payment-orchestrator/internal/idempotency"
	"github.com/paylink/payment-orchestrator/internal/lifecycle"
	"github.com/paylink/payment-orchestrator/internal/metrics"
	"github.com/paylink/payment-orchestrator/internal/server"
	"github.com/paylink/payment-orchestrator/internal/store/memory"
)

const plID = "0x" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// ---- stubs ----

type stubPayLinks struct {
	rec *domain.PayLinkRecord
	err error
}

func (s stubPayLinks) GetPayLink(context.Context, string) (*domain.PayLinkRecord, error) {
	return s.rec, s.err
}

type stubChain struct {
	status string
	found  bool
	err    error
}

func (s stubChain) PayLinkStatus(context.Context, string) (string, bool, error) {
	return s.status, s.found, s.err
}

type memRedis struct {
	mu   sync.Mutex
	data map[string]string
}

func newMemRedis() *memRedis { return &memRedis{data: map[string]string{}} }

func (m *memRedis) SetNX(_ context.Context, k, v string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[k]; ok {
		return false, nil
	}
	m.data[k] = v
	return true, nil
}
func (m *memRedis) Set(_ context.Context, k, v string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[k] = v
	return nil
}
func (m *memRedis) Get(_ context.Context, k string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[k]
	return v, ok, nil
}
func (m *memRedis) Del(_ context.Context, k string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, k)
	return nil
}
func (m *memRedis) Ping(context.Context) error { return nil }

// ---- helpers ----

func build(t *testing.T, pl domain.PayLinkLookup, ch domain.ChainReader, ready ...server.ReadyCheck) (*server.Server, *memory.Store) {
	t.Helper()
	store := memory.New()
	svc := domain.NewService(store, pl, ch, events.NewLogPublisher(nil), nil)
	idem := idempotency.New(newMemRedis(), time.Hour)
	if ready == nil {
		ready = []server.ReadyCheck{{Name: "store", Check: store.Ping}}
	}
	return server.New(svc, idem, metrics.New(), nil, ready), store
}

func payableStub() stubPayLinks {
	return stubPayLinks{rec: &domain.PayLinkRecord{ID: plID, Status: "CREATED", Expiry: time.Now().Add(time.Hour)}}
}

func do(srv *server.Server, method, path, body string, headers map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr, out
}

func idemHdr(k string) map[string]string { return map[string]string{"Idempotency-Key": k} }

// ---- tests ----

func TestInitiateHappy(t *testing.T) {
	srv, _ := build(t, payableStub(), stubChain{})
	rr, out := do(srv, http.MethodPost, "/v1/payments", `{"paylink_id":"`+plID+`","rail":"mpesa"}`, idemHdr("k1"))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body)
	}
	if out["status"] != string(lifecycle.StateAwaitingPayment) || out["id"] == "" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestInitiateMissingIdempotencyKey(t *testing.T) {
	srv, _ := build(t, payableStub(), stubChain{})
	rr, out := do(srv, http.MethodPost, "/v1/payments", `{"paylink_id":"`+plID+`","rail":"mpesa"}`, nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	if code := errCode(out); code != "INVALID_PAYLOAD" {
		t.Fatalf("code = %s", code)
	}
}

func TestInitiateBadJSON(t *testing.T) {
	srv, _ := build(t, payableStub(), stubChain{})
	rr, _ := do(srv, http.MethodPost, "/v1/payments", `{not json`, idemHdr("k1"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestInitiateIdempotentReplay(t *testing.T) {
	srv, _ := build(t, payableStub(), stubChain{})
	body := `{"paylink_id":"` + plID + `","rail":"mpesa"}`
	rr1, out1 := do(srv, http.MethodPost, "/v1/payments", body, idemHdr("k1"))
	rr2, out2 := do(srv, http.MethodPost, "/v1/payments", body, idemHdr("k1"))
	if rr1.Code != http.StatusCreated || rr2.Code != http.StatusCreated {
		t.Fatalf("codes = %d,%d", rr1.Code, rr2.Code)
	}
	if out1["id"] != out2["id"] || out1["id"] == nil {
		t.Fatalf("replay must return identical payment: %v vs %v", out1["id"], out2["id"])
	}
}

func TestInitiateIdempotentConflict(t *testing.T) {
	srv, _ := build(t, payableStub(), stubChain{})
	do(srv, http.MethodPost, "/v1/payments", `{"paylink_id":"`+plID+`","rail":"mpesa"}`, idemHdr("k1"))
	rr, out := do(srv, http.MethodPost, "/v1/payments", `{"paylink_id":"`+plID+`","rail":"card"}`, idemHdr("k1"))
	if rr.Code != http.StatusConflict || errCode(out) != "IDEMPOTENT_CONFLICT" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestInitiatePayLinkNotFound(t *testing.T) {
	srv, _ := build(t, stubPayLinks{rec: nil}, stubChain{})
	rr, out := do(srv, http.MethodPost, "/v1/payments", `{"paylink_id":"`+plID+`","rail":"mpesa"}`, idemHdr("k1"))
	if rr.Code != http.StatusNotFound || errCode(out) != "PAYLINK_NOT_FOUND" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestInitiateBadRail(t *testing.T) {
	srv, _ := build(t, payableStub(), stubChain{})
	rr, out := do(srv, http.MethodPost, "/v1/payments", `{"paylink_id":"`+plID+`","rail":"paypal"}`, idemHdr("k1"))
	if rr.Code != http.StatusBadRequest || errCode(out) != "INVALID_PAYLOAD" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
	// failed initiate releases the key — a corrected retry with the same key succeeds.
	rr2, _ := do(srv, http.MethodPost, "/v1/payments", `{"paylink_id":"`+plID+`","rail":"mpesa"}`, idemHdr("k1"))
	if rr2.Code != http.StatusCreated {
		t.Fatalf("retry after release should succeed, got %d", rr2.Code)
	}
}

func TestGetHappyAndReconcile(t *testing.T) {
	// chain reports VERIFIED → GET reconciles AWAITING → SETTLED.
	srv, _ := build(t, payableStub(), stubChain{status: "VERIFIED", found: true})
	_, created := do(srv, http.MethodPost, "/v1/payments", `{"paylink_id":"`+plID+`","rail":"mpesa"}`, idemHdr("k1"))
	id, _ := created["id"].(string)

	rr, out := do(srv, http.MethodGet, "/v1/payments/"+id, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if out["status"] != string(lifecycle.StateSettled) {
		t.Fatalf("GET should reconcile to SETTLED, got %v", out["status"])
	}
}

func TestGetNotFound(t *testing.T) {
	srv, _ := build(t, payableStub(), stubChain{})
	rr, out := do(srv, http.MethodGet, "/v1/payments/missing", "", nil)
	if rr.Code != http.StatusNotFound || errCode(out) != "PAYMENT_NOT_FOUND" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestHealthAndReady(t *testing.T) {
	srv, _ := build(t, payableStub(), stubChain{})
	if rr, _ := do(srv, http.MethodGet, "/internal/healthz", "", nil); rr.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rr.Code)
	}
	if rr, _ := do(srv, http.MethodGet, "/internal/readyz", "", nil); rr.Code != http.StatusOK {
		t.Fatalf("readyz = %d", rr.Code)
	}
}

func TestReadyFailure(t *testing.T) {
	failing := server.ReadyCheck{Name: "chain", Check: func(context.Context) error { return errors.New("down") }}
	srv, _ := build(t, payableStub(), stubChain{}, failing)
	rr, out := do(srv, http.MethodGet, "/internal/readyz", "", nil)
	if rr.Code != http.StatusServiceUnavailable || errCode(out) != "SERVICE_NOT_READY" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv, _ := build(t, payableStub(), stubChain{})
	do(srv, http.MethodGet, "/internal/healthz", "", nil) // generate a metric
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("metrics = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "http_requests_total") {
		t.Fatalf("metrics body missing http_requests_total")
	}
}

func errCode(out map[string]any) string {
	e, ok := out["error"].(map[string]any)
	if !ok {
		return ""
	}
	c, _ := e["code"].(string)
	return c
}
