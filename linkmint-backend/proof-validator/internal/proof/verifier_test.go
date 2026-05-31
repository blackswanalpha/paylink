package proof_test

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/paylink/paylink-chain/pkg/lvm"
	"github.com/paylink/proof-validator/internal/proof"
)

func pubHex(k *ecdsa.PrivateKey) string {
	return hex.EncodeToString(lvm.MarshalPublicKey(&k.PublicKey))
}

// sign produces a valid proof_signature over p's canonical bytes using key.
func sign(t *testing.T, p proof.Proof, key *ecdsa.PrivateKey) string {
	t.Helper()
	sig, err := lvm.Sign(lvm.SHA256Hash(proof.CanonicalBytes(p)), key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func TestVerify_Valid(t *testing.T) {
	key, _ := lvm.GenerateKey()
	v, err := proof.NewVerifier([]string{pubHex(key)})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	p := valid()
	p.Signature = sign(t, p, key)
	if err := v.Verify(p); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerify_Tampered(t *testing.T) {
	key, _ := lvm.GenerateKey()
	v, _ := proof.NewVerifier([]string{pubHex(key)})
	p := valid()
	p.Signature = sign(t, p, key)
	p.Amount = 9999 // tamper after signing → canonical bytes change
	if err := v.Verify(p); err == nil {
		t.Fatal("tampered proof must not verify")
	}
}

func TestVerify_UntrustedKey(t *testing.T) {
	signing, _ := lvm.GenerateKey()
	trusted, _ := lvm.GenerateKey()
	v, _ := proof.NewVerifier([]string{pubHex(trusted)})
	p := valid()
	p.Signature = sign(t, p, signing) // signed by an untrusted key
	if err := v.Verify(p); err == nil {
		t.Fatal("proof signed by an untrusted key must not verify")
	}
}

func TestVerify_MultipleTrustedKeys(t *testing.T) {
	k1, _ := lvm.GenerateKey()
	k2, _ := lvm.GenerateKey()
	v, _ := proof.NewVerifier([]string{pubHex(k1), pubHex(k2)})
	p := valid()
	p.Signature = sign(t, p, k2) // matches the second trusted key
	if err := v.Verify(p); err != nil {
		t.Fatalf("proof signed by a trusted key rejected: %v", err)
	}
}

func TestVerify_BadSignatureEncoding(t *testing.T) {
	key, _ := lvm.GenerateKey()
	v, _ := proof.NewVerifier([]string{pubHex(key)})
	p := valid()
	p.Signature = "!!!not base64!!!"
	if err := v.Verify(p); err == nil {
		t.Fatal("malformed signature must be rejected")
	}
}

func TestVerify_FailsClosedWithNoKeys(t *testing.T) {
	v, err := proof.NewVerifier(nil)
	if err != nil {
		t.Fatalf("NewVerifier(nil): %v", err)
	}
	if v.TrustedCount() != 0 {
		t.Fatalf("trusted count = %d, want 0", v.TrustedCount())
	}
	key, _ := lvm.GenerateKey()
	p := valid()
	p.Signature = sign(t, p, key)
	if err := v.Verify(p); err == nil {
		t.Fatal("with no trusted keys, every proof must be rejected (fail-closed)")
	}
}

func TestNewVerifier_BadKey(t *testing.T) {
	if _, err := proof.NewVerifier([]string{"not-hex"}); err == nil {
		t.Fatal("expected error for an invalid trusted pubkey")
	}
	if _, err := proof.NewVerifier([]string{strings.Repeat("00", 10)}); err == nil {
		t.Fatal("expected error for a too-short pubkey")
	}
}
