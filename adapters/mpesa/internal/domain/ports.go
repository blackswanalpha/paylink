// Package domain orchestrates the adapter pipeline: receive (a rail-neutral callback from the Node
// rail service) → normalize (to the proof shape) → sign → broadcast (to the proof-validator). It
// holds no funds and never settles (A.1/A.3); it only proves a payment happened. Dependencies are
// expressed as ports so the pipeline is unit-testable with fakes.
package domain

import (
	"context"

	"github.com/paylink/mpesa-adapter/internal/broadcast"
	"github.com/paylink/mpesa-adapter/internal/daraja"
	"github.com/paylink/mpesa-adapter/internal/proof"
)

// RailClient initiates an STK push via the Node Daraja rail service.
type RailClient interface {
	STKPush(ctx context.Context, p daraja.STKPushParams) (daraja.STKPushResult, error)
}

// Signer signs a proof and returns its base64 proof_signature.
type Signer interface {
	Sign(p proof.Proof) (string, error)
}

// Broadcaster submits a signed proof to the proof-validator.
type Broadcaster interface {
	Broadcast(ctx context.Context, p proof.Proof, idemKey string) (broadcast.Result, error)
}

// Metrics is an optional hook (nil-safe).
type Metrics interface {
	ChargeInitiated(result string)
	CallbackReceived(result string)
	ProofBroadcast(result string)
}
