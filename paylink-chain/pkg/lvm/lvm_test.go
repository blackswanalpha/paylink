package lvm

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/types"
)

func TestSubmitValidationWireBytes(t *testing.T) {
	from := HexToAddress("0x0000000000000000000000000000000000000010")
	plID := SHA256Hash([]byte("PLK-1"))
	proofHash := ProofHash(plID, "MPESA-TX-1", 1500)

	tx, err := BuildSubmitValidationTx(from, 7, plID, proofHash)
	if err != nil {
		t.Fatalf("BuildSubmitValidationTx: %v", err)
	}
	if tx.Type != TxSubmitValidation || tx.From != from || tx.Nonce != 7 {
		t.Fatalf("unexpected tx header: %+v", tx)
	}

	// Payload must be exactly the chain's SubmitValidationPayload JSON (compact, 0x-hex hashes).
	wantPayload := fmt.Sprintf(`{"paylinkId":"%s","proofHash":"%s"}`, plID.Hex(), proofHash.Hex())
	if string(tx.Payload) != wantPayload {
		t.Fatalf("payload mismatch:\n got %s\nwant %s", tx.Payload, wantPayload)
	}

	// SignableBytes must be {type,from,nonce,payload} in that order, compact — the bytes the
	// chain hashes server-side and the Python signer reproduces.
	wantSignable := fmt.Sprintf(`{"type":2,"from":"%s","nonce":7,"payload":%s}`, from.Hex(), wantPayload)
	if string(tx.SignableBytes()) != wantSignable {
		t.Fatalf("signable mismatch:\n got %s\nwant %s", tx.SignableBytes(), wantSignable)
	}
}

func TestSignTxMatchesServerFormula(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	from := PrivateKeyToAddress(key)
	plID := SHA256Hash([]byte("PLK-2"))
	tx, err := BuildSubmitValidationTx(from, 0, plID, ProofHash(plID, "tx-2", 42))
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := SignTx(tx, key); err != nil {
		t.Fatalf("SignTx: %v", err)
	}

	// Hash must equal what rpc/handlers.go recomputes: SHA256(SignableBytes()).
	if want := crypto.SHA256Hash(tx.SignableBytes()); tx.Hash != want {
		t.Fatalf("hash mismatch: got %s want %s", tx.Hash.Hex(), want.Hex())
	}
	if len(tx.Signature) != 64 {
		t.Fatalf("signature length: got %d want 64", len(tx.Signature))
	}
	if !Verify(tx.Hash, tx.Signature, &key.PublicKey) {
		t.Fatal("signature did not verify against signer pubkey")
	}
}

func TestProofHashFormatAndDeterminism(t *testing.T) {
	pl := SHA256Hash([]byte("paylink"))

	want := crypto.SHA256Hash([]byte(fmt.Sprintf(`{"paylinkId":"%s","txId":"tx-1","amount":1500}`, pl.Hex())))
	got := ProofHash(pl, "tx-1", 1500)
	if got != want {
		t.Fatalf("ProofHash format mismatch: got %s want %s", got.Hex(), want.Hex())
	}

	// Deterministic.
	if ProofHash(pl, "tx-1", 1500) != got {
		t.Fatal("ProofHash not deterministic")
	}
	// Distinct on amount and on txId.
	if ProofHash(pl, "tx-1", 1501) == got {
		t.Fatal("ProofHash collided on different amount")
	}
	if ProofHash(pl, "tx-2", 1500) == got {
		t.Fatal("ProofHash collided on different txId")
	}
	if ProofHash(SHA256Hash([]byte("other")), "tx-1", 1500) == got {
		t.Fatal("ProofHash collided on different paylink")
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	key, _ := GenerateKey()
	h := SHA256Hash([]byte("message"))
	sig, err := Sign(h, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !Verify(h, sig, &key.PublicKey) {
		t.Fatal("valid signature rejected")
	}
	if Verify(SHA256Hash([]byte("tampered")), sig, &key.PublicKey) {
		t.Fatal("signature verified over wrong hash")
	}
}

func TestPrivateKeyFromHexRoundTrip(t *testing.T) {
	key, _ := GenerateKey()
	d := crypto.MarshalPrivateKey(key) // big-endian D scalar
	parsed, err := PrivateKeyFromHex("0x" + fmt.Sprintf("%x", d))
	if err != nil {
		t.Fatalf("PrivateKeyFromHex: %v", err)
	}
	if PrivateKeyToAddress(parsed) != PrivateKeyToAddress(key) {
		t.Fatal("address mismatch after hex round-trip")
	}
}

func TestPublicKeyFromHexRoundTrip(t *testing.T) {
	key, _ := GenerateKey()
	pubHex := fmt.Sprintf("%x", MarshalPublicKey(&key.PublicKey))
	pub, err := PublicKeyFromHex(pubHex)
	if err != nil {
		t.Fatalf("PublicKeyFromHex: %v", err)
	}
	h := SHA256Hash([]byte("x"))
	sig, _ := Sign(h, key)
	if !Verify(h, sig, pub) {
		t.Fatal("round-tripped pubkey failed to verify a signature from its key")
	}
}

// Guard: the alias really is the internal type (no accidental second definition).
func TestAliasIdentity(t *testing.T) {
	var tx Transaction
	var _ types.Transaction = tx
	var _ json.Marshaler = tx.Hash
	if SHA256Hash([]byte("a")) != crypto.SHA256Hash([]byte("a")) {
		t.Fatal("SHA256Hash wrapper diverged from internal")
	}
}
