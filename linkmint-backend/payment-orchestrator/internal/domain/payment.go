// Package domain is the orchestrator's core: the Payment entity, the payment lifecycle
// service, and the outbound ports (Store, ChainReader, PayLinkLookup, Publisher) it depends on.
// Adapters (store/postgres, chain, paylinks, events) implement these ports; domain imports none
// of them, keeping settlement logic decoupled from transport.
package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/paylink/payment-orchestrator/internal/lifecycle"
)

// Payment is an orchestration record: it tracks the lifecycle of a payment expected for a
// PayLink. It holds NO funds and no fund-moving credentials (invariant A.1) and no rail-specific
// fields beyond the chosen rail label (invariant A.4) — Rail is an opaque routing hint.
type Payment struct {
	ID           string
	PayLinkID    string
	Rail         string
	Status       lifecycle.State
	LastEventSeq uint64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Domain event names (logical, per the backendfeatures.md taxonomy / ADR-004). The transport
// (Kafka/SQS) is a seam — see events.Publisher.
const (
	EventPaymentInitiated = "payment.initiated"
	EventPaymentSettled   = "payment.settled"
	EventPaymentCancelled = "payment.cancelled"
	EventPaymentFailed    = "payment.failed"
)

var allowedRails = map[string]bool{"mpesa": true, "card": true, "bank": true, "crypto": true}

// ValidRail reports whether r is a supported rail label.
func ValidRail(r string) bool { return allowedRails[r] }

var hashRe = regexp.MustCompile(`^0x[0-9a-f]{64}$`)

// normalizeHash lowercases and 0x-prefixes a hex PayLink id for canonical comparison/storage.
func normalizeHash(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if !strings.HasPrefix(h, "0x") {
		h = "0x" + h
	}
	return h
}

// validHash reports whether h is a canonical 32-byte (64 hex) 0x-prefixed PayLink id.
func validHash(h string) bool { return hashRe.MatchString(h) }

// domainEventForState maps a terminal lifecycle state to its domain event name ("" if none).
func domainEventForState(st lifecycle.State) string {
	switch st {
	case lifecycle.StateSettled:
		return EventPaymentSettled
	case lifecycle.StateCancelled:
		return EventPaymentCancelled
	case lifecycle.StateFailed:
		return EventPaymentFailed
	default:
		return ""
	}
}

// payload builds the domain-event payload for a payment (stable, transport-agnostic).
func (p Payment) payload() map[string]any {
	return map[string]any{
		"payment_id": p.ID,
		"paylink_id": p.PayLinkID,
		"rail":       p.Rail,
		"status":     string(p.Status),
		"created_at": p.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": p.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
