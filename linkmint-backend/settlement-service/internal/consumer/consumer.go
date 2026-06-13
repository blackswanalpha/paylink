// Package consumer bridges the work15 event bus to the settlement domain. It subscribes to the
// chain, merchant, and refund topics and dispatches by logical event name:
//
//	chain.paylink.verified        → aggregate the PayLink into its merchant's settlement (gross)
//	chain.fee.collected           → attach the chain/protocol fee (A.5)
//	merchant.onboarded            → merchant projection
//	merchant.bank_account.verified→ bank-account projection
//	refund.clawback.requested     → negative offset against the merchant's next settlement
//
// Contract (eventbus-go): returning an error means "not handled" — the offset is NOT committed and
// the event redelivers, so handlers MUST be idempotent (DbDedupe in the store, work17). Unknown
// names are committed untouched; undecodable payloads are logged + skipped (poison-safe).
package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/big"
	"time"

	"github.com/paylink/settlement-service/internal/domain"
)

// Consumed event logical names.
const (
	EventPaylinkVerified = "chain.paylink.verified"
	EventFeeCollected    = "chain.fee.collected"
	EventMerchantOnboard = "merchant.onboarded"
	EventBankVerified    = "merchant.bank_account.verified"
	EventClawbackRequest = "refund.clawback.requested"
)

// Topics the consumer subscribes to.
var Topics = []string{"chain", "merchant", "refund"}

// Service is the domain surface the consumer drives.
type Service interface {
	HandleVerified(ctx context.Context, ev domain.VerifiedEvent) (string, error)
	HandleFee(ctx context.Context, ev domain.FeeEvent) (string, error)
	HandleMerchantOnboarded(ctx context.Context, ev domain.MerchantOnboardedEvent) (string, error)
	HandleBankAccountVerified(ctx context.Context, ev domain.BankAccountVerifiedEvent) (string, error)
	HandleClawback(ctx context.Context, ev domain.ClawbackEvent) (string, error)
}

// Recorder records settlement_events_consumed_total{result} (nil-safe).
type Recorder interface {
	EventConsumed(result string)
}

// Handler filters and dispatches bus events. Its Handle matches eventbus.HandleFunc.
type Handler struct {
	svc Service
	m   Recorder
	log *slog.Logger
}

// New builds a Handler (log may be nil → slog.Default; m may be nil → no metrics).
func New(svc Service, m Recorder, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{svc: svc, m: m, log: log}
}

// chainPayload is the chain-event-mirror projected envelope payload (the fields settlement needs).
type chainPayload struct {
	EntityID  string          `json:"entity_id"`
	TxHash    string          `json:"tx_hash"`
	Timestamp int64           `json:"timestamp"` // unix milliseconds (chain event time)
	Data      json.RawMessage `json:"data"`
}

type settledData struct {
	Payee  string `json:"payee"`
	Amount uint64 `json:"amount"`
}

type feeData struct {
	PayLinkID string `json:"paylinkId"`
	TotalFee  uint64 `json:"totalFee"`
}

type merchantOnboarded struct {
	MerchantID string `json:"merchant_id"`
	Status     string `json:"status"`
}

type bankVerified struct {
	BankAccountID string `json:"bank_account_id"`
	MerchantID    string `json:"merchant_id"`
	Rail          string `json:"rail"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
}

type clawbackRequested struct {
	DisputeID   string `json:"dispute_id"`
	PayLinkID   string `json:"paylink_id"`
	AmountMinor *int64 `json:"amount_minor"`
}

// Handle processes one bus event. A decode failure is poison-safe (logged + committed); a service
// error propagates (no commit → redelivery).
func (h *Handler) Handle(ctx context.Context, name string, payload json.RawMessage) error {
	switch name {
	case EventPaylinkVerified:
		return h.handleVerified(ctx, payload)
	case EventFeeCollected:
		return h.handleFee(ctx, payload)
	case EventMerchantOnboard:
		return h.handleMerchant(ctx, payload)
	case EventBankVerified:
		return h.handleBank(ctx, payload)
	case EventClawbackRequest:
		return h.handleClawback(ctx, payload)
	default:
		return nil // not ours — commit untouched
	}
}

func (h *Handler) handleVerified(ctx context.Context, payload json.RawMessage) error {
	var p chainPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return h.poison("verified", err)
	}
	var d settledData
	if len(p.Data) > 0 {
		_ = json.Unmarshal(p.Data, &d)
	}
	ev := domain.VerifiedEvent{
		PLID:       p.EntityID,
		Payee:      d.Payee,
		Amount:     new(big.Int).SetUint64(d.Amount),
		TxHash:     p.TxHash,
		OccurredAt: msToTime(p.Timestamp),
	}
	return h.dispatch(h.svc.HandleVerified(ctx, ev))
}

func (h *Handler) handleFee(ctx context.Context, payload json.RawMessage) error {
	var p chainPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return h.poison("fee", err)
	}
	var d feeData
	if len(p.Data) > 0 {
		_ = json.Unmarshal(p.Data, &d)
	}
	plID := p.EntityID
	if plID == "" {
		plID = d.PayLinkID
	}
	ev := domain.FeeEvent{PLID: plID, ChainFee: new(big.Int).SetUint64(d.TotalFee)}
	return h.dispatch(h.svc.HandleFee(ctx, ev))
}

func (h *Handler) handleMerchant(ctx context.Context, payload json.RawMessage) error {
	var p merchantOnboarded
	if err := json.Unmarshal(payload, &p); err != nil {
		return h.poison("merchant", err)
	}
	return h.dispatch(h.svc.HandleMerchantOnboarded(ctx, domain.MerchantOnboardedEvent{
		MerchantID: p.MerchantID, Status: p.Status,
	}))
}

func (h *Handler) handleBank(ctx context.Context, payload json.RawMessage) error {
	var p bankVerified
	if err := json.Unmarshal(payload, &p); err != nil {
		return h.poison("bank", err)
	}
	return h.dispatch(h.svc.HandleBankAccountVerified(ctx, domain.BankAccountVerifiedEvent{
		BankAccountID: p.BankAccountID, MerchantID: p.MerchantID,
		Rail: p.Rail, Currency: p.Currency, Status: p.Status,
	}))
}

func (h *Handler) handleClawback(ctx context.Context, payload json.RawMessage) error {
	var p clawbackRequested
	if err := json.Unmarshal(payload, &p); err != nil {
		return h.poison("clawback", err)
	}
	amount := big.NewInt(0)
	if p.AmountMinor != nil {
		amount = big.NewInt(*p.AmountMinor)
	}
	return h.dispatch(h.svc.HandleClawback(ctx, domain.ClawbackEvent{
		RefundID: p.DisputeID, PLID: p.PayLinkID, Amount: amount,
	}))
}

// dispatch records the result metric and returns the service error (→ redelivery) if any.
func (h *Handler) dispatch(result string, err error) error {
	if err != nil {
		h.record(domain.ResultError)
		return err
	}
	h.record(result)
	return nil
}

// poison logs an undecodable payload, records it as ignored, and commits (skips) it.
func (h *Handler) poison(kind string, err error) error {
	h.log.Warn("settlement_event_decode_failed", "kind", kind, "err", err.Error())
	h.record(domain.ResultIgnored)
	return nil
}

func (h *Handler) record(result string) {
	if h.m != nil {
		h.m.EventConsumed(result)
	}
}

func msToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
