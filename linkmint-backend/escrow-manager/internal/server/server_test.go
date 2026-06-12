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

	idempotency "github.com/paylink/idempotency-go"

	"github.com/paylink/escrow-manager/internal/config"
	"github.com/paylink/escrow-manager/internal/domain"
	"github.com/paylink/escrow-manager/internal/events"
	"github.com/paylink/escrow-manager/internal/metrics"
	"github.com/paylink/escrow-manager/internal/server"
	"github.com/paylink/escrow-manager/internal/store/memory"
)

const creator = "0xcreator"

// ---- stubs ----

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

func build(t *testing.T, devAddr string, ready ...server.ReadyCheck) (*server.Server, *domain.Service) {
	t.Helper()
	store := memory.New()
	svc := domain.NewService(store, events.NewLogPublisher(nil), nil)
	idem := idempotency.New(newMemRedis(), config.ServiceName, time.Hour)
	if ready == nil {
		ready = []server.ReadyCheck{{Name: "store", Check: store.Ping}}
	}
	return server.New(svc, idem, metrics.New(), nil, ready, devAddr), svc
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

func hdrs(idemKey string) map[string]string {
	h := map[string]string{"X-Creator-Addr": creator}
	if idemKey != "" {
		h["Idempotency-Key"] = idemKey
	}
	return h
}

func deliveryBody(pl string) string {
	return `{"pl_id":"` + pl + `","payee_addr":"0xpayee","refund_to":"0xrefund","amount":"1000","currency":"KES","condition_type":"delivery_confirmation"}`
}

func createEscrow(t *testing.T, srv *server.Server, pl string) string {
	t.Helper()
	rr, out := do(srv, http.MethodPost, "/v1/escrows", deliveryBody(pl), hdrs("k-"+pl))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rr.Code, rr.Body)
	}
	id, _ := out["id"].(string)
	return id
}

func errCode(out map[string]any) string {
	e, ok := out["error"].(map[string]any)
	if !ok {
		return ""
	}
	c, _ := e["code"].(string)
	return c
}

// ---- tests ----

func TestCreateHappy(t *testing.T) {
	srv, _ := build(t, "")
	rr, out := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_1"), hdrs("k1"))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body)
	}
	if out["state"] != "WAITING" || out["id"] == "" || out["funded"] != false {
		t.Fatalf("unexpected body %v", out)
	}
	if out["creator_addr"] != creator {
		t.Fatalf("creator_addr = %v", out["creator_addr"])
	}
}

func TestCreateMissingCreatorHeader(t *testing.T) {
	srv, _ := build(t, "") // no dev fallback ⇒ 401
	rr, out := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_2"), map[string]string{"Idempotency-Key": "k1"})
	if rr.Code != http.StatusUnauthorized || errCode(out) != "UNAUTHORIZED" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestCreateDevFallbackAddr(t *testing.T) {
	srv, _ := build(t, "0xdev")
	rr, out := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_3"), map[string]string{"Idempotency-Key": "k1"})
	if rr.Code != http.StatusCreated || out["creator_addr"] != "0xdev" {
		t.Fatalf("status=%d creator=%v", rr.Code, out["creator_addr"])
	}
}

func TestCreateMissingIdempotencyKey(t *testing.T) {
	srv, _ := build(t, "")
	rr, out := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_4"), map[string]string{"X-Creator-Addr": creator})
	if rr.Code != http.StatusBadRequest || errCode(out) != "INVALID_PAYLOAD" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestCreateBadJSON(t *testing.T) {
	srv, _ := build(t, "")
	rr, _ := do(srv, http.MethodPost, "/v1/escrows", `{not json`, hdrs("k1"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestCreateIdempotentReplay(t *testing.T) {
	srv, _ := build(t, "")
	rr1, out1 := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_5"), hdrs("k5"))
	rr2, out2 := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_5"), hdrs("k5"))
	if rr1.Code != http.StatusCreated || rr2.Code != http.StatusCreated {
		t.Fatalf("codes = %d,%d (%s)", rr1.Code, rr2.Code, rr2.Body)
	}
	if out1["id"] != out2["id"] || out1["id"] == nil {
		t.Fatalf("replay must return the identical escrow: %v vs %v", out1["id"], out2["id"])
	}
}

func TestCreateIdempotentConflict(t *testing.T) {
	srv, _ := build(t, "")
	do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_6"), hdrs("k6"))
	rr, out := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_6b"), hdrs("k6"))
	if rr.Code != http.StatusConflict || errCode(out) != "IDEMPOTENT_CONFLICT" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestCreateIdempotentConflictDifferentCaller(t *testing.T) {
	srv, _ := build(t, "")
	do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_6c"), hdrs("k6c"))
	other := map[string]string{"X-Creator-Addr": "0xother", "Idempotency-Key": "k6c"}
	rr, out := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_6c"), other)
	if rr.Code != http.StatusConflict || errCode(out) != "IDEMPOTENT_CONFLICT" {
		t.Fatalf("same key+body from another caller must conflict: status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestCreateDuplicatePayLink(t *testing.T) {
	srv, _ := build(t, "")
	do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_7"), hdrs("k7a"))
	rr, out := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_7"), hdrs("k7b"))
	if rr.Code != http.StatusConflict || errCode(out) != "ESCROW_EXISTS" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
	// A failed create releases the key — a corrected retry with the same key succeeds.
	rr2, _ := do(srv, http.MethodPost, "/v1/escrows", deliveryBody("PLK_7x"), hdrs("k7b"))
	if rr2.Code != http.StatusCreated {
		t.Fatalf("retry after release should succeed, got %d", rr2.Code)
	}
}

func TestCreateValidationError(t *testing.T) {
	srv, _ := build(t, "")
	body := `{"pl_id":"PLK_8","payee_addr":"0xp","refund_to":"0xr","amount":"-1","currency":"KES","condition_type":"delivery_confirmation"}`
	rr, out := do(srv, http.MethodPost, "/v1/escrows", body, hdrs("k8"))
	if rr.Code != http.StatusBadRequest || errCode(out) != "INVALID_PAYLOAD" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestGetEscrow(t *testing.T) {
	srv, _ := build(t, "")
	id := createEscrow(t, srv, "PLK_9")
	rr, out := do(srv, http.MethodGet, "/v1/escrows/"+id, "", hdrs(""))
	if rr.Code != http.StatusOK || out["id"] != id {
		t.Fatalf("status=%d body=%v", rr.Code, out)
	}
	rr, out = do(srv, http.MethodGet, "/v1/escrows/ESC_missing", "", hdrs(""))
	if rr.Code != http.StatusNotFound || errCode(out) != "ESCROW_NOT_FOUND" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
	// View-scoped: an outsider gets the same 404 as a missing id; no caller → 401.
	rr, out = do(srv, http.MethodGet, "/v1/escrows/"+id, "", map[string]string{"X-Creator-Addr": "0xother"})
	if rr.Code != http.StatusNotFound || errCode(out) != "ESCROW_NOT_FOUND" {
		t.Fatalf("outsider: status=%d code=%s", rr.Code, errCode(out))
	}
	rr, out = do(srv, http.MethodGet, "/v1/escrows/"+id, "", nil)
	if rr.Code != http.StatusUnauthorized || errCode(out) != "UNAUTHORIZED" {
		t.Fatalf("missing caller: status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestListEscrows(t *testing.T) {
	srv, _ := build(t, "")
	createEscrow(t, srv, "PLK_10")
	createEscrow(t, srv, "PLK_11")

	rr, out := do(srv, http.MethodGet, "/v1/escrows", "", hdrs(""))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if items, _ := out["items"].([]any); len(items) != 2 {
		t.Fatalf("items = %v", out["items"])
	}
	// State filter + limit clamp.
	rr, out = do(srv, http.MethodGet, "/v1/escrows?state=WAITING&limit=1", "", hdrs(""))
	if items, _ := out["items"].([]any); rr.Code != http.StatusOK || len(items) != 1 {
		t.Fatalf("filtered: status=%d items=%v", rr.Code, out["items"])
	}
	// Creator-scoped: another caller sees nothing.
	rr, out = do(srv, http.MethodGet, "/v1/escrows", "", map[string]string{"X-Creator-Addr": "0xother"})
	if items, _ := out["items"].([]any); rr.Code != http.StatusOK || len(items) != 0 {
		t.Fatalf("scope: status=%d items=%v", rr.Code, out["items"])
	}
	// Invalid state → 400; missing caller → 401.
	rr, out = do(srv, http.MethodGet, "/v1/escrows?state=BOGUS", "", hdrs(""))
	if rr.Code != http.StatusBadRequest || errCode(out) != "INVALID_PAYLOAD" {
		t.Fatalf("invalid state: status=%d code=%s", rr.Code, errCode(out))
	}
	rr, out = do(srv, http.MethodGet, "/v1/escrows", "", nil)
	if rr.Code != http.StatusUnauthorized || errCode(out) != "UNAUTHORIZED" {
		t.Fatalf("missing caller: status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestConfirmFlow(t *testing.T) {
	srv, svc := build(t, "")
	id := createEscrow(t, srv, "PLK_12")

	// Confirm before funding: 200, stays WAITING.
	rr, out := do(srv, http.MethodPost, "/v1/escrows/"+id+"/confirm", "", hdrs(""))
	if rr.Code != http.StatusOK || out["state"] != "WAITING" {
		t.Fatalf("status=%d state=%v", rr.Code, out["state"])
	}
	// Fund via the domain seam (what the bus consumer does) → releases (condition already met).
	if res, err := svc.HandlePaylinkVerified(context.Background(), "PLK_12", "0xtx"); err != nil || res != domain.ResultReleased {
		t.Fatalf("funding: res=%s err=%v", res, err)
	}
	rr, out = do(srv, http.MethodGet, "/v1/escrows/"+id, "", hdrs(""))
	if out["state"] != "RELEASED" || out["funded"] != true {
		t.Fatalf("after funding: %v", out)
	}
	// Confirm after release → 409 INVALID_STATE.
	rr, out = do(srv, http.MethodPost, "/v1/escrows/"+id+"/confirm", "", hdrs(""))
	if rr.Code != http.StatusConflict || errCode(out) != "INVALID_STATE" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestConfirmAuthz(t *testing.T) {
	srv, _ := build(t, "")
	id := createEscrow(t, srv, "PLK_13")

	rr, out := do(srv, http.MethodPost, "/v1/escrows/"+id+"/confirm", "", nil)
	if rr.Code != http.StatusUnauthorized || errCode(out) != "UNAUTHORIZED" {
		t.Fatalf("missing caller: status=%d code=%s", rr.Code, errCode(out))
	}
	rr, out = do(srv, http.MethodPost, "/v1/escrows/"+id+"/confirm", "", map[string]string{"X-Creator-Addr": "0xother"})
	if rr.Code != http.StatusForbidden || errCode(out) != "NOT_PARTICIPANT" {
		t.Fatalf("non-creator: status=%d code=%s", rr.Code, errCode(out))
	}
	rr, out = do(srv, http.MethodPost, "/v1/escrows/ESC_missing/confirm", "", hdrs(""))
	if rr.Code != http.StatusNotFound || errCode(out) != "ESCROW_NOT_FOUND" {
		t.Fatalf("missing escrow: status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestConfirmTimeLockRejected(t *testing.T) {
	srv, _ := build(t, "")
	release := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	body := `{"pl_id":"PLK_14","payee_addr":"0xp","refund_to":"0xr","amount":"5","currency":"KES",` +
		`"condition_type":"time_lock","condition_params":{"release_at":"` + release + `"}}`
	rr, out := do(srv, http.MethodPost, "/v1/escrows", body, hdrs("k14"))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body)
	}
	id, _ := out["id"].(string)
	rr, out = do(srv, http.MethodPost, "/v1/escrows/"+id+"/confirm", "", hdrs(""))
	if rr.Code != http.StatusConflict || errCode(out) != "CONDITION_NOT_CONFIRMABLE" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestDisputeFlow(t *testing.T) {
	srv, _ := build(t, "")
	id := createEscrow(t, srv, "PLK_15")

	rr, out := do(srv, http.MethodPost, "/v1/escrows/"+id+"/dispute", `{"reason":""}`, hdrs(""))
	if rr.Code != http.StatusBadRequest || errCode(out) != "INVALID_PAYLOAD" {
		t.Fatalf("empty reason: status=%d code=%s", rr.Code, errCode(out))
	}
	rr, out = do(srv, http.MethodPost, "/v1/escrows/"+id+"/dispute", `{"reason":"not delivered"}`, hdrs(""))
	if rr.Code != http.StatusOK || out["state"] != "DISPUTED" || out["dispute_reason"] != "not delivered" {
		t.Fatalf("dispute: status=%d body=%v", rr.Code, out)
	}
	// Second dispute → 409; non-participant → 403; missing caller → 401.
	rr, out = do(srv, http.MethodPost, "/v1/escrows/"+id+"/dispute", `{"reason":"again"}`, hdrs(""))
	if rr.Code != http.StatusConflict || errCode(out) != "INVALID_STATE" {
		t.Fatalf("re-dispute: status=%d code=%s", rr.Code, errCode(out))
	}
	rr, out = do(srv, http.MethodPost, "/v1/escrows/"+id+"/dispute", `{"reason":"x"}`, map[string]string{"X-Creator-Addr": "0xother"})
	if rr.Code != http.StatusForbidden || errCode(out) != "NOT_PARTICIPANT" {
		t.Fatalf("stranger: status=%d code=%s", rr.Code, errCode(out))
	}
	rr, out = do(srv, http.MethodPost, "/v1/escrows/"+id+"/dispute", `{"reason":"x"}`, nil)
	if rr.Code != http.StatusUnauthorized || errCode(out) != "UNAUTHORIZED" {
		t.Fatalf("missing caller: status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestHealthAndReady(t *testing.T) {
	srv, _ := build(t, "")
	if rr, _ := do(srv, http.MethodGet, "/internal/healthz", "", nil); rr.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rr.Code)
	}
	if rr, _ := do(srv, http.MethodGet, "/internal/readyz", "", nil); rr.Code != http.StatusOK {
		t.Fatalf("readyz = %d", rr.Code)
	}
}

func TestReadyFailure(t *testing.T) {
	failing := server.ReadyCheck{Name: "redis", Check: func(context.Context) error { return errors.New("down") }}
	srv, _ := build(t, "", failing)
	rr, out := do(srv, http.MethodGet, "/internal/readyz", "", nil)
	if rr.Code != http.StatusServiceUnavailable || errCode(out) != "SERVICE_NOT_READY" {
		t.Fatalf("status=%d code=%s", rr.Code, errCode(out))
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv, _ := build(t, "")
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
