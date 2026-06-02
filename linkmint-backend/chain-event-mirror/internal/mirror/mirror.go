// Package mirror translates lVM chain events into chain.<kind> domain events on the bus.
package mirror

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/paylink/chain-event-mirror/internal/chain"
	"github.com/paylink/chain-event-mirror/internal/metrics"
)

// Publisher is the subset of eventbus-go's Publisher the mirror uses.
type Publisher interface {
	Publish(ctx context.Context, name, key string, payload any) error
}

// Mirror republishes chain events as chain.<kind> domain events (topic "chain", key = entity id).
type Mirror struct {
	pub Publisher
	m   *metrics.Metrics
	log *slog.Logger
}

// New builds a Mirror (log may be nil → slog.Default; m may be nil → no metrics).
func New(pub Publisher, m *metrics.Metrics, log *slog.Logger) *Mirror {
	if log == nil {
		log = slog.Default()
	}
	return &Mirror{pub: pub, m: m, log: log}
}

// eventPayload is the chain event projected onto the envelope payload (snake_case; the chain's
// monotonic seq is intentionally dropped — it is not stable across a node restart).
type eventPayload struct {
	EntityID    string          `json:"entity_id"`
	EntityType  string          `json:"entity_type"`
	Kind        string          `json:"kind"`
	BlockHeight uint64          `json:"block_height"`
	Timestamp   int64           `json:"timestamp"`
	TxHash      string          `json:"tx_hash,omitempty"`
	FromState   string          `json:"from_state,omitempty"`
	ToState     string          `json:"to_state,omitempty"`
	Transition  string          `json:"transition,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
}

// Handle maps a chain event to a chain.<kind> envelope and publishes it. It is the wsstream handler.
// A publish error is returned (logged by the WS loop) and recorded; the live stream then continues —
// the mirror is best-effort, with the chain RPC remaining authoritative.
func (mr *Mirror) Handle(ctx context.Context, ev chain.Event) error {
	if ev.Kind == "" {
		return nil // nothing to mirror
	}
	name := "chain." + ev.Kind
	payload := eventPayload{
		EntityID:    ev.EntityID,
		EntityType:  ev.EntityType,
		Kind:        ev.Kind,
		BlockHeight: ev.BlockHeight,
		Timestamp:   ev.Timestamp,
		TxHash:      ev.TxHash,
		FromState:   ev.FromState,
		ToState:     ev.ToState,
		Transition:  ev.Transition,
		Data:        ev.Data,
	}
	if err := mr.pub.Publish(ctx, name, ev.EntityID, payload); err != nil {
		mr.record(ev.Kind, "error")
		return err
	}
	mr.record(ev.Kind, "ok")
	mr.log.Debug("mirrored", "name", name, "key", ev.EntityID, "height", ev.BlockHeight)
	return nil
}

func (mr *Mirror) record(kind, result string) {
	if mr.m != nil {
		mr.m.Mirrored(kind, result)
	}
}
