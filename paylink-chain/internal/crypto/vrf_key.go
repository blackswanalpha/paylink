package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/paylink/paylink-chain/internal/types"
)

// GenerateVRFKey creates a new ED25519 key pair for VRF operations.
func GenerateVRFKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate VRF key: %w", err)
	}
	return pub, priv, nil
}

// MarshalVRFPrivateKey serializes a VRF private key to bytes.
func MarshalVRFPrivateKey(key ed25519.PrivateKey) []byte {
	out := make([]byte, len(key))
	copy(out, key)
	return out
}

// UnmarshalVRFPrivateKey deserializes a VRF private key from bytes.
func UnmarshalVRFPrivateKey(data []byte) (ed25519.PrivateKey, error) {
	if len(data) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid VRF private key length: got %d, want %d", len(data), ed25519.PrivateKeySize)
	}
	key := make(ed25519.PrivateKey, ed25519.PrivateKeySize)
	copy(key, data)
	return key, nil
}

// MarshalVRFPublicKey serializes a VRF public key to bytes.
func MarshalVRFPublicKey(pub ed25519.PublicKey) []byte {
	out := make([]byte, len(pub))
	copy(out, pub)
	return out
}

// UnmarshalVRFPublicKey deserializes a VRF public key from bytes.
func UnmarshalVRFPublicKey(data []byte) (ed25519.PublicKey, error) {
	if len(data) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid VRF public key length: got %d, want %d", len(data), ed25519.PublicKeySize)
	}
	pub := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(pub, data)
	return pub, nil
}

// VRFPublicKeyToAddress derives a types.Address from a VRF public key.
// Uses Keccak256(pubKey)[12:] for a 20-byte address.
func VRFPublicKeyToAddress(pub ed25519.PublicKey) types.Address {
	hash := Keccak256(pub)
	return types.BytesToAddress(hash[12:])
}

// SaveVRFKey writes a VRF private key to a file in hex encoding.
func SaveVRFKey(path string, key ed25519.PrivateKey) error {
	encoded := hex.EncodeToString(key)
	return os.WriteFile(path, []byte(encoded), 0600)
}

// LoadVRFKey reads a VRF private key from a hex-encoded file.
func LoadVRFKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read VRF key file: %w", err)
	}
	keyBytes, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("decode VRF key hex: %w", err)
	}
	return UnmarshalVRFPrivateKey(keyBytes)
}

// LoadOrGenerateVRFKey loads a VRF key from the given path, or generates and saves a new one.
func LoadOrGenerateVRFKey(path string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	key, err := LoadVRFKey(path)
	if err == nil {
		pub := key.Public().(ed25519.PublicKey)
		return key, pub, nil
	}

	pub, priv, err := GenerateVRFKey()
	if err != nil {
		return nil, nil, err
	}

	if err := SaveVRFKey(path, priv); err != nil {
		return nil, nil, fmt.Errorf("save VRF key: %w", err)
	}

	return priv, pub, nil
}
