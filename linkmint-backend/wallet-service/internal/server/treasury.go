package server

import (
	"net/http"
	"time"

	"github.com/paylink/wallet-service/internal/httpx"
)

type treasuryView struct {
	TotalSupply      string `json:"total_supply"`
	MaxSupply        string `json:"max_supply"`
	TotalBurned      string `json:"total_burned"`
	FeesCollected    string `json:"fees_collected"`
	ValidatorRewards string `json:"validator_rewards"`
	TreasuryAmount   string `json:"treasury_amount"`
	ChainHeight      uint64 `json:"chain_height"`
	UpdatedAt        string `json:"updated_at"`
}

// getTreasuryStats handles GET /v1/treasury/stats (public — no auth).
func (s *Server) getTreasuryStats(w http.ResponseWriter, r *http.Request) {
	t, err := s.svc.GetTreasuryStats(r.Context())
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, treasuryView{
		TotalSupply: bigStr(t.TotalSupply), MaxSupply: bigStr(t.MaxSupply), TotalBurned: bigStr(t.TotalBurned),
		FeesCollected: bigStr(t.FeesCollected), ValidatorRewards: bigStr(t.ValidatorRewards),
		TreasuryAmount: bigStr(t.TreasuryAmount), ChainHeight: t.ChainHeight, UpdatedAt: rfc3339(t.UpdatedAt),
	})
}

// rfc3339Ptr formats an optional timestamp ("" when nil/zero).
func rfc3339Ptr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
