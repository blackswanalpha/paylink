// Package correlation maps a Daraja CheckoutRequestID back to the PayLink it was charging, so an
// asynchronous STK callback can be normalized into a proof. Deduping duplicate callbacks is NOT
// this layer's job: the adapter broadcasts with a deterministic Idempotency-Key (the rail tx id)
// and the proof-validator's idempotency + the on-chain proof-hash check (A.7) are the single
// anti-replay authority — a re-delivered callback simply re-broadcasts and gets "already_settled".
package correlation

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Get when no correlation exists for a CheckoutRequestID (unknown or
// expired charge).
var ErrNotFound = errors.New("correlation not found")

// Record is what we stored when initiating the charge — the expected PayLink facts the callback is
// normalized against. Amount is the expected on-chain PayLink amount (the proof's amount must equal
// it; the validator cross-checks).
type Record struct {
	PayLinkID  string `json:"paylink_id"`
	Amount     uint64 `json:"amount"`
	Receiver   string `json:"receiver"`    // receiver shortcode (proof receiver) — A.1
	PayerPhone string `json:"payer_phone"` // expected payer MSISDN (informational)
}

// Store correlates charges to PayLinks.
type Store interface {
	// Put records the correlation for a freshly-initiated charge.
	Put(ctx context.Context, checkoutRequestID string, r Record) error
	// Get returns the correlation, or ErrNotFound.
	Get(ctx context.Context, checkoutRequestID string) (Record, error)
}
