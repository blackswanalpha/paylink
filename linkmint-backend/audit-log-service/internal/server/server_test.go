package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/paylink/audit-log-service/internal/auth"
	"github.com/paylink/audit-log-service/internal/domain"
	"github.com/paylink/audit-log-service/internal/events"
	"github.com/paylink/audit-log-service/internal/metrics"
	"github.com/paylink/audit-log-service/internal/store/memory"
	idempotency "github.com/paylink/idempotency-go"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

// fakeRedis is an in-memory idempotency.RedisLike.
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

func newTestServer(t *testing.T, secret string) *Server {
	t.Helper()
	store := memory.New()
	svc := domain.NewService(store, events.NewLogPublisher(nil), nil)
	idem := idempotency.New(newFakeRedis(), "audit-log-service", time.Hour)
	verifier, _ := auth.New("", "", "", nil) // disabled => gateway-trust
	return New(svc, idem, metrics.New(), verifier, secret, nil, []ReadyCheck{{Name: "store", Check: store.Ping}})
}

const actorObj = `{"id":"11111111-1111-1111-1111-111111111111","kind":"user"}`

func reqBody(actor, action, resource, ctx string) string {
	return `{"actor":` + actor + `,"action":"` + action + `","resource":"` + resource + `","context":` + ctx + `}`
}

func do(t *testing.T, s *Server, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func postEntryID(t *testing.T, rec *httptest.ResponseRecorder) int64 {
	t.Helper()
	if rec.Code != http.StatusCreated {
		t.Fatalf("post failed: code=%d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		EntryID int64  `json:"entry_id"`
		Hash    string `json:"hash"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Hash == "" {
		t.Fatal("missing hash in response")
	}
	return resp.EntryID
}

func TestHealthAndReady(t *testing.T) {
	s := newTestServer(t, "")
	if rec := do(t, s, "GET", "/internal/healthz", "", nil); rec.Code != 200 {
		t.Fatalf("healthz=%d", rec.Code)
	}
	if rec := do(t, s, "GET", "/internal/readyz", "", nil); rec.Code != 200 {
		t.Fatalf("readyz=%d", rec.Code)
	}
	if rec := do(t, s, "GET", "/metrics", "", nil); rec.Code != 200 {
		t.Fatalf("metrics=%d", rec.Code)
	}
}

func TestPostGetVerify(t *testing.T) {
	s := newTestServer(t, "")
	id := postEntryID(t, do(t, s, "POST", "/v1/audit-log", reqBody(actorObj, "admin.search", "user:x", `{"trace_id":"t1"}`), nil))
	if id != 1 {
		t.Fatalf("first entry id = %d", id)
	}

	rec := do(t, s, "GET", "/v1/audit-log/1", "", nil)
	if rec.Code != 200 {
		t.Fatalf("get code=%d", rec.Code)
	}
	var got struct {
		Entry map[string]any `json:"entry"`
		Proof struct {
			Valid     bool   `json:"valid"`
			ChainType string `json:"chain_type"`
		} `json:"proof"`
	}
	json.Unmarshal(rec.Body.Bytes(), &got)
	if !got.Proof.Valid || got.Proof.ChainType != "linear" {
		t.Fatalf("bad proof: %+v", got.Proof)
	}
	if got.Entry["action"] != "admin.search" {
		t.Fatalf("entry action=%v", got.Entry["action"])
	}

	rec = do(t, s, "GET", "/v1/audit-log/verify", "", nil)
	if rec.Code != 200 {
		t.Fatalf("verify code=%d", rec.Code)
	}
	var v struct {
		OK bool `json:"ok"`
	}
	json.Unmarshal(rec.Body.Bytes(), &v)
	if !v.OK {
		t.Fatalf("verify not ok: %s", rec.Body)
	}
}

func TestListFilter(t *testing.T) {
	s := newTestServer(t, "")
	do(t, s, "POST", "/v1/audit-log", reqBody(actorObj, "a1", "user:x", `{}`), nil)
	do(t, s, "POST", "/v1/audit-log", reqBody(actorObj, "a2", "user:y", `{}`), nil)

	rec := do(t, s, "GET", "/v1/audit-log?resource=user:y", "", nil)
	if rec.Code != 200 {
		t.Fatalf("list code=%d", rec.Code)
	}
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0]["resource"] != "user:y" {
		t.Fatalf("resource filter failed: %+v", resp.Items)
	}
}

func TestGetNotFound(t *testing.T) {
	s := newTestServer(t, "")
	rec := do(t, s, "GET", "/v1/audit-log/999", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
	assertErrorCode(t, rec, "ENTRY_NOT_FOUND")
}

func TestValidationRejected(t *testing.T) {
	s := newTestServer(t, "")
	// missing context
	rec := do(t, s, "POST", "/v1/audit-log", `{"actor":`+actorObj+`,"action":"a","resource":"r"}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec, "INVALID_PAYLOAD")

	// bad actor id
	rec = do(t, s, "POST", "/v1/audit-log", reqBody(`{"id":"not-a-uuid","kind":"user"}`, "a", "r", `{}`), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad uuid want 400, got %d", rec.Code)
	}

	// unknown field
	rec = do(t, s, "POST", "/v1/audit-log", `{"actor":`+actorObj+`,"action":"a","resource":"r","context":{},"x":1}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field want 400, got %d", rec.Code)
	}
}

func TestStringActorCompat(t *testing.T) {
	s := newTestServer(t, "")
	// bare-string actor (admin-backoffice sends the JWT sub) → kind=user, id parsed when a UUID
	rec := do(t, s, "POST", "/v1/audit-log", reqBody(`"11111111-1111-1111-1111-111111111111"`, "admin.search", "user:x", `{}`), nil)
	id := postEntryID(t, rec)
	rec = do(t, s, "GET", "/v1/audit-log/"+strconv.FormatInt(id, 10), "", nil)
	var got struct {
		Entry struct {
			Actor struct {
				ID   *string `json:"id"`
				Kind string  `json:"kind"`
			} `json:"actor"`
		} `json:"entry"`
	}
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Entry.Actor.Kind != "user" || got.Entry.Actor.ID == nil {
		t.Fatalf("string actor not mapped: %+v", got.Entry.Actor)
	}
}

func TestInternalGate(t *testing.T) {
	s := newTestServer(t, "s3cret")
	b := reqBody(actorObj, "a", "r", `{}`)
	if rec := do(t, s, "POST", "/v1/audit-log", b, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token want 401, got %d", rec.Code)
	}
	if rec := do(t, s, "POST", "/v1/audit-log", b, map[string]string{"X-Internal-Token": "wrong"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token want 401, got %d", rec.Code)
	}
	if rec := do(t, s, "POST", "/v1/audit-log", b, map[string]string{"X-Internal-Token": "s3cret"}); rec.Code != http.StatusCreated {
		t.Fatalf("correct token want 201, got %d", rec.Code)
	}
}

func TestIdempotency(t *testing.T) {
	s := newTestServer(t, "")
	b := reqBody(actorObj, "a", "r", `{"trace_id":"t"}`)
	h := map[string]string{"Idempotency-Key": "key-1"}

	id1 := postEntryID(t, do(t, s, "POST", "/v1/audit-log", b, h))
	id2 := postEntryID(t, do(t, s, "POST", "/v1/audit-log", b, h)) // replay
	if id1 != id2 {
		t.Fatalf("idempotent replay should return the same entry: %d vs %d", id1, id2)
	}
	// same key, different body → conflict
	rec := do(t, s, "POST", "/v1/audit-log", reqBody(actorObj, "different", "r", `{}`), h)
	if rec.Code != http.StatusConflict {
		t.Fatalf("key reuse with new body want 409, got %d", rec.Code)
	}
	assertErrorCode(t, rec, "IDEMPOTENT_CONFLICT")
}

func TestAbsentKeyAppendsEach(t *testing.T) {
	s := newTestServer(t, "")
	b := reqBody(actorObj, "a", "r", `{}`)
	id1 := postEntryID(t, do(t, s, "POST", "/v1/audit-log", b, nil))
	id2 := postEntryID(t, do(t, s, "POST", "/v1/audit-log", b, nil))
	if id1 == id2 {
		t.Fatal("without an idempotency key each post must append a new entry")
	}
}

func TestListActorFilterAndCursor(t *testing.T) {
	s := newTestServer(t, "")
	// two entries by the same actor, one by a different actor
	do(t, s, "POST", "/v1/audit-log", reqBody(actorObj, "a1", "user:x", `{}`), nil)
	do(t, s, "POST", "/v1/audit-log", reqBody(actorObj, "a2", "user:y", `{}`), nil)
	do(t, s, "POST", "/v1/audit-log", reqBody(`{"id":"22222222-2222-2222-2222-222222222222","kind":"user"}`, "a3", "user:z", `{}`), nil)

	rec := do(t, s, "GET", "/v1/audit-log?actor=11111111-1111-1111-1111-111111111111&limit=1", "", nil)
	if rec.Code != 200 {
		t.Fatalf("list code=%d", rec.Code)
	}
	var p struct {
		Items      []map[string]any `json:"items"`
		NextCursor *string          `json:"next_cursor"`
	}
	json.Unmarshal(rec.Body.Bytes(), &p)
	if len(p.Items) != 1 || p.NextCursor == nil {
		t.Fatalf("want 1 item + cursor, got %d cursor=%v", len(p.Items), p.NextCursor)
	}
	// follow the cursor — remaining entry for this actor
	rec = do(t, s, "GET", "/v1/audit-log?actor=11111111-1111-1111-1111-111111111111&cursor="+*p.NextCursor, "", nil)
	json.Unmarshal(rec.Body.Bytes(), &p)
	if len(p.Items) != 1 {
		t.Fatalf("cursor page want 1 item, got %d", len(p.Items))
	}
}

func TestVerifyWithTimeParams(t *testing.T) {
	s := newTestServer(t, "")
	do(t, s, "POST", "/v1/audit-log", reqBody(actorObj, "a", "r", `{}`), nil)
	rec := do(t, s, "GET", "/v1/audit-log/verify?from=2026-01-01T00:00:00Z&to=2030-01-01T00:00:00Z", "", nil)
	if rec.Code != 200 {
		t.Fatalf("verify code=%d", rec.Code)
	}
	var v struct {
		OK bool `json:"ok"`
	}
	json.Unmarshal(rec.Body.Bytes(), &v)
	if !v.OK {
		t.Fatalf("verify not ok: %s", rec.Body)
	}
}

func TestInvalidQueryParams(t *testing.T) {
	s := newTestServer(t, "")
	for _, path := range []string{
		"/v1/audit-log?actor=not-a-uuid",
		"/v1/audit-log?from=not-a-time",
		"/v1/audit-log?cursor=abc",
		"/v1/audit-log/verify?to=not-a-time",
		"/v1/audit-log/not-an-int",
	} {
		rec := do(t, s, "GET", path, "", nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s want 400, got %d", path, rec.Code)
		}
	}
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var env struct {
		Error struct {
			Code    string `json:"code"`
			TraceID string `json:"trace_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope decode: %v (body=%s)", err, rec.Body)
	}
	if env.Error.Code != want {
		t.Fatalf("error code = %q, want %q", env.Error.Code, want)
	}
	if env.Error.TraceID == "" {
		t.Fatal("envelope missing trace_id")
	}
}
