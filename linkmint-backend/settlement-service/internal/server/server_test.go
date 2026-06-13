package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	idempotency "github.com/paylink/idempotency-go"

	"github.com/paylink/settlement-service/internal/domain"
	"github.com/paylink/settlement-service/internal/metrics"
	"github.com/paylink/settlement-service/internal/server"
	"github.com/paylink/settlement-service/internal/store/memory"
)

const payee = "0x00000000000000000000000000000000000000aa"

// memRedis is a minimal in-memory idempotency.RedisLike.
type memRedis struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemRedis() *memRedis { return &memRedis{m: map[string]string{}} }

func (r *memRedis) SetNX(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[key]; ok {
		return false, nil
	}
	r.m[key] = value
	return true, nil
}
func (r *memRedis) Set(_ context.Context, key, value string, _ time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[key] = value
	return nil
}
func (r *memRedis) Get(_ context.Context, key string) (string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.m[key]
	return v, ok, nil
}
func (r *memRedis) Del(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.m, key)
	return nil
}
func (r *memRedis) Ping(context.Context) error { return nil }

// newTestServer builds a server over a memory store with the clock pinned to the future and a high
// minimum payout (so the scheduler closes settlements without auto-instructing a payout, leaving
// room for the manual POST /v1/payouts path).
func newTestServer(t *testing.T, ingestToken string) (*server.Server, *domain.Service) {
	t.Helper()
	st := memory.New()
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	svc := domain.NewService(st, nil, nil,
		domain.WithCurrency("KES"), domain.WithTimezone("UTC"),
		domain.WithClock(func() time.Time { return now }),
		domain.WithMinPayout(func(string) *big.Int { return big.NewInt(1_000_000_000) }),
		domain.WithDefaultRail("mpesa"),
	)
	idem := idempotency.New(newMemRedis(), "settlement-service", time.Hour)
	srv := server.New(svc, idem, metrics.New(), nil, nil, "", ingestToken)
	return srv, svc
}

func do(srv *server.Server, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, r)
	return rr
}

func TestHealthAndReady(t *testing.T) {
	srv, _ := newTestServer(t, "")
	if rr := do(srv, http.MethodGet, "/internal/healthz", "", nil); rr.Code != http.StatusOK {
		t.Fatalf("healthz=%d", rr.Code)
	}
	if rr := do(srv, http.MethodGet, "/internal/readyz", "", nil); rr.Code != http.StatusOK {
		t.Fatalf("readyz=%d", rr.Code)
	}
}

func TestListRequiresCaller(t *testing.T) {
	srv, _ := newTestServer(t, "")
	rr := do(srv, http.MethodGet, "/v1/settlements", "", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", rr.Code)
	}
}

func TestSettlementReadsAndScoping(t *testing.T) {
	srv, svc := newTestServer(t, "")
	ctx := context.Background()
	day := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	if _, err := svc.HandleVerified(ctx, domain.VerifiedEvent{PLID: "PLK_A", Payee: payee, Amount: big.NewInt(1500), OccurredAt: day}); err != nil {
		t.Fatal(err)
	}

	rr := do(srv, http.MethodGet, "/v1/settlements", "", map[string]string{"X-Creator-Addr": payee})
	if rr.Code != http.StatusOK {
		t.Fatalf("list=%d", rr.Code)
	}
	var list struct {
		Items []struct {
			ID    string `json:"id"`
			Gross string `json:"gross"`
		} `json:"items"`
	}
	mustJSON(t, rr, &list)
	if len(list.Items) != 1 || list.Items[0].Gross != "1500" {
		t.Fatalf("list = %+v", list.Items)
	}
	id := list.Items[0].ID

	// Owner can read; a different caller gets 404 (no leak).
	if rr := do(srv, http.MethodGet, "/v1/settlements/"+id, "", map[string]string{"X-Creator-Addr": payee}); rr.Code != http.StatusOK {
		t.Fatalf("get own=%d", rr.Code)
	}
	if rr := do(srv, http.MethodGet, "/v1/settlements/"+id, "", map[string]string{"X-Creator-Addr": "0xother"}); rr.Code != http.StatusNotFound {
		t.Fatalf("get other=%d, want 404", rr.Code)
	}
}

func TestCreatePayoutIdempotentAndIngest(t *testing.T) {
	srv, svc := newTestServer(t, "secret")
	ctx := context.Background()
	day := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, domain.VerifiedEvent{PLID: "PLK_A", Payee: payee, Amount: big.NewInt(1500), OccurredAt: day})
	svc.Schedule(ctx) // closes the settlement (min payout too high to auto-instruct)

	// Find the CLOSED settlement id.
	rr := do(srv, http.MethodGet, "/v1/settlements?status=CLOSED", "", map[string]string{"X-Creator-Addr": payee})
	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	mustJSON(t, rr, &list)
	if len(list.Items) != 1 {
		t.Fatalf("closed settlements = %d", len(list.Items))
	}
	sid := list.Items[0].ID

	hdr := map[string]string{"X-Creator-Addr": payee, "Idempotency-Key": "k1", "Content-Type": "application/json"}
	rr1 := do(srv, http.MethodPost, "/v1/payouts", `{"settlement_id":"`+sid+`"}`, hdr)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("create payout=%d body=%s", rr1.Code, rr1.Body.String())
	}
	var p struct{ ID, Reference, Status string }
	mustJSON(t, rr1, &p)
	if p.Status != domain.PayoutInstructed {
		t.Fatalf("payout status=%s", p.Status)
	}

	// Idempotent replay returns the same payout id.
	rr2 := do(srv, http.MethodPost, "/v1/payouts", `{"settlement_id":"`+sid+`"}`, hdr)
	var p2 struct{ ID string }
	mustJSON(t, rr2, &p2)
	if rr2.Code != http.StatusCreated || p2.ID != p.ID {
		t.Fatalf("replay code=%d id=%s want %s", rr2.Code, p2.ID, p.ID)
	}

	// Ingest without the internal token → 403.
	file := `{"rail":"mpesa","lines":[{"reference":"` + p.Reference + `","amount":"1500","currency":"KES"}]}`
	if rr := do(srv, http.MethodPost, "/settlements/files/ingest", file, map[string]string{"Content-Type": "application/json"}); rr.Code != http.StatusForbidden {
		t.Fatalf("ingest without token=%d, want 403", rr.Code)
	}
	// With the token → matches the payout.
	rr3 := do(srv, http.MethodPost, "/settlements/files/ingest", file,
		map[string]string{"X-Internal-Token": "secret", "X-File-Id": "f1"})
	if rr3.Code != http.StatusOK {
		t.Fatalf("ingest=%d body=%s", rr3.Code, rr3.Body.String())
	}
	var res struct{ Matched, Unmatched int }
	mustJSON(t, rr3, &res)
	if res.Matched != 1 || res.Unmatched != 0 {
		t.Fatalf("ingest result matched=%d unmatched=%d", res.Matched, res.Unmatched)
	}
}

func TestCreatePayoutRequiresIdempotencyKey(t *testing.T) {
	srv, _ := newTestServer(t, "")
	rr := do(srv, http.MethodPost, "/v1/payouts", `{"settlement_id":"x"}`,
		map[string]string{"X-Creator-Addr": payee, "Content-Type": "application/json"})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing idem key=%d, want 400", rr.Code)
	}
}

func mustJSON(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(v); err != nil {
		t.Fatalf("decode body %q: %v", rr.Body.String(), err)
	}
}
