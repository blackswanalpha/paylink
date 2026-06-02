package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	idempotency "github.com/paylink/idempotency-go"
	"github.com/paylink/mpesa-adapter/internal/domain"
	"github.com/paylink/mpesa-adapter/internal/httpx"
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
	idemHeader   = "Idempotency-Key"
	chargesRoute = "charges"
)

// chargeRequest is the POST /v1/charges body. No rail-specific fields beyond a phone/shortcode,
// which are the proof's sender/receiver identifiers (within the A.4 shape).
type chargeRequest struct {
	PayLinkID         string `json:"pl_id"`
	Amount            uint64 `json:"amount"`
	PayerPhone        string `json:"payer_phone"`
	ReceiverShortCode string `json:"receiver_shortcode"`
}

// chargeView is the POST /v1/charges response.
type chargeView struct {
	CheckoutRequestID string `json:"checkout_request_id"`
	MerchantRequestID string `json:"merchant_request_id,omitempty"`
	Status            string `json:"status"`
}

// initiateCharge handles POST /v1/charges: start an STK push (via the rail service) for a PayLink.
// Idempotent on the Idempotency-Key header so a retried initiate does not start two pushes.
func (s *Server) initiateCharge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idemKey := r.Header.Get(idemHeader)
	if idemKey == "" {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "Idempotency-Key header is required", nil))
		return
	}

	var req chargeRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	fp := idempotency.Fingerprint([]byte(req.PayLinkID + "|" + req.PayerPhone + "|" + req.ReceiverShortCode + "|" + strconv.FormatUint(req.Amount, 10)))
	cached, err := s.idem.Begin(ctx, chargesRoute, idemKey, fp)
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

	res, err := s.svc.InitiateCharge(ctx, domain.ChargeInput{
		PayLinkID:         req.PayLinkID,
		Amount:            req.Amount,
		PayerPhone:        req.PayerPhone,
		ReceiverShortCode: req.ReceiverShortCode,
	})
	if err != nil {
		s.idem.Release(ctx, chargesRoute, idemKey)
		httpx.WriteError(w, r, err)
		return
	}

	view := chargeView{CheckoutRequestID: res.CheckoutRequestID, MerchantRequestID: res.MerchantRequestID, Status: res.Status}
	body, _ := json.Marshal(view)
	if err := s.idem.Complete(ctx, chargesRoute, idemKey, fp, http.StatusAccepted, body); err != nil {
		s.log.Warn("idempotency_complete_failed", "err", err.Error())
	}
	httpx.WriteJSON(w, http.StatusAccepted, view)
}
