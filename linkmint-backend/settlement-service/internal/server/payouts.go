package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	idempotency "github.com/paylink/idempotency-go"

	"github.com/paylink/settlement-service/internal/domain"
	"github.com/paylink/settlement-service/internal/httpx"
)

const (
	maxBodyBytes    = 1 << 20 // 1 MiB
	idemHeader      = "Idempotency-Key"
	createPayoutRte = "create_payout"
)

// idemError maps an idempotency-library error to the HTTP envelope (conflict → 409, else 500).
func idemError(err error) error {
	if errors.Is(err, idempotency.ErrConflict) {
		return httpx.NewError(httpx.CodeIdempotentConflict, err.Error(), nil)
	}
	return httpx.NewError(httpx.CodeInternalError, err.Error(), nil)
}

// payoutView is the API representation of a payout.
type payoutView struct {
	ID           string `json:"id"`
	SettlementID string `json:"settlement_id"`
	MerchantKey  string `json:"merchant_key"`
	Rail         string `json:"rail"`
	Currency     string `json:"currency"`
	Amount       string `json:"amount"`
	Status       string `json:"status"`
	Reference    string `json:"reference"`
	ScheduledFor string `json:"scheduled_for"`
	InstructedAt string `json:"instructed_at,omitempty"`
	PaidAt       string `json:"paid_at,omitempty"`
}

func toPayoutView(p domain.Payout) payoutView {
	v := payoutView{
		ID: p.ID, SettlementID: p.SettlementID, MerchantKey: p.MerchantKey, Rail: p.Rail,
		Currency: p.Currency, Amount: p.Amount.String(), Status: p.Status, Reference: p.Reference,
		ScheduledFor: p.ScheduledFor.UTC().Format(time.RFC3339Nano),
	}
	if p.InstructedAt != nil {
		v.InstructedAt = p.InstructedAt.UTC().Format(time.RFC3339Nano)
	}
	if p.PaidAt != nil {
		v.PaidAt = p.PaidAt.UTC().Format(time.RFC3339Nano)
	}
	return v
}

// createPayoutRequest is the POST /v1/payouts body: pay out one CLOSED settlement on demand.
type createPayoutRequest struct {
	SettlementID string `json:"settlement_id"`
}

// listPayouts handles GET /v1/payouts?status=&limit= (caller/merchant-scoped).
func (s *Server) listPayouts(w http.ResponseWriter, r *http.Request) {
	caller := s.requireCaller(w, r)
	if caller == "" {
		return
	}
	out, err := s.svc.ListPayouts(r.Context(), caller,
		r.URL.Query().Get("status"), parseLimit(r.URL.Query().Get("limit"), 20, 100))
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	views := make([]payoutView, 0, len(out))
	for _, p := range out {
		views = append(views, toPayoutView(p))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": views})
}

// getPayout handles GET /v1/payouts/{id} (merchant-scoped — others get 404).
func (s *Server) getPayout(w http.ResponseWriter, r *http.Request) {
	caller := s.requireCaller(w, r)
	if caller == "" {
		return
	}
	p, err := s.svc.GetPayout(r.Context(), chi.URLParam(r, "id"), caller)
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPayoutView(p))
}

// createPayout handles POST /v1/payouts — instruct a payout for a CLOSED, caller-owned settlement.
// Idempotent on the Idempotency-Key header.
func (s *Server) createPayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	caller := s.requireCaller(w, r)
	if caller == "" {
		return
	}
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
	var req createPayoutRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "request body is not valid JSON: "+err.Error(), nil))
		return
	}
	if req.SettlementID == "" {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "settlement_id is required", nil))
		return
	}

	// The caller joins the fingerprint so the same key+body from a different caller is a conflict.
	fp := idempotency.Fingerprint(append(append([]byte{}, raw...), []byte("|"+caller)...))
	cached, err := s.idem.Begin(ctx, createPayoutRte, idemKey, fp)
	if err != nil {
		httpx.WriteError(w, r, idemError(err))
		return
	}
	if cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(cached.Status)
		_, _ = w.Write(cached.Body)
		return
	}

	p, err := s.svc.CreatePayout(ctx, req.SettlementID, caller)
	if err != nil {
		s.idem.Release(ctx, createPayoutRte, idemKey)
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	view := toPayoutView(p)
	body, _ := json.Marshal(view)
	if err := s.idem.Complete(ctx, createPayoutRte, idemKey, fp, http.StatusCreated, body); err != nil {
		s.log.Warn("idempotency_complete_failed", "err", err.Error())
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}
