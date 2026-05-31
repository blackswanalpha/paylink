// Package signer holds the adapter's P-256 signing key and signs payment proofs. Its public key
// must be present in the proof-validator's PROOF_VALIDATOR_TRUSTED_PUBKEYS or every proof is
// rejected. It reuses paylink-chain/pkg/lvm so the curve/hash/encoding are byte-exact.
package signer

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	"github.com/paylink/mpesa-adapter/internal/proof"
	"github.com/paylink/paylink-chain/pkg/lvm"
)

// Signer signs proofs with a service-held P-256 key.
type Signer struct {
	key    *ecdsa.PrivateKey
	addr   lvm.Address
	pubHex string
}

// Load builds a Signer from the D-scalar hex key (with or without 0x). When keyHex is empty a key
// is generated and generated=true is returned so the caller can warn (devnet convenience only — a
// generated key won't be in the validator's trusted set).
func Load(keyHex string) (s *Signer, generated bool, err error) {
	var key *ecdsa.PrivateKey
	if keyHex == "" {
		key, err = lvm.GenerateKey()
		generated = true
	} else {
		key, err = lvm.PrivateKeyFromHex(keyHex)
	}
	if err != nil {
		return nil, false, fmt.Errorf("load signer key: %w", err)
	}
	return &Signer{
		key:    key,
		addr:   lvm.PrivateKeyToAddress(key),
		pubHex: hex.EncodeToString(lvm.MarshalPublicKey(&key.PublicKey)),
	}, generated, nil
}

// Sign signs the proof and returns its base64 proof_signature.
func (s *Signer) Sign(p proof.Proof) (string, error) { return proof.Sign(p, s.key) }

// Address is the adapter's on-chain address (derived from the signing key; informational).
func (s *Signer) Address() lvm.Address { return s.addr }

// PubKeyHex is the uncompressed P-256 public key (0x04||X||Y) hex — log it at boot so an operator
// can confirm it matches the validator's PROOF_VALIDATOR_TRUSTED_PUBKEYS.
func (s *Signer) PubKeyHex() string { return s.pubHex }
