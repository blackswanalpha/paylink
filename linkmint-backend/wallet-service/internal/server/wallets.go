package server

import (
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/paylink/wallet-service/internal/domain"
	"github.com/paylink/wallet-service/internal/httpx"
)

// walletView is the API representation of a wallet's balance + nonce.
type walletView struct {
	Addr        string `json:"addr"`
	Balance     string `json:"balance"`
	Nonce       uint64 `json:"nonce"`
	BlockHeight uint64 `json:"block_height"`
	FetchedAt   string `json:"fetched_at"`
	Stale       bool   `json:"stale"`
}

type transactionView struct {
	ID           string `json:"id"`
	Addr         string `json:"addr"`
	Counterparty string `json:"counterparty,omitempty"`
	Direction    string `json:"direction"`
	Kind         string `json:"kind"`
	Amount       string `json:"amount"`
	TxHash       string `json:"tx_hash,omitempty"`
	BlockHeight  uint64 `json:"block_height"`
	OccurredAt   string `json:"occurred_at"`
}

// getWallet handles GET /v1/wallets/{addr} (self-scoped).
func (s *Server) getWallet(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")
	if !s.requireSelf(w, r, addr) {
		return
	}
	acc, err := s.svc.GetWallet(r.Context(), addr)
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, walletView{
		Addr: acc.Addr, Balance: bigStr(acc.Balance), Nonce: acc.Nonce,
		BlockHeight: acc.BlockHeight, FetchedAt: rfc3339(acc.FetchedAt), Stale: acc.Stale,
	})
}

// listTransactions handles GET /v1/wallets/{addr}/transactions?limit=&cursor= (self-scoped).
func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")
	if !s.requireSelf(w, r, addr) {
		return
	}
	limit := parseLimit(r.URL.Query().Get("limit"))
	items, next, err := s.svc.ListTransactions(r.Context(), addr, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	views := make([]transactionView, 0, len(items))
	for _, t := range items {
		views = append(views, transactionView{
			ID: t.ID, Addr: t.Addr, Counterparty: t.Counterparty, Direction: t.Direction, Kind: t.Kind,
			Amount: bigStr(t.Amount), TxHash: t.TxHash, BlockHeight: t.BlockHeight, OccurredAt: rfc3339(t.OccurredAt),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": views, "next_cursor": next})
}

// ── shared helpers ──

func bigStr(b *big.Int) string {
	if b == nil {
		return "0"
	}
	return b.String()
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// parseLimit parses a positive limit query param (default/clamp handled by the domain).
func parseLimit(raw string) int {
	if raw == "" {
		return domain.DefaultPageLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return domain.DefaultPageLimit
	}
	return n
}
