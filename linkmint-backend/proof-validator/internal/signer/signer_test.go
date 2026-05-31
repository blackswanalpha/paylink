package signer_test

import (
	"encoding/hex"
	"testing"

	"github.com/paylink/paylink-chain/pkg/lvm"
	"github.com/paylink/proof-validator/internal/signer"
)

const devKey = "b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"

func TestServiceKeySigner_AddressMatchesChain(t *testing.T) {
	s, generated, err := signer.Build("service_key", devKey)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if generated {
		t.Fatal("should not generate when a key is provided")
	}
	key, _ := lvm.PrivateKeyFromHex(devKey)
	if s.Address() != lvm.PrivateKeyToAddress(key) {
		t.Fatalf("address %s != lvm-derived %s", s.Address().Hex(), lvm.PrivateKeyToAddress(key).Hex())
	}
}

func TestServiceKeySigner_SignTxVerifies(t *testing.T) {
	s, _, _ := signer.Build("service_key", devKey)
	tx, _ := lvm.BuildSubmitValidationTx(s.Address(), 0, lvm.SHA256Hash([]byte("pl")), lvm.SHA256Hash([]byte("ph")))
	if err := s.SignTx(tx); err != nil {
		t.Fatalf("SignTx: %v", err)
	}
	if tx.Hash != lvm.SHA256Hash(tx.SignableBytes()) {
		t.Fatal("tx.Hash != SHA256(SignableBytes) — diverges from the chain's server-side recompute")
	}
	key, _ := lvm.PrivateKeyFromHex(devKey)
	if !lvm.Verify(tx.Hash, tx.Signature, &key.PublicKey) {
		t.Fatal("signature does not verify against the signer pubkey")
	}
}

func TestUnsignedSigner_EmptySigButCorrectAddress(t *testing.T) {
	s, _, err := signer.Build("unsigned", devKey)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	key, _ := lvm.PrivateKeyFromHex(devKey)
	if s.Address() != lvm.PrivateKeyToAddress(key) {
		t.Fatal("unsigned signer must still expose the correct From address")
	}
	tx, _ := lvm.BuildSubmitValidationTx(s.Address(), 0, lvm.SHA256Hash([]byte("pl")), lvm.SHA256Hash([]byte("ph")))
	if err := s.SignTx(tx); err != nil {
		t.Fatalf("SignTx: %v", err)
	}
	if len(tx.Signature) != 0 {
		t.Fatalf("unsigned signer must leave an empty signature, got %d bytes", len(tx.Signature))
	}
	if tx.Hash != lvm.SHA256Hash(tx.SignableBytes()) {
		t.Fatal("unsigned signer must still set the correct From-derived hash")
	}
}

func TestBuild_GeneratesWhenNoKey(t *testing.T) {
	s, generated, err := signer.Build("service_key", "")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !generated {
		t.Fatal("expected generated=true when no key is provided")
	}
	if s.Address().IsZero() {
		t.Fatal("generated signer should have a non-zero address")
	}
}

func TestBuild_BadKey(t *testing.T) {
	if _, _, err := signer.Build("service_key", "nothex"); err == nil {
		t.Fatal("expected error for an invalid signer key")
	}
	// A valid-hex but odd input is still parseable to a scalar; ensure a clearly bad hex errors.
	if _, _, err := signer.Build("service_key", hex.EncodeToString([]byte("ok"))+"zz"); err == nil {
		t.Fatal("expected error for invalid hex")
	}
}
