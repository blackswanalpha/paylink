package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"

	"github.com/paylink/paylink-chain/internal/types"
)

// ECVRF implements a Verifiable Random Function using ED25519 deterministic signatures.
//
// Construction: ED25519 signatures are deterministic per RFC 8032 -- signing the same
// input with the same key always produces the same signature. This gives us:
//   - Output = SHA256(signature) -- deterministic, unpredictable without the private key
//   - Proof  = the ED25519 signature itself -- verifiable by anyone with the public key
//
// Security: relies on the unforgeability of ED25519. An adversary cannot produce a valid
// proof (signature) without the private key, and cannot predict the output without it.
type ECVRF struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// NewECVRF creates a new ECVRF instance from an ED25519 private key.
// The private key must be 64 bytes (ed25519.PrivateKeySize).
func NewECVRF(privateKey ed25519.PrivateKey) (*ECVRF, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ED25519 private key length: got %d, want %d", len(privateKey), ed25519.PrivateKeySize)
	}
	return &ECVRF{
		privateKey: privateKey,
		publicKey:  privateKey.Public().(ed25519.PublicKey),
	}, nil
}

// Evaluate computes a VRF output and proof for the given input.
// The output is deterministic: same key + same input always produces the same output.
// The proof is an ED25519 signature that anyone with the public key can verify.
func (v *ECVRF) Evaluate(input []byte) (types.Hash, []byte, error) {
	// Tag the input to domain-separate VRF usage from other ED25519 signing
	tagged := vrfTagInput(input)

	// Sign: deterministic per RFC 8032
	signature := ed25519.Sign(v.privateKey, tagged)

	// Output: hash of signature to produce uniform 32-byte output
	output := sha256.Sum256(signature)

	// Proof: public key (32 bytes) || signature (64 bytes) = 96 bytes
	proof := make([]byte, ed25519.PublicKeySize+ed25519.SignatureSize)
	copy(proof[:ed25519.PublicKeySize], v.publicKey)
	copy(proof[ed25519.PublicKeySize:], signature)

	return output, proof, nil
}

// Verify checks a VRF proof against an input and expected output.
// This is a static operation -- it only needs the proof (which embeds the public key).
func (v *ECVRF) Verify(input []byte, output types.Hash, proof []byte) bool {
	return VerifyVRFProof(input, output, proof)
}

// PublicKey returns the VRF public key (ED25519, 32 bytes).
func (v *ECVRF) PublicKey() ed25519.PublicKey {
	return v.publicKey
}

// VerifyVRFProof is a standalone verification function that does not require the ECVRF
// instance. Any node can verify a VRF proof using only the proof bytes.
func VerifyVRFProof(input []byte, output types.Hash, proof []byte) bool {
	expectedLen := ed25519.PublicKeySize + ed25519.SignatureSize
	if len(proof) != expectedLen {
		return false
	}

	pubKey := ed25519.PublicKey(proof[:ed25519.PublicKeySize])
	signature := proof[ed25519.PublicKeySize:]

	// Verify signature
	tagged := vrfTagInput(input)
	if !ed25519.Verify(pubKey, tagged, signature) {
		return false
	}

	// Verify output matches SHA256(signature)
	expectedOutput := sha256.Sum256(signature)
	return output == expectedOutput
}

// VRFProofPublicKey extracts the public key from a VRF proof.
func VRFProofPublicKey(proof []byte) (ed25519.PublicKey, error) {
	expectedLen := ed25519.PublicKeySize + ed25519.SignatureSize
	if len(proof) != expectedLen {
		return nil, fmt.Errorf("invalid proof length: got %d, want %d", len(proof), expectedLen)
	}
	pubKey := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(pubKey, proof[:ed25519.PublicKeySize])
	return pubKey, nil
}

// vrfTagInput domain-separates VRF inputs from other ED25519 uses.
func vrfTagInput(input []byte) []byte {
	tag := []byte("LinkMint-VRF-v1:")
	tagged := make([]byte, len(tag)+len(input))
	copy(tagged, tag)
	copy(tagged[len(tag):], input)
	return tagged
}
