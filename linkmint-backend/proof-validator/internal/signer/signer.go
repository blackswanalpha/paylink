// Package signer signs lVM transactions on behalf of the validator. It mirrors the proven
// paylink-service signer (P-256, raw r||s base64) but reuses paylink-chain/pkg/lvm so the wire
// format is byte-exact and never re-derived.
package signer

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/paylink/paylink-chain/pkg/lvm"
)

// Signer signs transactions and exposes the signer's on-chain address (the tx From).
type Signer interface {
	Address() lvm.Address
	SignTx(tx *lvm.Transaction) error
}

// ServiceKeySigner signs with a service-held P-256 key.
type ServiceKeySigner struct {
	key  *ecdsa.PrivateKey
	addr lvm.Address
}

func (s *ServiceKeySigner) Address() lvm.Address             { return s.addr }
func (s *ServiceKeySigner) SignTx(tx *lvm.Transaction) error { return lvm.SignTx(tx, s.key) }

// Build constructs a Signer from config. mode must be "service_key"; keyHex is the P-256 D scalar
// in hex (with or without 0x). When keyHex is empty a key is generated and generated=true is
// returned so the caller can warn (devnet convenience only). The former "unsigned" mode is gone:
// the chain verifies every tx signature (ADR-015), so an unsigned tx is rejected at admission and
// the mode would silently fail all settlements.
func Build(mode, keyHex string) (s Signer, generated bool, err error) {
	if mode != "service_key" {
		return nil, false, fmt.Errorf("signer: unsupported mode %q — the chain enforces tx signatures (ADR-015); use service_key", mode)
	}
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
	return &ServiceKeySigner{key: key, addr: lvm.PrivateKeyToAddress(key)}, generated, nil
}
