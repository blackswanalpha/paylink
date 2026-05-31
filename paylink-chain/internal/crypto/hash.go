package crypto

import (
	"crypto/sha256"

	"github.com/paylink/paylink-chain/internal/types"
	"golang.org/x/crypto/sha3"
)

// SHA256Hash computes SHA-256 of the input data.
func SHA256Hash(data []byte) types.Hash {
	return sha256.Sum256(data)
}

// Keccak256 computes Keccak-256 of the input data.
func Keccak256(data []byte) types.Hash {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	var out types.Hash
	h.Sum(out[:0])
	return out
}

// CombineHashes computes SHA-256 of the concatenation of multiple hashes.
func CombineHashes(hashes ...types.Hash) types.Hash {
	var combined []byte
	for _, h := range hashes {
		combined = append(combined, h[:]...)
	}
	return SHA256Hash(combined)
}
