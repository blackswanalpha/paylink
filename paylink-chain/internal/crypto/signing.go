package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/paylink/paylink-chain/internal/types"
)

// Sign signs a message hash with a private key. Returns the signature as r || s (64 bytes).
func Sign(hash types.Hash, key *ecdsa.PrivateKey) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		return nil, fmt.Errorf("sign failed: %w", err)
	}
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	return sig, nil
}

// Verify checks if a signature is valid for a message hash and public key.
func Verify(hash types.Hash, sig []byte, pub *ecdsa.PublicKey) bool {
	if len(sig) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])
	return ecdsa.Verify(pub, hash[:], r, s)
}

// VerifyWithAddress checks if a signature is valid for a message hash and expected address.
func VerifyWithAddress(hash types.Hash, sig []byte, pub *ecdsa.PublicKey, expectedAddr types.Address) bool {
	if !Verify(hash, sig, pub) {
		return false
	}
	addr := PubkeyToAddress(pub)
	return addr == expectedAddr
}

// MarshalPublicKey serializes a public key to uncompressed bytes (65 bytes: 0x04 || X || Y).
func MarshalPublicKey(pub *ecdsa.PublicKey) []byte {
	return elliptic.Marshal(pub.Curve, pub.X, pub.Y)
}

// UnmarshalPublicKey deserializes a public key from uncompressed bytes.
func UnmarshalPublicKey(data []byte) (*ecdsa.PublicKey, error) {
	curve := elliptic.P256()
	x, y := elliptic.Unmarshal(curve, data)
	if x == nil {
		return nil, fmt.Errorf("invalid public key")
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}
