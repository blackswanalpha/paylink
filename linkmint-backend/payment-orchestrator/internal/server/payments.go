package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/httpx"
	"github.com/paylink/payment-orchestrator/internal/idempotency"
)

const (
	maxBodyBytes  = 1 << 20 // 1 MiB
	idemHeader    = "Idempotency-Key"
	initiateRoute = "initiate"
)

// initiateRequest is the POST /v1/payments body. Rail is an opaque routing label — no
// rail-specific fields cross the boundary (invariant A.4).
type initiateRequest struct {
	PayLinkID string `json:"paylink_id"`
	Rail      string `json:"rail"`
}

// paymentView is the API representation of a payment.
type paymentView struct {
	ID        string `json:"id"`
	PayLinkID string `json:"paylink_id"`
	Rail      string `json:"rail"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toView(p domain.Payment) paymentView {
	return paymentView{
		ID:        p.ID,
		PayLinkID: p.PayLinkID,
		Rail:      p.Rail,
		Status:    string(p.Status),
		CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: p.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

// initiatePayment handles POST /v1/payments. It is idempotent on the Idempotency-Key header.
func (s *Server) initiatePayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idemKey := r.Header.Get(idemHeader)
	if idemKey == "" {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "Idempotency-Key header is required", nil))
		return
	}

	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "could not read request body", nil))
		return
	}
	var req initiateRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "request body is not valid JSON: "+err.Error(), nil))
		return
	}

	fp := idempotency.Fingerprint(raw)
	cached, err := s.idem.Begin(ctx, initiateRoute, idemKey, fp)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(cached.Status)
		_, _ = w.Write(cached.Body)
		return
	}

	p, err := s.svc.Initiate(ctx, domain.InitiateInput{PayLinkID: req.PayLinkID, Rail: req.Rail})
	if err != nil {
		// Release the reservation so a corrected retry can reuse the key.
		s.idem.Release(ctx, initiateRoute, idemKey)
		httpx.WriteError(w, r, err)
		return
	}

	view := toView(p)
	body, _ := json.Marshal(view) // paymentView is statically marshalable (only strings) — cannot fail
	if err := s.idem.Complete(ctx, initiateRoute, idemKey, fp, http.StatusCreated, body); err != nil {
		s.log.Warn("idempotency_complete_failed", "err", err.Error())
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

// getPayment handles GET /v1/payments/{id} (and the internal-admin drill-down). The response is
// reconciled against on-chain truth.
func (s *Server) getPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := s.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toView(p))
}

// adminSearchPayments handles GET /internal/admin/payments?q=&limit= — the read-only admin lookup
// (admin-backoffice, work11). It matches an exact payment id / paylink id / status; no reconcile.
func (s *Server) adminSearchPayments(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "q query parameter is required", nil))
		return
	}
	payments, err := s.svc.Search(r.Context(), q, parseLimit(r.URL.Query().Get("limit"), 20, 100))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	views := make([]paymentView, 0, len(payments))
	for _, p := range payments {
		views = append(views, toView(p))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": views})
}

// parseLimit parses a positive limit query param, clamped to max, falling back to def.
func parseLimit(raw string, def, max int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
