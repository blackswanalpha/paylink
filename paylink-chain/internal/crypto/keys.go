package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/paylink/paylink-chain/internal/types"
)

// GenerateKey creates a new ECDSA P-256 private key.
func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// PubkeyToAddress derives a 20-byte address from an ECDSA public key.
// Address = last 20 bytes of Keccak256(uncompressed pubkey X || Y).
func PubkeyToAddress(pub *ecdsa.PublicKey) types.Address {
	pubBytes := elliptic.Marshal(pub.Curve, pub.X, pub.Y)
	// Skip the 0x04 prefix byte
	hash := Keccak256(pubBytes[1:])
	return types.BytesToAddress(hash[12:])
}

// PrivateKeyToAddress derives the address from a private key.
func PrivateKeyToAddress(key *ecdsa.PrivateKey) types.Address {
	return PubkeyToAddress(&key.PublicKey)
}

// MarshalPrivateKey serializes a private key to bytes.
func MarshalPrivateKey(key *ecdsa.PrivateKey) []byte {
	return key.D.Bytes()
}

// UnmarshalPrivateKey deserializes a private key from bytes.
func UnmarshalPrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	curve := elliptic.P256()
	key := new(ecdsa.PrivateKey)
	key.D = new(big.Int).SetBytes(data)
	key.PublicKey.Curve = curve
	key.PublicKey.X, key.PublicKey.Y = curve.ScalarBaseMult(data)
	if key.PublicKey.X == nil {
		return nil, fmt.Errorf("invalid private key")
	}
	return key, nil
}
