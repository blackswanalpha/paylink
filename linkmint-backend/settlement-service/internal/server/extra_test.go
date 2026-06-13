package server_test

import (
	"context"
	"errors"
	"math/big"
	"net/http"
	"testing"
	"time"

	idempotency "github.com/paylink/idempotency-go"

	"github.com/paylink/settlement-service/internal/domain"
	"github.com/paylink/settlement-service/internal/metrics"
	"github.com/paylink/settlement-service/internal/server"
	"github.com/paylink/settlement-service/internal/store/memory"
)

func TestReadyzFailure(t *testing.T) {
	st := memory.New()
	svc := domain.NewService(st, nil, nil)
	idem := idempotency.New(newMemRedis(), "settlement-service", time.Hour)
	ready := []server.ReadyCheck{{Name: "db", Check: func(context.Context) error { return errors.New("down") }}}
	srv := server.New(svc, idem, metrics.New(), nil, ready, "", "")

	rr := do(srv, http.MethodGet, "/internal/readyz", "", nil)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz=%d, want 503", rr.Code)
	}
}

func TestPayoutReadsAndNotFound(t *testing.T) {
	srv, svc := newTestServer(t, "")
	ctx := context.Background()
	day := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, domain.VerifiedEvent{PLID: "PLK_A", Payee: payee, Amount: big.NewInt(1500), OccurredAt: day})

	// Empty payout list (none scheduled yet).
	rr := do(srv, http.MethodGet, "/v1/payouts", "", map[string]string{"X-Creator-Addr": payee})
	if rr.Code != http.StatusOK {
		t.Fatalf("list payouts=%d", rr.Code)
	}
	// Missing payout / settlement → 404.
	if rr := do(srv, http.MethodGet, "/v1/payouts/nope", "", map[string]string{"X-Creator-Addr": payee}); rr.Code != http.StatusNotFound {
		t.Fatalf("get missing payout=%d, want 404", rr.Code)
	}
	if rr := do(srv, http.MethodGet, "/v1/settlements/nope", "", map[string]string{"X-Creator-Addr": payee}); rr.Code != http.StatusNotFound {
		t.Fatalf("get missing settlement=%d, want 404", rr.Code)
	}
}

func TestCreatePayoutUnknownSettlement(t *testing.T) {
	srv, _ := newTestServer(t, "")
	rr := do(srv, http.MethodPost, "/v1/payouts", `{"settlement_id":"nope"}`,
		map[string]string{"X-Creator-Addr": payee, "Idempotency-Key": "k1", "Content-Type": "application/json"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("create payout unknown settlement=%d, want 404", rr.Code)
	}
}

func TestCreatePayoutMissingSettlementID(t *testing.T) {
	srv, _ := newTestServer(t, "")
	rr := do(srv, http.MethodPost, "/v1/payouts", `{}`,
		map[string]string{"X-Creator-Addr": payee, "Idempotency-Key": "k1", "Content-Type": "application/json"})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing settlement_id=%d, want 400", rr.Code)
	}
}

func TestIngestBadFile(t *testing.T) {
	srv, _ := newTestServer(t, "")
	rr := do(srv, http.MethodPost, "/settlements/files/ingest", `not a file`, map[string]string{"X-Rail": "mpesa"})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad ingest file=%d, want 400", rr.Code)
	}
}
