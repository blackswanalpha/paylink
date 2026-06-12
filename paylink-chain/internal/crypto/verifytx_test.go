package crypto

import (
	"encoding/json"
	"testing"

	"github.com/paylink/paylink-chain/internal/types"
)

func signedTestTx(t *testing.T) (*types.Transaction, func()) {
	t.Helper()
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	payload, _ := json.Marshal(types.TransferPayload{To: types.Address{0x42}, Amount: 100})
	tx := &types.Transaction{
		Type:    types.TxTransfer,
		From:    PrivateKeyToAddress(key),
		Nonce:   0,
		Payload: payload,
	}
	tx.PubKey = MarshalPublicKey(&key.PublicKey)
	tx.Hash = SHA256Hash(tx.SignableBytes())
	sig, err := Sign(tx.Hash, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	tx.Signature = sig
	return tx, func() {}
}

func TestVerifyTx_Valid(t *testing.T) {
	tx, _ := signedTestTx(t)
	if err := VerifyTx(tx); err != nil {
		t.Fatalf("valid tx rejected: %v", err)
	}
}

func TestVerifyTx_MissingPubKey(t *testing.T) {
	tx, _ := signedTestTx(t)
	tx.PubKey = nil
	if err := VerifyTx(tx); err == nil {
		t.Fatal("tx without pubkey must be rejected")
	}
}

func TestVerifyTx_WrongFrom(t *testing.T) {
	tx, _ := signedTestTx(t)
	tx.From = types.Address{0xDE, 0xAD} // forged sender
	if err := VerifyTx(tx); err == nil {
		t.Fatal("pubkey not deriving From must be rejected")
	}
}

func TestVerifyTx_ForeignKey(t *testing.T) {
	// Attacker signs with their OWN key but claims someone else's From.
	tx, _ := signedTestTx(t)
	victim, _ := signedTestTx(t)
	tx.From = victim.From
	tx.Hash = SHA256Hash(tx.SignableBytes())
	if err := VerifyTx(tx); err == nil {
		t.Fatal("signature from a different key must be rejected")
	}
}

func TestVerifyTx_TamperedPayload(t *testing.T) {
	tx, _ := signedTestTx(t)
	payload, _ := json.Marshal(types.TransferPayload{To: types.Address{0x42}, Amount: 999999})
	tx.Payload = payload
	tx.Hash = SHA256Hash(tx.SignableBytes()) // attacker recomputes the hash too
	if err := VerifyTx(tx); err == nil {
		t.Fatal("tampered payload must invalidate the signature")
	}
}

func TestVerifyTx_TamperedNonce(t *testing.T) {
	tx, _ := signedTestTx(t)
	tx.Nonce = 7
	tx.Hash = SHA256Hash(tx.SignableBytes())
	if err := VerifyTx(tx); err == nil {
		t.Fatal("tampered nonce must invalidate the signature")
	}
}

func TestVerifyTx_SpoofedHash(t *testing.T) {
	tx, _ := signedTestTx(t)
	tx.Hash = SHA256Hash([]byte("some other tx"))
	if err := VerifyTx(tx); err == nil {
		t.Fatal("declared hash not matching signable bytes must be rejected")
	}
}

func TestVerifyTx_EmptySignature(t *testing.T) {
	tx, _ := signedTestTx(t)
	tx.Signature = nil
	if err := VerifyTx(tx); err == nil {
		t.Fatal("unsigned tx must be rejected")
	}
}
