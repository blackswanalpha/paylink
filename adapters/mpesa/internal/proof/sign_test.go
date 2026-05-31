package proof_test

import (
	"testing"

	"github.com/paylink/mpesa-adapter/internal/proof"
	"github.com/paylink/paylink-chain/pkg/lvm"
)

// Well-known devnet adapter key + the matching trusted public key the proof-validator is configured
// with (PROOF_VALIDATOR_TRUSTED_PUBKEYS). Signing with the private key MUST verify against this
// public key — the off-chain trust contract between work04 and work03.
const (
	adapterPrivHex   = "3f7a1c0d9e8b6a5f4d3c2b1a09f8e7d6c5b4a3928170615243f5e6d7c8b9a0f1"
	trustedPubKeyHex = "04e63cbe3984eae5834516e4af2e8e7fa88ce497f68bdcacf95a8fdaf9db4b02efa0ebcb964a1a74ec3d8c748b1e32986788f6c9a4aac39f0b79ac359801a5317d"
)

func TestSignVerifyRoundTrip_TrustedKey(t *testing.T) {
	key, err := lvm.PrivateKeyFromHex(adapterPrivHex)
	if err != nil {
		t.Fatalf("parse adapter key: %v", err)
	}
	pub, err := lvm.PublicKeyFromHex(trustedPubKeyHex)
	if err != nil {
		t.Fatalf("parse trusted pubkey: %v", err)
	}

	p := valid()
	sig, err := proof.Sign(p, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	p.Signature = sig

	// The signature must verify against the validator's trusted key (proves key↔pubkey pairing and
	// byte-exact canonical bytes).
	if !proof.Verify(p, pub) {
		t.Fatal("signature did not verify against the trusted public key")
	}

	// Tamper: changing the amount must invalidate the signature (the signed canonical bytes differ).
	tampered := p
	tampered.Amount = 9999
	if proof.Verify(tampered, pub) {
		t.Fatal("tampered proof unexpectedly verified")
	}
}
