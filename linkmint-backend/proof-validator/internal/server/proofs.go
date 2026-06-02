package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	idempotency "github.com/paylink/idempotency-go"
	"github.com/paylink/proof-validator/internal/domain"
	"github.com/paylink/proof-validator/internal/httpx"
	"github.com/paylink/proof-validator/internal/proof"
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
	maxBodyBytes = 1 << 20 // 1 MiB
	idemHeader   = "Idempotency-Key"
	submitRoute  = "submit_proof"
)

// submitView is the POST /v1/proofs response.
type submitView struct {
	ProofHash string `json:"proof_hash"`
	TxHash    string `json:"tx_hash,omitempty"`
	Status    string `json:"status"`
}

// recordView is the GET /v1/proofs/{proof_hash} response.
type recordView struct {
	ProofHash string `json:"proof_hash"`
	PayLinkID string `json:"paylink_id"`
	Rail      string `json:"rail"`
	TxID      string `json:"tx_id"`
	Amount    uint64 `json:"amount"`
	Status    string `json:"status"`
	TxHash    string `json:"tx_hash,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// statusCode maps a result status to an HTTP status: a freshly broadcast settlement is 202
// Accepted (finality is the chain's async quorum decision); everything else is 200 OK.
func statusCode(status string) int {
	if status == domain.StatusBroadcast {
		return http.StatusAccepted
	}
	return http.StatusOK
}

// submitProof handles POST /v1/proofs. It is idempotent on the Idempotency-Key header: verify the
// proof, then broadcast a settlement transaction (or reject without broadcasting).
func (s *Server) submitProof(w http.ResponseWriter, r *http.Request) {
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
	p, err := proof.Parse(raw)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	fp := idempotency.Fingerprint(raw)
	cached, err := s.idem.Begin(ctx, submitRoute, idemKey, fp)
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

	res, err := s.svc.SubmitProof(ctx, p)
	if err != nil {
		// Release the reservation so a corrected retry can reuse the key.
		s.idem.Release(ctx, submitRoute, idemKey)
		httpx.WriteError(w, r, err)
		return
	}

	view := submitView{ProofHash: res.ProofHash, TxHash: res.TxHash, Status: res.Status}
	body, _ := json.Marshal(view) // submitView is statically marshalable — cannot fail
	code := statusCode(res.Status)
	if err := s.idem.Complete(ctx, submitRoute, idemKey, fp, code, body); err != nil {
		s.log.Warn("idempotency_complete_failed", "err", err.Error())
	}
	httpx.WriteJSON(w, code, view)
}

// getProof handles GET /v1/proofs/{proof_hash}.
func (s *Server) getProof(w http.ResponseWriter, r *http.Request) {
	rec, err := s.svc.Get(r.Context(), chi.URLParam(r, "proof_hash"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, recordView{
		ProofHash: rec.ProofHash,
		PayLinkID: rec.PayLinkID,
		Rail:      rec.Rail,
		TxID:      rec.TxID,
		Amount:    rec.Amount,
		Status:    rec.Status,
		TxHash:    rec.TxHash,
		CreatedAt: rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
}
