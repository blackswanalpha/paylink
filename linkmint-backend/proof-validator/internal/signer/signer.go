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

// UnsignedSigner supplies a correct From-derived hash but an empty signature. Valid because the
// chain does not verify tx signatures yet (ADR-005); lets a deployment run before a key is issued.
type UnsignedSigner struct {
	addr lvm.Address
}

func (u *UnsignedSigner) Address() lvm.Address { return u.addr }

func (u *UnsignedSigner) SignTx(tx *lvm.Transaction) error {
	tx.Hash = lvm.SHA256Hash(tx.SignableBytes())
	tx.Signature = []byte{}
	return nil
}

// Build constructs a Signer from config. mode is "service_key" or "unsigned"; keyHex is the P-256
// D scalar in hex (with or without 0x). When keyHex is empty a key is generated and generated=true
// is returned so the caller can warn (devnet convenience only).
func Build(mode, keyHex string) (s Signer, generated bool, err error) {
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
	addr := lvm.PrivateKeyToAddress(key)
	if mode == "unsigned" {
		return &UnsignedSigner{addr: addr}, generated, nil
	}
	return &ServiceKeySigner{key: key, addr: addr}, generated, nil
}
