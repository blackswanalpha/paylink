package server

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"

	"github.com/paylink/mpesa-adapter/internal/daraja"
	"github.com/paylink/mpesa-adapter/internal/httpx"
)

const internalTokenHeader = "X-Internal-Token"

// callbackView is the POST /v1/callbacks/mpesa response (to the Node rail service).
type callbackView struct {
	Status    string `json:"status"`
	ProofHash string `json:"proof_hash,omitempty"`
	TxHash    string `json:"tx_hash,omitempty"`
}

// handleCallback handles POST /v1/callbacks/mpesa: the Node rail service forwards a rail-neutral,
// already-parsed STK callback here. We authenticate the internal token, normalize → sign →
// broadcast. A 2xx acknowledges the callback (terminal); a 5xx asks the caller to redeliver.
func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if s.internalToken != "" {
		got := r.Header.Get(internalTokenHeader)
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.internalToken)) != 1 {
			httpx.WriteError(w, r, httpx.NewError(httpx.CodeUnauthorized, "invalid or missing internal token", nil))
			return
		}
	}

	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "could not read request body", nil))
		return
	}
	var cb daraja.CallbackResult
	if err := json.Unmarshal(raw, &cb); err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "callback body is not valid JSON: "+err.Error(), nil))
		return
	}
	if cb.CheckoutRequestID == "" {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "checkout_request_id is required", nil))
		return
	}

	outcome, err := s.svc.HandleCallback(ctx, cb)
	if err != nil {
		// Retryable (validator unavailable / internal): surface it so the caller redelivers.
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, callbackView{Status: outcome.Status, ProofHash: outcome.ProofHash, TxHash: outcome.TxHash})
}
