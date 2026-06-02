package ledger

import "errors"

// Sentinel errors. Callers can errors.Is() against these; the wrapped message carries detail.
var (
	// ErrUnbalanced is returned when a posting's DR total != CR total (per currency), or it lacks
	// at least one DR and one CR leg. Posting it would violate A.6, so it is rejected before any write.
	ErrUnbalanced = errors.New("ledger: unbalanced posting")

	// ErrInvalidLeg is returned for a malformed leg: bad direction, non-positive amount, or empty
	// account/currency.
	ErrInvalidLeg = errors.New("ledger: invalid leg")

	// ErrGroupNotFound is returned by Reverse when the referenced entry_group has no entries.
	ErrGroupNotFound = errors.New("ledger: entry group not found")
)
