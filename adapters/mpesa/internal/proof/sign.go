package proof

import (
	"crypto/ecdsa"
	"encoding/base64"

	"github.com/paylink/paylink-chain/pkg/lvm"
)

// Sign produces proof_signature: base64(P-256 r||s) over SHA256(CanonicalBytes(p)). It uses
// paylink-chain/pkg/lvm so the curve, hash, and encoding are byte-exact with the lVM tx signer and
// what the validator verifies. The same sequence the work03 e2e test uses.
func Sign(p Proof, key *ecdsa.PrivateKey) (string, error) {
	sig, err := lvm.Sign(lvm.SHA256Hash(CanonicalBytes(p)), key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verify reports whether p.Signature verifies over CanonicalBytes(p) against pub. Used in tests to
// prove the adapter's signature is acceptable to a validator that trusts pub.
func Verify(p Proof, pub *ecdsa.PublicKey) bool {
	sig, err := base64.StdEncoding.DecodeString(p.Signature)
	if err != nil || len(sig) != 64 {
		return false
	}
	return lvm.Verify(lvm.SHA256Hash(CanonicalBytes(p)), sig, pub)
}
