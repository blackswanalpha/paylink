// Package consumer handles chain.paylink.verified bus events (the chain topic, published by
// chain-event-mirror): on-chain settlement of the escrowed PayLink marks the escrow funded and
// may release it (A.3 — funding truth comes from the chain, never invented here).
//
// The handler is wired to eventbus-go's Consumer (group "escrow-manager", topic "chain").
// Contract: returning an error means "not handled" — the offset is NOT committed and the event
// redelivers, so everything in here must be idempotent. The durable dedupe lives in the store
// (DbDedupe on escrow.processed_events, same transaction as the funded-write — work17).
// Undecodable/incomplete payloads are logged and dropped (poison-safe): they can never succeed
// and would otherwise block the partition forever.
package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/paylink/escrow-manager/internal/domain"
)

// EventPaylinkVerified is the logical event name this consumer reacts to.
const EventPaylinkVerified = "chain.paylink.verified"

// Service is the domain surface the consumer drives.
type Service interface {
	// HandlePaylinkVerified applies the funding event; it returns the consumed-result label
	// (domain.Result*) or an error that must trigger redelivery.
	HandlePaylinkVerified(ctx context.Context, plID, txHash string) (string, error)
}

// Recorder records escrow_events_consumed_total{result} (nil-safe).
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

// verifiedPayload is the subset of the chain-event-mirror payload the escrow needs. entity_id
// is the PayLink id (pl_id is accepted as a fallback for forward-compat).
type verifiedPayload struct {
	EntityID string `json:"entity_id"`
	PLID     string `json:"pl_id"`
	TxHash   string `json:"tx_hash"`
}

// Handle processes one bus event. Non-matching names are committed untouched; a service error
// propagates (no offset commit → redelivery).
func (h *Handler) Handle(ctx context.Context, name string, payload json.RawMessage) error {
	if name != EventPaylinkVerified {
		return nil
	}
	var p verifiedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		h.log.Warn("escrow_event_decode_failed", "name", name, "err", err.Error())
		h.record(domain.ResultIgnored)
		return nil // poison-safe: skip + commit
	}
	plID := p.EntityID
	if plID == "" {
		plID = p.PLID
	}
	if plID == "" {
		h.log.Warn("escrow_event_missing_paylink_id", "name", name)
		h.record(domain.ResultIgnored)
		return nil // poison-safe: skip + commit
	}

	result, err := h.svc.HandlePaylinkVerified(ctx, plID, p.TxHash)
	if err != nil {
		h.record("error")
		h.log.Error("escrow_event_handle_failed", "pl_id", plID, "err", err.Error())
		return err // no offset commit → the bus redelivers
	}
	h.record(result)
	if result == domain.ResultFunded || result == domain.ResultReleased {
		h.log.Info("escrow_event_applied", "pl_id", plID, "result", result)
	}
	return nil
}

func (h *Handler) record(result string) {
	if h.m != nil {
		h.m.EventConsumed(result)
	}
}
