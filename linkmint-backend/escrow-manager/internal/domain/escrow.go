// Package domain is the escrow-manager's core: the Escrow entity, the conditional
// release/refund service, and the outbound ports (Store, Publisher) it depends on. Adapters
// (store/memory, store/postgres, events, consumer, sweeper, server) implement or drive these
// ports; domain imports none of them.
//
// NON-CUSTODIAL (invariant A.1): an Escrow is coordination state only. There are no balance
// fields, no wallet, and no chain-write client anywhere in this module — escrow.released and
// escrow.refunded are INSTRUCTIONS for the settlement layer, never fund movements. Funding
// truth arrives from the chain via the chain.paylink.verified bus event (A.3).
package domain

import (
	"time"

	"github.com/paylink/escrow-manager/internal/fsm"
)

// Condition types.
const (
	ConditionDeliveryConfirmation = "delivery_confirmation"
	ConditionTimeLock             = "time_lock"
	ConditionMultiPartyApproval   = "multi_party_approval"
)

// ValidConditionType reports whether t is a supported condition type.
func ValidConditionType(t string) bool {
	switch t {
	case ConditionDeliveryConfirmation, ConditionTimeLock, ConditionMultiPartyApproval:
		return true
	}
	return false
}

// Domain event names (logical, per the backendfeatures.md taxonomy / ADR-004). All four route
// to the `escrow` topic (eventbus.TopicFor). The transport (Kafka/SQS) is a seam — see
// events.Publisher / eventbus-go.
const (
	EventEscrowCreated  = "escrow.created"
	EventEscrowReleased = "escrow.released"
	EventEscrowRefunded = "escrow.refunded"
	EventEscrowDisputed = "escrow.disputed"
)

// ConditionParams is the per-type condition configuration (stored as jsonb).
//   - delivery_confirmation: no params.
//   - time_lock: ReleaseAt (must be future and before timeout_at).
//   - multi_party_approval: Approvers + Threshold (1 ≤ Threshold ≤ len(Approvers)).
type ConditionParams struct {
	ReleaseAt *time.Time `json:"release_at,omitempty"`
	Approvers []string   `json:"approvers,omitempty"`
	Threshold int        `json:"threshold,omitempty"`
}

// Escrow is a conditional-release coordination record for one PayLink (pl_id is an opaque
// reference and UNIQUE — one escrow per PayLink). Amount/Currency mirror the PayLink for
// instruction payloads; they are never a held balance (A.1).
type Escrow struct {
	ID              string
	PLID            string
	CreatorAddr     string
	PayeeAddr       string
	RefundTo        string
	Amount          string // positive integer string (numeric(30,0))
	Currency        string
	ConditionType   string
	ConditionParams ConditionParams
	State           fsm.State
	Funded          bool // set by the chain.paylink.verified consumer — a flag, not a state
	FundedTxHash    string
	ReleaseAt       *time.Time // time_lock only (copied from params for the sweep index)
	TimeoutAt       time.Time
	DisputeReason   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsParticipant reports whether addr is the creator, payee, or refund recipient.
func (e Escrow) IsParticipant(addr string) bool {
	return addr == e.CreatorAddr || addr == e.PayeeAddr || addr == e.RefundTo
}

// CanView reports whether addr may read the escrow: a participant, or — for
// multi_party_approval — one of the listed approvers (they must see what they approve).
func (e Escrow) CanView(addr string) bool {
	if e.IsParticipant(addr) {
		return true
	}
	for _, a := range e.ConditionParams.Approvers {
		if addr == a {
			return true
		}
	}
	return false
}

// createdPayload is the escrow.created event payload.
func (e Escrow) createdPayload() map[string]any {
	p := map[string]any{
		"escrow_id":      e.ID,
		"pl_id":          e.PLID,
		"creator_addr":   e.CreatorAddr,
		"payee_addr":     e.PayeeAddr,
		"refund_to":      e.RefundTo,
		"amount":         e.Amount,
		"currency":       e.Currency,
		"condition_type": e.ConditionType,
		"state":          string(e.State),
		"timeout_at":     e.TimeoutAt.UTC().Format(time.RFC3339Nano),
	}
	if e.ReleaseAt != nil {
		p["release_at"] = e.ReleaseAt.UTC().Format(time.RFC3339Nano)
	}
	return p
}

// releasedPayload is the escrow.released event payload: a release INSTRUCTION (pay the payee
// via the settlement layer), never a transfer (A.1).
func (e Escrow) releasedPayload() map[string]any {
	return map[string]any{
		"escrow_id":  e.ID,
		"pl_id":      e.PLID,
		"payee_addr": e.PayeeAddr,
		"amount":     e.Amount,
		"currency":   e.Currency,
		"funded":     e.Funded,
		"tx_hash":    e.FundedTxHash,
	}
}

// refundedPayload is the escrow.refunded event payload: a refund INSTRUCTION (return funds to
// refund_to via the rail/chain). funded=false means nothing was ever paid — nothing to move.
func (e Escrow) refundedPayload() map[string]any {
	return map[string]any{
		"escrow_id": e.ID,
		"pl_id":     e.PLID,
		"refund_to": e.RefundTo,
		"amount":    e.Amount,
		"currency":  e.Currency,
		"funded":    e.Funded,
		"tx_hash":   e.FundedTxHash,
	}
}

// disputedPayload is the escrow.disputed event payload (resolution is work22).
func (e Escrow) disputedPayload() map[string]any {
	return map[string]any{
		"escrow_id": e.ID,
		"pl_id":     e.PLID,
		"reason":    e.DisputeReason,
		"funded":    e.Funded,
		"state":     string(e.State),
	}
}
