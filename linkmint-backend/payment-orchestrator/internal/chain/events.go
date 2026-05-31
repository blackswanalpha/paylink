// Package chain is the orchestrator's boundary to the lVM: a JSON-RPC client for authoritative
// PayLink status and the event types/contract for the WebSocket datastream (/ws). It speaks the
// chain's JSON wire format only — it does NOT import paylink-chain/internal/* (byte-exact tx
// wire format is the proof-validator's / adapters' concern, work03/04).
package chain

import "context"

// Event mirrors the lVM datastream event JSON (paylink-chain/internal/events.Event). Only the
// fields the orchestrator consumes are decoded; field tags match the chain byte-for-byte.
type Event struct {
	Sequence    uint64 `json:"seq"`
	Kind        string `json:"kind"`
	EntityType  string `json:"entityType"`
	EntityID    string `json:"entityId"`
	Timestamp   int64  `json:"timestamp"`
	BlockHeight uint64 `json:"blockHeight"`
	TxHash      string `json:"txHash,omitempty"`
	FromState   string `json:"fromState,omitempty"`
	ToState     string `json:"toState,omitempty"`
	Transition  string `json:"transition,omitempty"`
}

// Chain event kinds and the entity type the orchestrator subscribes to.
const (
	KindPayLinkCreated   = "paylink.created"
	KindPayLinkVerified  = "paylink.verified"
	KindPayLinkCancelled = "paylink.cancelled"
	KindPayLinkFailed    = "paylink.failed"
	EntityPayLink        = "paylink"
)

// SettlementEventKinds advance a payment to a terminal lifecycle state. Used as the WS filter.
var SettlementEventKinds = []string{KindPayLinkVerified, KindPayLinkCancelled, KindPayLinkFailed}

// ChainStatusForEvent maps an event to the on-chain PayLink status it implies. It prefers the
// event's ToState (the FSM state, which equals the status string), falling back to the kind.
// ok is false for events that imply no payable/settlement status.
func ChainStatusForEvent(ev Event) (string, bool) {
	switch ev.ToState {
	case "VERIFIED", "CANCELLED", "FAILED", "CREATED":
		return ev.ToState, true
	}
	switch ev.Kind {
	case KindPayLinkVerified:
		return "VERIFIED", true
	case KindPayLinkCancelled:
		return "CANCELLED", true
	case KindPayLinkFailed:
		return "FAILED", true
	case KindPayLinkCreated:
		return "CREATED", true
	}
	return "", false
}

// EventSource is a stream of chain events. Run blocks, invoking handle for each event until ctx
// is cancelled, reconnecting internally on transient failures. Implemented by chain/wsstream.
type EventSource interface {
	Run(ctx context.Context, handle func(context.Context, Event) error) error
}
