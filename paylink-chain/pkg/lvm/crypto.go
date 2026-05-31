package lvm

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/paylink/paylink-chain/internal/crypto"
)

// ── Hashing (thin re-exports) ──

// SHA256Hash computes SHA-256 (the chain's transaction/proof hash).
func SHA256Hash(data []byte) Hash { return crypto.SHA256Hash(data) }

// Keccak256 computes legacy Keccak-256 (used only for address derivation).
func Keccak256(data []byte) Hash { return crypto.Keccak256(data) }

// ── ECDSA P-256 signing/verification (thin re-exports) ──

// Sign returns the raw r||s (64-byte) ECDSA P-256 signature over hash.
func Sign(hash Hash, key *ecdsa.PrivateKey) ([]byte, error) { return crypto.Sign(hash, key) }

// Verify reports whether sig (raw r||s, 64 bytes) is a valid signature over hash for pub.
func Verify(hash Hash, sig []byte, pub *ecdsa.PublicKey) bool { return crypto.Verify(hash, sig, pub) }

// ── Keys & addresses (thin re-exports) ──

// GenerateKey creates a new P-256 private key.
func GenerateKey() (*ecdsa.PrivateKey, error) { return crypto.GenerateKey() }

// PubkeyToAddress derives the 20-byte address from a public key (last 20 bytes of
// Keccak-256 of the uncompressed pubkey X||Y).
func PubkeyToAddress(pub *ecdsa.PublicKey) Address { return crypto.PubkeyToAddress(pub) }

// PrivateKeyToAddress derives the address from a private key.
func PrivateKeyToAddress(key *ecdsa.PrivateKey) Address { return crypto.PrivateKeyToAddress(key) }

// MarshalPublicKey serializes a public key to uncompressed bytes (65 bytes: 0x04 || X || Y).
func MarshalPublicKey(pub *ecdsa.PublicKey) []byte { return crypto.MarshalPublicKey(pub) }

// UnmarshalPublicKey parses an uncompressed public key (0x04 || X || Y).
func UnmarshalPublicKey(data []byte) (*ecdsa.PublicKey, error) {
	return crypto.UnmarshalPublicKey(data)
}

// UnmarshalPrivateKey parses a private key from its big-endian D scalar bytes.
func UnmarshalPrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	return crypto.UnmarshalPrivateKey(data)
}

// PrivateKeyFromHex parses a private key from a hex string (the big-endian D scalar, with or
// without a 0x prefix) — the same format as `paylinkd --privkey` and the paylink-service signer.
func PrivateKeyFromHex(s string) (*ecdsa.PrivateKey, error) {
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode private key hex: %w", err)
	}
	return crypto.UnmarshalPrivateKey(b)
}

// PublicKeyFromHex parses an uncompressed P-256 public key from a hex string (0x04||X||Y, with or
// without a 0x prefix). Used to load the trusted proof-signer keys the validator verifies against.
func PublicKeyFromHex(s string) (*ecdsa.PublicKey, error) {
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode public key hex: %w", err)
	}
	return crypto.UnmarshalPublicKey(b)
}

// SignTx fills tx.Hash and tx.Signature exactly as the chain's paylink_sendTransaction recomputes
// them: Hash = SHA256(SignableBytes()), Signature = Sign(Hash, key). tx.Payload must already be set
// (via the Build* helpers). The chain does not yet verify the signature (ADR-005), but we sign for
// forward-compatibility and a correct From-derived hash. Idempotent.
func SignTx(tx *Transaction, key *ecdsa.PrivateKey) error {
	h := crypto.SHA256Hash(tx.SignableBytes())
	sig, err := crypto.Sign(h, key)
	if err != nil {
		return fmt.Errorf("sign tx: %w", err)
	}
	tx.Hash = h
	tx.Signature = sig
	return nil
}
