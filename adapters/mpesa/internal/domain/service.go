package domain

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/paylink/mpesa-adapter/internal/correlation"
	"github.com/paylink/mpesa-adapter/internal/daraja"
	"github.com/paylink/mpesa-adapter/internal/httpx"
	"github.com/paylink/mpesa-adapter/internal/proof"
)

// Service runs the adapter pipeline.
type Service struct {
	rail             RailClient
	corr             correlation.Store
	signer           Signer
	bcast            Broadcaster
	metrics          Metrics // nil-safe
	log              *slog.Logger
	now              func() time.Time
	defaultShortCode string
}

// Option configures a Service.
type Option func(*Service)

// WithMetrics attaches a metrics hook.
func WithMetrics(m Metrics) Option { return func(s *Service) { s.metrics = m } }

// WithClock overrides the time source (tests).
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// NewService builds a Service. log may be nil (defaults to slog.Default).
func NewService(rail RailClient, corr correlation.Store, signer Signer, bcast Broadcaster, defaultShortCode string, log *slog.Logger, opts ...Option) *Service {
	if log == nil {
		log = slog.Default()
	}
	s := &Service{
		rail:             rail,
		corr:             corr,
		signer:           signer,
		bcast:            bcast,
		log:              log,
		now:              time.Now,
		defaultShortCode: defaultShortCode,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// ChargeInput is the validated input to InitiateCharge.
type ChargeInput struct {
	PayLinkID         string
	Amount            uint64
	PayerPhone        string
	ReceiverShortCode string // optional; defaults to the configured shortcode (A.1: the RECEIVER's)
}

// ChargeResult is returned to the caller of POST /v1/charges.
type ChargeResult struct {
	CheckoutRequestID string
	MerchantRequestID string
	Status            string // "pending"
}

// InitiateCharge validates the request, asks the rail service to start an STK push, and records the
// correlation so the asynchronous callback can be matched back to this PayLink.
func (s *Service) InitiateCharge(ctx context.Context, in ChargeInput) (ChargeResult, error) {
	if !validHash(in.PayLinkID) {
		return ChargeResult{}, httpx.NewError(httpx.CodeInvalidPayload, "pl_id must be a 0x-prefixed 32-byte hex hash", map[string]any{"pl_id": in.PayLinkID})
	}
	if in.Amount == 0 {
		return ChargeResult{}, httpx.NewError(httpx.CodeInvalidPayload, "amount must be greater than zero", nil)
	}
	if in.PayerPhone == "" {
		return ChargeResult{}, httpx.NewError(httpx.CodeInvalidPayload, "payer_phone is required", nil)
	}
	receiver := in.ReceiverShortCode
	if receiver == "" {
		receiver = s.defaultShortCode
	}
	if receiver == "" {
		return ChargeResult{}, httpx.NewError(httpx.CodeInvalidPayload, "receiver_shortcode is required (no default configured)", nil)
	}

	res, err := s.rail.STKPush(ctx, daraja.STKPushParams{
		ShortCode:  receiver,
		PayerPhone: in.PayerPhone,
		Amount:     in.Amount,
		AccountRef: accountRef(in.PayLinkID),
		PayLinkID:  in.PayLinkID,
	})
	if err != nil {
		s.mark(func(m Metrics) { m.ChargeInitiated("daraja_error") })
		return ChargeResult{}, err
	}

	if perr := s.corr.Put(ctx, res.CheckoutRequestID, correlation.Record{
		PayLinkID:  in.PayLinkID,
		Amount:     in.Amount,
		Receiver:   receiver,
		PayerPhone: in.PayerPhone,
	}); perr != nil {
		s.mark(func(m Metrics) { m.ChargeInitiated("error") })
		return ChargeResult{}, httpx.NewError(httpx.CodeInternalError, "store correlation: "+perr.Error(), nil)
	}

	s.mark(func(m Metrics) { m.ChargeInitiated("accepted") })
	s.log.Info("charge_initiated", "paylink_id", in.PayLinkID, "checkout_request_id", res.CheckoutRequestID, "receiver", receiver)
	return ChargeResult{CheckoutRequestID: res.CheckoutRequestID, MerchantRequestID: res.MerchantRequestID, Status: "pending"}, nil
}

// CallbackOutcome is the result of handling a rail callback.
type CallbackOutcome struct {
	Status    string // broadcast | already_settled | rejected | ignored_no_correlation | ignored_failed_payment | ignored_amount_mismatch
	ProofHash string
	TxHash    string
}

// HandleCallback normalizes a rail-neutral callback into a proof, signs it, and broadcasts it.
// A returned error is RETRYABLE (validator unavailable / internal) — the HTTP layer surfaces it so
// the rail service / Daraja redeliver. Terminal results (ignored / rejected / settled) return a nil
// error with a descriptive Outcome.Status so the callback is acknowledged.
func (s *Service) HandleCallback(ctx context.Context, cb daraja.CallbackResult) (CallbackOutcome, error) {
	rec, err := s.corr.Get(ctx, cb.CheckoutRequestID)
	if err != nil {
		if errors.Is(err, correlation.ErrNotFound) {
			s.mark(func(m Metrics) { m.CallbackReceived("no_correlation") })
			s.log.Warn("callback_no_correlation", "checkout_request_id", cb.CheckoutRequestID)
			return CallbackOutcome{Status: "ignored_no_correlation"}, nil
		}
		s.mark(func(m Metrics) { m.CallbackReceived("error") })
		return CallbackOutcome{}, httpx.NewError(httpx.CodeInternalError, "load correlation: "+err.Error(), nil)
	}

	if !cb.Succeeded() {
		s.mark(func(m Metrics) { m.CallbackReceived("failed_payment") })
		s.log.Info("callback_failed_payment", "paylink_id", rec.PayLinkID, "result_code", cb.ResultCode, "result_desc", cb.ResultDesc)
		return CallbackOutcome{Status: "ignored_failed_payment"}, nil
	}

	// Non-custodial correctness: only prove a payment that paid exactly what the PayLink requires.
	// A different amount must not settle (and the validator would reject it as PROOF_AMOUNT_MISMATCH).
	if cb.Amount != 0 && cb.Amount != rec.Amount {
		s.mark(func(m Metrics) { m.CallbackReceived("amount_mismatch") })
		s.log.Warn("callback_amount_mismatch", "paylink_id", rec.PayLinkID, "paid", cb.Amount, "expected", rec.Amount)
		return CallbackOutcome{Status: "ignored_amount_mismatch"}, nil
	}

	txID := cb.MpesaReceiptNumber
	if txID == "" {
		txID = cb.CheckoutRequestID
	}
	sender := cb.PhoneNumber
	if sender == "" {
		sender = rec.PayerPhone
	}

	p := proof.Proof{
		PayLinkID: rec.PayLinkID,
		Rail:      "mpesa",
		TxID:      txID,
		Amount:    rec.Amount, // == on-chain PayLink amount (validator cross-checks)
		Timestamp: s.now().Unix(),
		Sender:    sender,
		Receiver:  rec.Receiver,
	}
	if verr := proof.Validate(p); verr != nil {
		s.mark(func(m Metrics) { m.CallbackReceived("error") })
		return CallbackOutcome{}, httpx.NewError(httpx.CodeInternalError, "normalized proof invalid: "+verr.Error(), nil)
	}

	sig, serr := s.signer.Sign(p)
	if serr != nil {
		s.mark(func(m Metrics) { m.CallbackReceived("error") })
		return CallbackOutcome{}, httpx.NewError(httpx.CodeInternalError, "sign proof: "+serr.Error(), nil)
	}
	p.Signature = sig

	// Deterministic Idempotency-Key (the rail tx id): a re-delivered callback re-broadcasts the same
	// key and the validator replays "already_settled" (A.7) rather than double-settling.
	res, berr := s.bcast.Broadcast(ctx, p, "mpesa:"+txID)
	if berr != nil {
		if httpx.AsAppError(berr).Code == httpx.CodeProofRejected {
			s.mark(func(m Metrics) { m.ProofBroadcast("rejected") })
			s.log.Error("proof_rejected_by_validator", "paylink_id", rec.PayLinkID, "tx_id", txID, "err", berr.Error())
			return CallbackOutcome{Status: "rejected"}, nil // terminal — ack, do not retry
		}
		s.mark(func(m Metrics) { m.ProofBroadcast("error") })
		return CallbackOutcome{}, berr // retryable (validator unavailable / internal)
	}

	outcome := "broadcast"
	if res.Status != "broadcast" {
		outcome = res.Status // "already_settled" | "settled"
	}
	s.mark(func(m Metrics) { m.ProofBroadcast(res.Status) })
	s.mark(func(m Metrics) { m.CallbackReceived(outcome) })
	s.log.Info("proof_broadcast", "paylink_id", rec.PayLinkID, "tx_id", txID, "proof_hash", res.ProofHash, "settlement_tx", res.TxHash, "status", res.Status)
	return CallbackOutcome{Status: outcome, ProofHash: res.ProofHash, TxHash: res.TxHash}, nil
}

func (s *Service) mark(f func(Metrics)) {
	if s.metrics != nil {
		f(s.metrics)
	}
}

// validHash reports whether s is a 0x-prefixed 32-byte (64 hex char) hash.
func validHash(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") {
		return false
	}
	b, err := hex.DecodeString(s[2:])
	return err == nil && len(b) == 32
}

// accountRef derives a short Daraja AccountReference (<= 12 chars) from the PayLink id.
func accountRef(plID string) string {
	h := strings.TrimPrefix(plID, "0x")
	if len(h) > 12 {
		h = h[:12]
	}
	return h
}
