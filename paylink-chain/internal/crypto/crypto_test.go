package crypto

import (
	"testing"

	"github.com/paylink/paylink-chain/internal/types"
)

func TestSHA256Hash(t *testing.T) {
	data := []byte("hello world")
	h1 := SHA256Hash(data)
	h2 := SHA256Hash(data)

	if h1 != h2 {
		t.Fatal("SHA256 not deterministic")
	}
	if h1.IsZero() {
		t.Fatal("SHA256 produced zero hash")
	}
}

func TestKeccak256(t *testing.T) {
	data := []byte("hello world")
	h := Keccak256(data)
	if h.IsZero() {
		t.Fatal("Keccak256 produced zero hash")
	}

	// Different input should produce different hash
	h2 := Keccak256([]byte("hello world!"))
	if h == h2 {
		t.Fatal("Different inputs produced same hash")
	}
}

func TestKeyGenAndAddress(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	addr := PrivateKeyToAddress(key)
	if addr.IsZero() {
		t.Fatal("Address is zero")
	}

	// Same key should produce same address
	addr2 := PubkeyToAddress(&key.PublicKey)
	if addr != addr2 {
		t.Fatal("Address derivation not deterministic")
	}
}

func TestKeyMarshalRoundtrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	data := MarshalPrivateKey(key)
	key2, err := UnmarshalPrivateKey(data)
	if err != nil {
		t.Fatalf("UnmarshalPrivateKey: %v", err)
	}

	if PrivateKeyToAddress(key) != PrivateKeyToAddress(key2) {
		t.Fatal("Key roundtrip produced different address")
	}
}

func TestSignAndVerify(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	msg := SHA256Hash([]byte("test message"))

	sig, err := Sign(msg, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if !Verify(msg, sig, &key.PublicKey) {
		t.Fatal("Valid signature rejected")
	}

	// Tampered message should fail
	tampered := msg
	tampered[0] ^= 0xff
	if Verify(tampered, sig, &key.PublicKey) {
		t.Fatal("Tampered message accepted")
	}
}

func TestVerifyWithAddress(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	addr := PrivateKeyToAddress(key)
	msg := SHA256Hash([]byte("test"))

	sig, err := Sign(msg, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if !VerifyWithAddress(msg, sig, &key.PublicKey, addr) {
		t.Fatal("VerifyWithAddress failed for correct address")
	}

	wrongAddr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	if VerifyWithAddress(msg, sig, &key.PublicKey, wrongAddr) {
		t.Fatal("VerifyWithAddress passed for wrong address")
	}
}

func TestPublicKeyMarshalRoundtrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	pubBytes := MarshalPublicKey(&key.PublicKey)
	pub2, err := UnmarshalPublicKey(pubBytes)
	if err != nil {
		t.Fatalf("UnmarshalPublicKey: %v", err)
	}

	if PubkeyToAddress(&key.PublicKey) != PubkeyToAddress(pub2) {
		t.Fatal("Public key roundtrip produced different address")
	}
}

func TestCombineHashes(t *testing.T) {
	h1 := SHA256Hash([]byte("a"))
	h2 := SHA256Hash([]byte("b"))

	combined := CombineHashes(h1, h2)
	if combined.IsZero() {
		t.Fatal("CombineHashes produced zero hash")
	}

	// Order matters
	reversed := CombineHashes(h2, h1)
	if combined == reversed {
		t.Fatal("CombineHashes is not order-dependent")
	}
}

func TestStubVRF(t *testing.T) {
	vrf := &StubVRF{}

	input := []byte("test input")
	output, proof, err := vrf.Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if !vrf.Verify(input, output, proof) {
		t.Fatal("VRF verification failed")
	}
}
