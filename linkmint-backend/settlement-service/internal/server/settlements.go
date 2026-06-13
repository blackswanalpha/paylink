package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/paylink/settlement-service/internal/domain"
	"github.com/paylink/settlement-service/internal/httpx"
)

// mapErr maps a domain sentinel error to the HTTP envelope (unknown → opaque 500 via httpx).
func mapErr(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return httpx.NewError(httpx.CodeSettlementNotFound, "not found", nil)
	case errors.Is(err, domain.ErrInvalidState):
		return httpx.NewError(httpx.CodeInvalidState, err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidAmount):
		return httpx.NewError(httpx.CodeInvalidPayload, err.Error(), nil)
	default:
		return err
	}
}

// settlementView is the API representation of a settlement.
type settlementView struct {
	ID             string     `json:"id"`
	MerchantKey    string     `json:"merchant_key"`
	Currency       string     `json:"currency"`
	SettlementDate string     `json:"settlement_date"`
	Status         string     `json:"status"`
	Gross          string     `json:"gross"`
	PlatformFee    string     `json:"platform_fee"`
	ChainFee       string     `json:"chain_fee"`
	Net            string     `json:"net"`
	CutoffAt       string     `json:"cutoff_at"`
	OpenedAt       string     `json:"opened_at"`
	ClosedAt       string     `json:"closed_at,omitempty"`
	Items          []itemView `json:"items,omitempty"`
}

type itemView struct {
	ID             string `json:"id"`
	PLID           string `json:"pl_id"`
	Kind           string `json:"kind"`
	Gross          string `json:"gross"`
	PlatformFee    string `json:"platform_fee"`
	ChainFee       string `json:"chain_fee"`
	Net            string `json:"net"`
	VerifiedTxHash string `json:"verified_tx_hash,omitempty"`
	CreatedAt      string `json:"created_at"`
}

func toSettlementView(s domain.Settlement, items []domain.SettlementItem) settlementView {
	v := settlementView{
		ID: s.ID, MerchantKey: s.MerchantKey, Currency: s.Currency, SettlementDate: s.SettlementDate,
		Status: s.Status, Gross: s.Gross.String(), PlatformFee: s.PlatformFee.String(),
		ChainFee: s.ChainFee.String(), Net: s.Net.String(),
		CutoffAt: s.CutoffAt.UTC().Format(time.RFC3339Nano), OpenedAt: s.OpenedAt.UTC().Format(time.RFC3339Nano),
	}
	if s.ClosedAt != nil {
		v.ClosedAt = s.ClosedAt.UTC().Format(time.RFC3339Nano)
	}
	for _, it := range items {
		v.Items = append(v.Items, itemView{
			ID: it.ID, PLID: it.PLID, Kind: it.Kind, Gross: it.Gross.String(),
			PlatformFee: it.PlatformFee.String(), ChainFee: it.ChainFee.String(), Net: it.Net.String(),
			VerifiedTxHash: it.VerifiedTxHash, CreatedAt: it.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return v
}

// listSettlements handles GET /v1/settlements?status=&limit= (caller/merchant-scoped).
func (s *Server) listSettlements(w http.ResponseWriter, r *http.Request) {
	caller := s.requireCaller(w, r)
	if caller == "" {
		return
	}
	out, err := s.svc.ListSettlements(r.Context(), caller,
		r.URL.Query().Get("status"), parseLimit(r.URL.Query().Get("limit"), 20, 100))
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	views := make([]settlementView, 0, len(out))
	for _, st := range out {
		views = append(views, toSettlementView(st, nil))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": views})
}

// getSettlement handles GET /v1/settlements/{id} (with items; merchant-scoped — others get 404).
func (s *Server) getSettlement(w http.ResponseWriter, r *http.Request) {
	caller := s.requireCaller(w, r)
	if caller == "" {
		return
	}
	st, items, err := s.svc.GetSettlement(r.Context(), chi.URLParam(r, "id"), caller)
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSettlementView(st, items))
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
