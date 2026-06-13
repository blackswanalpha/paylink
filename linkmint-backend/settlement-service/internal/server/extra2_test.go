package server_test

import (
	"context"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/paylink/settlement-service/internal/domain"
)

// seedClosed seeds one verified PayLink and closes its settlement (high min payout ⇒ no auto-payout).
func seedClosed(t *testing.T, svc *domain.Service) {
	t.Helper()
	ctx := context.Background()
	day := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, domain.VerifiedEvent{PLID: "PLK_A", Payee: payee, Amount: big.NewInt(1500), OccurredAt: day})
	svc.Schedule(ctx)
}

func TestCreatePayoutOnOpenSettlementIs409(t *testing.T) {
	srv, svc := newTestServer(t, "")
	ctx := context.Background()
	day := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	_, _ = svc.HandleVerified(ctx, domain.VerifiedEvent{PLID: "PLK_A", Payee: payee, Amount: big.NewInt(1500), OccurredAt: day})
	// Settlement is OPEN (no Schedule). Find its id.
	rr := do(srv, http.MethodGet, "/v1/settlements", "", map[string]string{"X-Creator-Addr": payee})
	var list struct {
		Items []struct{ ID string } `json:"items"`
	}
	mustJSON(t, rr, &list)
	sid := list.Items[0].ID

	rr = do(srv, http.MethodPost, "/v1/payouts", `{"settlement_id":"`+sid+`"}`,
		map[string]string{"X-Creator-Addr": payee, "Idempotency-Key": "k1", "Content-Type": "application/json"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("payout on OPEN settlement=%d, want 409", rr.Code)
	}
}

func TestPayoutListGetAndLimit(t *testing.T) {
	srv, svc := newTestServer(t, "")
	seedClosed(t, svc)
	rr := do(srv, http.MethodGet, "/v1/settlements?status=CLOSED", "", map[string]string{"X-Creator-Addr": payee})
	var sl struct {
		Items []struct{ ID string } `json:"items"`
	}
	mustJSON(t, rr, &sl)
	sid := sl.Items[0].ID

	hdr := map[string]string{"X-Creator-Addr": payee, "Idempotency-Key": "k1", "Content-Type": "application/json"}
	rr = do(srv, http.MethodPost, "/v1/payouts", `{"settlement_id":"`+sid+`"}`, hdr)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create payout=%d", rr.Code)
	}
	var p struct{ ID string }
	mustJSON(t, rr, &p)

	// List (with a clamped limit) and get-by-id.
	rr = do(srv, http.MethodGet, "/v1/payouts?limit=1", "", map[string]string{"X-Creator-Addr": payee})
	var pl struct {
		Items []struct {
			ID, Status string
		} `json:"items"`
	}
	mustJSON(t, rr, &pl)
	if len(pl.Items) != 1 || pl.Items[0].Status != domain.PayoutInstructed {
		t.Fatalf("payout list = %+v", pl.Items)
	}
	if rr := do(srv, http.MethodGet, "/v1/payouts/"+p.ID, "", map[string]string{"X-Creator-Addr": payee}); rr.Code != http.StatusOK {
		t.Fatalf("get payout=%d", rr.Code)
	}
}

func TestIdempotencyConflictDifferentBody(t *testing.T) {
	srv, svc := newTestServer(t, "")
	seedClosed(t, svc)
	rr := do(srv, http.MethodGet, "/v1/settlements?status=CLOSED", "", map[string]string{"X-Creator-Addr": payee})
	var sl struct {
		Items []struct{ ID string } `json:"items"`
	}
	mustJSON(t, rr, &sl)
	sid := sl.Items[0].ID

	hdr := map[string]string{"X-Creator-Addr": payee, "Idempotency-Key": "dup", "Content-Type": "application/json"}
	if rr := do(srv, http.MethodPost, "/v1/payouts", `{"settlement_id":"`+sid+`"}`, hdr); rr.Code != http.StatusCreated {
		t.Fatalf("first=%d", rr.Code)
	}
	// Same key, different body → fingerprint mismatch → 409.
	rr = do(srv, http.MethodPost, "/v1/payouts", `{"settlement_id":"different"}`, hdr)
	if rr.Code != http.StatusConflict {
		t.Fatalf("idempotency conflict=%d, want 409", rr.Code)
	}
}
