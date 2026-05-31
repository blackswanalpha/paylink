package proof

import (
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"

	"github.com/paylink/paylink-chain/pkg/lvm"
	"github.com/paylink/proof-validator/internal/httpx"
)

// Verifier checks a proof's signature against a set of trusted P-256 public keys (the adapter
// signing keys). It is the off-chain trust anchor: the chain does not verify the proof signature,
// so a proof is only worth broadcasting if it was signed by a key we trust.
type Verifier struct {
	keys []*ecdsa.PublicKey
}

// NewVerifier parses the trusted public keys (uncompressed P-256 hex, 0x04||X||Y). An empty set
// is allowed but fails closed — every proof will be rejected (callers should warn at boot).
func NewVerifier(hexKeys []string) (*Verifier, error) {
	keys := make([]*ecdsa.PublicKey, 0, len(hexKeys))
	for _, h := range hexKeys {
		k, err := lvm.PublicKeyFromHex(h)
		if err != nil {
			return nil, fmt.Errorf("trusted proof pubkey %q: %w", h, err)
		}
		keys = append(keys, k)
	}
	return &Verifier{keys: keys}, nil
}

// TrustedCount returns the number of configured trusted keys.
func (v *Verifier) TrustedCount() int { return len(v.keys) }

// Verify checks p.Signature over CanonicalBytes(p) against the trusted keys; nil means accepted.
// Returns INVALID_PROOF_SIGNATURE if the signature is malformed or matches no trusted key.
func (v *Verifier) Verify(p Proof) error {
	sig, err := base64.StdEncoding.DecodeString(p.Signature)
	if err != nil || len(sig) != 64 {
		return httpx.NewError(httpx.CodeInvalidProofSignature, "proof_signature is not a valid 64-byte signature", nil)
	}
	digest := lvm.SHA256Hash(CanonicalBytes(p))
	for _, k := range v.keys {
		if lvm.Verify(digest, sig, k) {
			return nil
		}
	}
	return httpx.NewError(httpx.CodeInvalidProofSignature, "proof_signature did not verify against any trusted key", nil)
}
