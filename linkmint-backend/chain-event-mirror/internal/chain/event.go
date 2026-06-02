// Package chain is the mirror's view of the lVM datastream wire format. Like
// payment-orchestrator/internal/chain, it does NOT import paylink-chain/internal/* — Go forbids
// cross-module internal imports — so the Event struct and the datastream protocol are hand-mirrored
// byte-for-byte (field tags match the chain). The chain RPC remains the authoritative source of
// truth; this bus projection is best-effort.
package chain

import (
	"context"
	"encoding/json"
)

// Event mirrors paylink-chain/internal/events.Event. All fields are decoded (the mirror forwards the
// whole event, including the kind-specific Data payload, onto the bus).
type Event struct {
	Sequence    uint64          `json:"seq"`
	Kind        string          `json:"kind"`
	EntityType  string          `json:"entityType"`
	EntityID    string          `json:"entityId"`
	Timestamp   int64           `json:"timestamp"`
	BlockHeight uint64          `json:"blockHeight"`
	TxHash      string          `json:"txHash,omitempty"`
	FromState   string          `json:"fromState,omitempty"`
	ToState     string          `json:"toState,omitempty"`
	Transition  string          `json:"transition,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
}

// EventSource is a stream of chain events. Run blocks, invoking handle for each event until ctx is
// cancelled, reconnecting internally on transient failures. Implemented by chain/wsstream.
type EventSource interface {
	Run(ctx context.Context, handle func(context.Context, Event) error) error
}
