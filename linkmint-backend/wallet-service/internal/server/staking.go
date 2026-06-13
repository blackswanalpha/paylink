package server

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"

	"github.com/paylink/wallet-service/internal/domain"
	"github.com/paylink/wallet-service/internal/httpx"
)

type positionView struct {
	Addr              string `json:"addr"`
	StakedAmount      string `json:"staked_amount"`
	PendingWithdrawal string `json:"pending_withdrawal"`
	TotalRewards      string `json:"total_rewards"`
	TotalSlashed      string `json:"total_slashed"`
	WithdrawableAt    string `json:"withdrawable_at,omitempty"`
	IsActive          bool   `json:"is_active"`
	UpdatedAt         string `json:"updated_at"`
}

type rewardView struct {
	ID           string `json:"id"`
	Addr         string `json:"addr"`
	Amount       string `json:"amount"`
	TotalRewards string `json:"total_rewards"`
	Source       string `json:"source"`
	TxHash       string `json:"tx_hash,omitempty"`
	BlockHeight  uint64 `json:"block_height"`
	OccurredAt   string `json:"occurred_at"`
}

type intentRequest struct {
	Addr   string `json:"addr"`
	Action string `json:"action"`
	Amount string `json:"amount"`
}

type feeEstimateView struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
	Policy   string `json:"policy"`
}

type intentView struct {
	UnsignedTx       any             `json:"unsigned_tx"`
	SignableBytesB64 string          `json:"signable_bytes"`
	SignableBytesHex string          `json:"signable_bytes_hex"`
	Nonce            uint64          `json:"nonce"`
	ChainID          string          `json:"chain_id"`
	FeeEstimate      feeEstimateView `json:"fee_estimate"`
}

// getPositions handles GET /v1/staking/positions?addr= (self-scoped).
func (s *Server) getPositions(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("addr")
	if !s.requireSelf(w, r, addr) {
		return
	}
	p, err := s.svc.GetPosition(r.Context(), addr)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpx.WriteError(w, r, httpx.NewError(httpx.CodePositionNotFound, "no staking position for this address", nil))
			return
		}
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, positionView{
		Addr: p.Addr, StakedAmount: bigStr(p.StakedAmount), PendingWithdrawal: bigStr(p.PendingWithdrawal),
		TotalRewards: bigStr(p.TotalRewards), TotalSlashed: bigStr(p.TotalSlashed),
		WithdrawableAt: rfc3339Ptr(p.WithdrawableAt), IsActive: p.IsActive, UpdatedAt: rfc3339(p.UpdatedAt),
	})
}

// getRewards handles GET /v1/staking/rewards?addr=&limit=&cursor= (self-scoped).
func (s *Server) getRewards(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("addr")
	if !s.requireSelf(w, r, addr) {
		return
	}
	limit := parseLimit(r.URL.Query().Get("limit"))
	items, next, err := s.svc.ListRewards(r.Context(), addr, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	views := make([]rewardView, 0, len(items))
	for _, rw := range items {
		views = append(views, rewardView{
			ID: rw.ID, Addr: rw.Addr, Amount: bigStr(rw.Amount), TotalRewards: bigStr(rw.TotalRewards),
			Source: rw.Source, TxHash: rw.TxHash, BlockHeight: rw.BlockHeight, OccurredAt: rfc3339(rw.OccurredAt),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": views, "next_cursor": next})
}

// postIntent handles POST /v1/staking/intent — returns an UNSIGNED stake/unstake tx + fee estimate.
// NON-CUSTODIAL (A.1): no key material is held or returned; the client signs the bytes itself.
func (s *Server) postIntent(w http.ResponseWriter, r *http.Request) {
	var req intentRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !s.requireSelf(w, r, req.Addr) {
		return
	}
	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidAmount, "amount must be a base-10 integer string", nil))
		return
	}
	intent, err := s.svc.BuildIntent(r.Context(), domain.IntentRequest{Addr: req.Addr, Action: req.Action, Amount: amount})
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	unsigned, err := intent.UnsignedJSON()
	if err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInternalError, "encode unsigned tx", nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intentView{
		UnsignedTx:       unsigned,
		SignableBytesB64: base64.StdEncoding.EncodeToString(intent.SignableBytes),
		SignableBytesHex: "0x" + hex.EncodeToString(intent.SignableBytes),
		Nonce:            intent.Nonce,
		ChainID:          intent.ChainID,
		FeeEstimate: feeEstimateView{
			Amount: bigStr(intent.FeeEstimate.Amount), Currency: intent.FeeEstimate.Currency, Policy: intent.FeeEstimate.Policy,
		},
	})
}
