package crypto

import (
	"crypto/ed25519"

	"github.com/paylink/paylink-chain/internal/types"
)

// VRF provides verifiable random function capabilities for validator committee selection.
// Phase 1: StubVRF (deterministic SHA256). Phase 2: ECVRF (ED25519-based).
type VRF interface {
	// Evaluate computes a VRF output and proof for the given input.
	Evaluate(input []byte) (output types.Hash, proof []byte, err error)
	// Verify checks a VRF proof against an input and output.
	Verify(input []byte, output types.Hash, proof []byte) bool
}

// NewVRF creates a real ECVRF instance from an ED25519 private key.
// This is the Phase 2 VRF used for committee selection.
func NewVRF(privateKey ed25519.PrivateKey) (VRF, error) {
	return NewECVRF(privateKey)
}

// StubVRF is a deterministic placeholder for Phase 1 single-validator mode.
type StubVRF struct{}

func (v *StubVRF) Evaluate(input []byte) (types.Hash, []byte, error) {
	output := SHA256Hash(input)
	return output, input, nil
}

func (v *StubVRF) Verify(input []byte, output types.Hash, proof []byte) bool {
	expected := SHA256Hash(input)
	return expected == output
}
