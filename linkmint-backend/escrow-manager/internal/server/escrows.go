package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	idempotency "github.com/paylink/idempotency-go"

	"github.com/paylink/escrow-manager/internal/domain"
	"github.com/paylink/escrow-manager/internal/httpx"
)

// idemError maps an idempotency-library error to this service's HTTP envelope: a conflict becomes
// 409 IDEMPOTENT_CONFLICT, any other (backend) error a 500. The library is transport-free, so the
// status mapping lives here at the service boundary.
func idemError(err error) error {
	if errors.Is(err, idempotency.ErrConflict) {
		return httpx.NewError(httpx.CodeIdempotentConflict, err.Error(), nil)
	}
	return httpx.NewError(httpx.CodeInternalError, err.Error(), nil)
}

const (
	maxBodyBytes      = 1 << 20 // 1 MiB
	idemHeader        = "Idempotency-Key"
	creatorAddrHeader = "X-Creator-Addr"
	createRoute       = "create"
)

// callerAddr resolves the caller's on-chain address: the gateway-injected X-Creator-Addr
// (ADR-006 — the gateway strips any client-supplied value and injects the authenticated one),
// falling back to ESCROW_DEV_CREATOR_ADDR for local dev. "" means unauthenticated → 401.
func (s *Server) callerAddr(r *http.Request) string {
	if v := r.Header.Get(creatorAddrHeader); v != "" {
		return v
	}
	return s.devCreatorAddr
}

// requireCaller writes a 401 envelope and returns "" when no caller identity is present.
func (s *Server) requireCaller(w http.ResponseWriter, r *http.Request) string {
	addr := s.callerAddr(r)
	if addr == "" {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeUnauthorized,
			"missing caller identity (X-Creator-Addr)", nil))
	}
	return addr
}

// createRequest is the POST /v1/escrows body. pl_id is an opaque PayLink reference — the escrow
// is rail-unaware and holds no funds (A.1/A.4).
type createRequest struct {
	PLID            string           `json:"pl_id"`
	PayeeAddr       string           `json:"payee_addr"`
	RefundTo        string           `json:"refund_to"`
	Amount          string           `json:"amount"`
	Currency        string           `json:"currency"`
	ConditionType   string           `json:"condition_type"`
	ConditionParams *conditionParams `json:"condition_params,omitempty"`
	TimeoutAt       *time.Time       `json:"timeout_at,omitempty"`
}

type conditionParams struct {
	ReleaseAt *time.Time `json:"release_at,omitempty"`
	Approvers []string   `json:"approvers,omitempty"`
	Threshold int        `json:"threshold,omitempty"`
}

// disputeRequest is the POST /v1/escrows/{id}/dispute body.
type disputeRequest struct {
	Reason string `json:"reason"`
}

// escrowView is the API representation of an escrow.
type escrowView struct {
	ID              string              `json:"id"`
	PLID            string              `json:"pl_id"`
	CreatorAddr     string              `json:"creator_addr"`
	PayeeAddr       string              `json:"payee_addr"`
	RefundTo        string              `json:"refund_to"`
	Amount          string              `json:"amount"`
	Currency        string              `json:"currency"`
	ConditionType   string              `json:"condition_type"`
	ConditionParams conditionParamsView `json:"condition_params"`
	State           string              `json:"state"`
	Funded          bool                `json:"funded"`
	FundedTxHash    string              `json:"funded_tx_hash,omitempty"`
	ReleaseAt       string              `json:"release_at,omitempty"`
	TimeoutAt       string              `json:"timeout_at"`
	DisputeReason   string              `json:"dispute_reason,omitempty"`
	CreatedAt       string              `json:"created_at"`
	UpdatedAt       string              `json:"updated_at"`
}

type conditionParamsView struct {
	ReleaseAt string   `json:"release_at,omitempty"`
	Approvers []string `json:"approvers,omitempty"`
	Threshold int      `json:"threshold,omitempty"`
}

func toView(e domain.Escrow) escrowView {
	v := escrowView{
		ID:            e.ID,
		PLID:          e.PLID,
		CreatorAddr:   e.CreatorAddr,
		PayeeAddr:     e.PayeeAddr,
		RefundTo:      e.RefundTo,
		Amount:        e.Amount,
		Currency:      e.Currency,
		ConditionType: e.ConditionType,
		ConditionParams: conditionParamsView{
			Approvers: e.ConditionParams.Approvers,
			Threshold: e.ConditionParams.Threshold,
		},
		State:         string(e.State),
		Funded:        e.Funded,
		FundedTxHash:  e.FundedTxHash,
		TimeoutAt:     e.TimeoutAt.UTC().Format(time.RFC3339Nano),
		DisputeReason: e.DisputeReason,
		CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:     e.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if e.ConditionParams.ReleaseAt != nil {
		v.ConditionParams.ReleaseAt = e.ConditionParams.ReleaseAt.UTC().Format(time.RFC3339Nano)
	}
	if e.ReleaseAt != nil {
		v.ReleaseAt = e.ReleaseAt.UTC().Format(time.RFC3339Nano)
	}
	return v
}

// createEscrow handles POST /v1/escrows. It is idempotent on the Idempotency-Key header.
func (s *Server) createEscrow(w http.ResponseWriter, r *http.Request) {
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
	var req createRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "request body is not valid JSON: "+err.Error(), nil))
		return
	}

	// The caller identity joins the fingerprint so the same key+body from a DIFFERENT caller is
	// a conflict (409), never a cross-caller replay of someone else's escrow.
	fp := idempotency.Fingerprint(append(append([]byte{}, raw...), []byte("|"+caller)...))
	cached, err := s.idem.Begin(ctx, createRoute, idemKey, fp)
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

	in := domain.CreateInput{
		CreatorAddr:   caller,
		PLID:          req.PLID,
		PayeeAddr:     req.PayeeAddr,
		RefundTo:      req.RefundTo,
		Amount:        req.Amount,
		Currency:      req.Currency,
		ConditionType: req.ConditionType,
		TimeoutAt:     req.TimeoutAt,
	}
	if req.ConditionParams != nil {
		in.ConditionParams = domain.ConditionParams{
			ReleaseAt: req.ConditionParams.ReleaseAt,
			Approvers: req.ConditionParams.Approvers,
			Threshold: req.ConditionParams.Threshold,
		}
	}
	e, err := s.svc.Create(ctx, in)
	if err != nil {
		// Release the reservation so a corrected retry can reuse the key.
		s.idem.Release(ctx, createRoute, idemKey)
		httpx.WriteError(w, r, err)
		return
	}

	view := toView(e)
	body, _ := json.Marshal(view) // escrowView is statically marshalable — cannot fail
	if err := s.idem.Complete(ctx, createRoute, idemKey, fp, http.StatusCreated, body); err != nil {
		s.log.Warn("idempotency_complete_failed", "err", err.Error())
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

// getEscrow handles GET /v1/escrows/{id} — viewable by participants + approvers only;
// outsiders get the same 404 as a missing id.
func (s *Server) getEscrow(w http.ResponseWriter, r *http.Request) {
	caller := s.requireCaller(w, r)
	if caller == "" {
		return
	}
	id := chi.URLParam(r, "id")
	e, err := s.svc.Get(r.Context(), id, caller)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toView(e))
}

// listEscrows handles GET /v1/escrows?state=&limit= — the caller's escrows (creator-scoped).
func (s *Server) listEscrows(w http.ResponseWriter, r *http.Request) {
	caller := s.requireCaller(w, r)
	if caller == "" {
		return
	}
	escrows, err := s.svc.List(r.Context(), caller,
		r.URL.Query().Get("state"), parseLimit(r.URL.Query().Get("limit"), 20, 100))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	views := make([]escrowView, 0, len(escrows))
	for _, e := range escrows {
		views = append(views, toView(e))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": views})
}

// confirmEscrow handles POST /v1/escrows/{id}/confirm. The caller is the confirmation/approval
// identity; no body is required.
func (s *Server) confirmEscrow(w http.ResponseWriter, r *http.Request) {
	caller := s.requireCaller(w, r)
	if caller == "" {
		return
	}
	e, err := s.svc.Confirm(r.Context(), chi.URLParam(r, "id"), caller)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toView(e))
}

// disputeEscrow handles POST /v1/escrows/{id}/dispute.
func (s *Server) disputeEscrow(w http.ResponseWriter, r *http.Request) {
	caller := s.requireCaller(w, r)
	if caller == "" {
		return
	}
	var req disputeRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	e, err := s.svc.Dispute(r.Context(), chi.URLParam(r, "id"), caller, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toView(e))
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
