package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paylink/chain-event-mirror/internal/metrics"
)

func TestHealthz(t *testing.T) {
	srv := New(metrics.New(), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestReadyz_AllChecksPass(t *testing.T) {
	srv := New(metrics.New(), []ReadyCheck{{Name: "kafka", Check: func(context.Context) error { return nil }}})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/readyz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ready"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestReadyz_FailingCheck(t *testing.T) {
	srv := New(metrics.New(), []ReadyCheck{{Name: "kafka", Check: func(context.Context) error { return errors.New("down") }}})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"not_ready"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv := New(metrics.New(), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
}
