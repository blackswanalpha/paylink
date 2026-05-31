package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/paylink/paylink-chain/internal/types"
)

func generateTestVRF(t *testing.T) *ECVRF {
	t.Helper()
	_, priv, err := GenerateVRFKey()
	if err != nil {
		t.Fatalf("generate VRF key: %v", err)
	}
	vrf, err := NewECVRF(priv)
	if err != nil {
		t.Fatalf("new ECVRF: %v", err)
	}
	return vrf
}

func TestECVRF_NewECVRF_ValidKey(t *testing.T) {
	_, priv, err := GenerateVRFKey()
	if err != nil {
		t.Fatal(err)
	}
	vrf, err := NewECVRF(priv)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if vrf == nil {
		t.Fatal("expected non-nil ECVRF")
	}
	if len(vrf.PublicKey()) != ed25519.PublicKeySize {
		t.Fatalf("expected %d byte public key, got %d", ed25519.PublicKeySize, len(vrf.PublicKey()))
	}
}

func TestECVRF_NewECVRF_InvalidKey(t *testing.T) {
	_, err := NewECVRF([]byte("too-short"))
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}

func TestECVRF_Determinism(t *testing.T) {
	vrf := generateTestVRF(t)
	input := []byte("test-input-determinism")

	output1, proof1, err := vrf.Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	output2, proof2, err := vrf.Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}

	if output1 != output2 {
		t.Errorf("VRF not deterministic: %s != %s", output1.Hex(), output2.Hex())
	}
	if !bytes.Equal(proof1, proof2) {
		t.Error("proofs not deterministic")
	}
}

func TestECVRF_Determinism_1000Inputs(t *testing.T) {
	vrf := generateTestVRF(t)

	for i := 0; i < 1000; i++ {
		input := make([]byte, 32)
		rand.Read(input)

		out1, _, err := vrf.Evaluate(input)
		if err != nil {
			t.Fatal(err)
		}
		out2, _, err := vrf.Evaluate(input)
		if err != nil {
			t.Fatal(err)
		}
		if out1 != out2 {
			t.Fatalf("non-deterministic at iteration %d", i)
		}
	}
}

func TestECVRF_Uniqueness(t *testing.T) {
	vrf := generateTestVRF(t)

	outputs := make(map[types.Hash]bool)
	for i := 0; i < 100; i++ {
		input := make([]byte, 32)
		rand.Read(input)

		output, _, err := vrf.Evaluate(input)
		if err != nil {
			t.Fatal(err)
		}
		if outputs[output] {
			t.Fatalf("duplicate output at iteration %d", i)
		}
		outputs[output] = true
	}
}

func TestECVRF_DifferentKeysDifferentOutputs(t *testing.T) {
	vrf1 := generateTestVRF(t)
	vrf2 := generateTestVRF(t)
	input := []byte("same-input-different-keys")

	out1, _, _ := vrf1.Evaluate(input)
	out2, _, _ := vrf2.Evaluate(input)

	if out1 == out2 {
		t.Error("different keys produced same output")
	}
}

func TestECVRF_VerifyValid(t *testing.T) {
	vrf := generateTestVRF(t)
	input := []byte("test-verify")

	output, proof, err := vrf.Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}

	if !vrf.Verify(input, output, proof) {
		t.Error("verify failed for valid proof")
	}
}

func TestECVRF_VerifyValid_1000Inputs(t *testing.T) {
	vrf := generateTestVRF(t)

	for i := 0; i < 1000; i++ {
		input := make([]byte, 32)
		rand.Read(input)

		output, proof, err := vrf.Evaluate(input)
		if err != nil {
			t.Fatal(err)
		}
		if !vrf.Verify(input, output, proof) {
			t.Fatalf("verify failed at iteration %d", i)
		}
	}
}

func TestECVRF_VerifyStandalone(t *testing.T) {
	vrf := generateTestVRF(t)
	input := []byte("test-standalone-verify")

	output, proof, err := vrf.Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}

	// VerifyVRFProof is a standalone function -- doesn't need the ECVRF instance
	if !VerifyVRFProof(input, output, proof) {
		t.Error("standalone verify failed for valid proof")
	}
}

func TestECVRF_VerifyRejectsTamperedProof(t *testing.T) {
	vrf := generateTestVRF(t)
	input := []byte("test-tamper")

	output, proof, _ := vrf.Evaluate(input)

	// Tamper with the signature portion of the proof
	tampered := make([]byte, len(proof))
	copy(tampered, proof)
	tampered[ed25519.PublicKeySize+10] ^= 0xFF

	if vrf.Verify(input, output, tampered) {
		t.Error("verify should reject tampered proof")
	}
}

func TestECVRF_VerifyRejectsTamperedOutput(t *testing.T) {
	vrf := generateTestVRF(t)
	input := []byte("test-tamper-output")

	output, proof, _ := vrf.Evaluate(input)

	// Tamper with the output
	output[0] ^= 0xFF

	if vrf.Verify(input, output, proof) {
		t.Error("verify should reject tampered output")
	}
}

func TestECVRF_VerifyRejectsWrongInput(t *testing.T) {
	vrf := generateTestVRF(t)

	output, proof, _ := vrf.Evaluate([]byte("input-A"))

	if vrf.Verify([]byte("input-B"), output, proof) {
		t.Error("verify should reject wrong input")
	}
}

func TestECVRF_VerifyRejectsWrongPublicKey(t *testing.T) {
	vrf1 := generateTestVRF(t)
	vrf2 := generateTestVRF(t)
	input := []byte("test-wrong-key")

	output, proof, _ := vrf1.Evaluate(input)

	// Replace public key in proof with vrf2's key
	tampered := make([]byte, len(proof))
	copy(tampered, proof)
	copy(tampered[:ed25519.PublicKeySize], vrf2.PublicKey())

	if VerifyVRFProof(input, output, tampered) {
		t.Error("verify should reject proof with wrong public key")
	}
}

func TestECVRF_VerifyRejectsTruncatedProof(t *testing.T) {
	vrf := generateTestVRF(t)
	input := []byte("test-truncated")

	output, proof, _ := vrf.Evaluate(input)

	if vrf.Verify(input, output, proof[:len(proof)-1]) {
		t.Error("verify should reject truncated proof")
	}
}

func TestECVRF_VerifyRejectsEmptyProof(t *testing.T) {
	vrf := generateTestVRF(t)
	if vrf.Verify([]byte("input"), types.Hash{}, nil) {
		t.Error("verify should reject nil proof")
	}
	if vrf.Verify([]byte("input"), types.Hash{}, []byte{}) {
		t.Error("verify should reject empty proof")
	}
}

func TestECVRF_ProofPublicKeyExtraction(t *testing.T) {
	vrf := generateTestVRF(t)
	_, proof, _ := vrf.Evaluate([]byte("test-extract"))

	extracted, err := VRFProofPublicKey(proof)
	if err != nil {
		t.Fatalf("extract public key: %v", err)
	}
	if !bytes.Equal(extracted, vrf.PublicKey()) {
		t.Error("extracted public key doesn't match")
	}
}

func TestECVRF_ProofPublicKeyExtractionInvalid(t *testing.T) {
	_, err := VRFProofPublicKey([]byte("short"))
	if err == nil {
		t.Error("expected error for invalid proof")
	}
}

func TestECVRF_OutputDistribution(t *testing.T) {
	// Verify outputs have reasonable uniformity -- check each byte position
	// has both high and low values across many evaluations
	vrf := generateTestVRF(t)
	hasHigh := [32]bool{}
	hasLow := [32]bool{}

	for i := 0; i < 500; i++ {
		input := make([]byte, 32)
		rand.Read(input)

		output, _, _ := vrf.Evaluate(input)
		for j := 0; j < 32; j++ {
			if output[j] >= 128 {
				hasHigh[j] = true
			} else {
				hasLow[j] = true
			}
		}
	}

	for j := 0; j < 32; j++ {
		if !hasHigh[j] || !hasLow[j] {
			t.Errorf("byte position %d has poor distribution", j)
		}
	}
}

func TestECVRF_FuzzMutatedProofs(t *testing.T) {
	vrf := generateTestVRF(t)
	input := []byte("fuzz-test-input")
	output, proof, _ := vrf.Evaluate(input)

	// Mutate each byte of the proof and verify it's rejected
	for i := 0; i < len(proof); i++ {
		mutated := make([]byte, len(proof))
		copy(mutated, proof)
		mutated[i] ^= 0x01

		if VerifyVRFProof(input, output, mutated) {
			t.Errorf("verify accepted mutated proof at byte %d", i)
		}
	}
}

func BenchmarkECVRF_Evaluate(b *testing.B) {
	_, priv, _ := GenerateVRFKey()
	vrf, _ := NewECVRF(priv)
	input := []byte("benchmark-input")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vrf.Evaluate(input)
	}
}

func BenchmarkECVRF_Verify(b *testing.B) {
	_, priv, _ := GenerateVRFKey()
	vrf, _ := NewECVRF(priv)
	input := []byte("benchmark-input")
	output, proof, _ := vrf.Evaluate(input)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VerifyVRFProof(input, output, proof)
	}
}
