//go:build e2e

// Package e2e drives the real stack (docker compose --profile e2e: postgres + redis +
// paylink-chain + proof-validator) and proves the work03 acceptance criteria end-to-end:
// a valid proof settles a PayLink on-chain, a tampered proof is rejected with nothing broadcast,
// and an already-settled proof is not re-broadcast (A.7).
//
//	Run: docker compose --profile e2e up -d --build && \
//	     cd linkmint-backend/proof-validator && go test -tags=e2e ./test/... -v
package e2e

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/pkg/lvm"
	"github.com/paylink/proof-validator/internal/proof"
)

// Devnet keys (well-known, see genesis.devnet.json / .env.example).
const (
	merchantKey = "1a2b3c4d5e6f70819293a4b5c6d7e8f9000102030405060708090a0b0c0d0e0f"
	adapterKey  = "3f7a1c0d9e8b6a5f4d3c2b1a09f8e7d6c5b4a3928170615243f5e6d7c8b9a0f1"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func chainURL() string { return env("CHAIN_RPC_URL", "http://localhost:8545/") }
func svcURL() string   { return env("PROOF_VALIDATOR_URL", "http://localhost:8081") }

// rpc issues a JSON-RPC 2.0 call and decodes result into out (out may be nil).
func rpc(t *testing.T, method string, params, out any) error {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params, "id": 1})
	resp, err := http.Post(chainURL(), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var rr struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &rr); err != nil {
		return fmt.Errorf("decode rpc response %q: %w", raw, err)
	}
	if rr.Error != nil {
		return fmt.Errorf("rpc %s error: %s", method, rr.Error.Message)
	}
	if out != nil && len(rr.Result) > 0 {
		return json.Unmarshal(rr.Result, out)
	}
	return nil
}

func mustKey(t *testing.T, hexKey string) *ecdsa.PrivateKey {
	t.Helper()
	k, err := lvm.PrivateKeyFromHex(hexKey)
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}
	return k
}

// createPayLink submits a TxCreatePayLink from the merchant and returns the PayLink id (hex).
func createPayLink(t *testing.T, amount uint64) (string, lvm.Hash) {
	t.Helper()
	mk := mustKey(t, merchantKey)
	from := lvm.PrivateKeyToAddress(mk)

	var nonce uint64
	if err := rpc(t, "paylink_getNonce", map[string]string{"address": from.Hex()}, &nonce); err != nil {
		t.Fatalf("getNonce: %v", err)
	}

	plID := lvm.SHA256Hash([]byte(fmt.Sprintf("e2e-pl-%d", time.Now().UnixNano())))
	tx, err := lvm.BuildCreatePayLinkTx(from, nonce, lvm.CreatePayLinkPayload{
		PayLinkID: plID,
		Receiver:  from, // any non-zero address
		Amount:    amount,
		Expiry:    time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("build create: %v", err)
	}
	if err := lvm.SignTx(tx, mk); err != nil {
		t.Fatalf("sign create: %v", err)
	}
	if err := rpc(t, "paylink_sendTransaction", tx, nil); err != nil {
		t.Fatalf("send create: %v", err)
	}

	// Wait for the PayLink to appear as CREATED.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var pl struct {
			Status string `json:"status"`
		}
		if err := rpc(t, "paylink_getPayLink", map[string]string{"id": plID.Hex()}, &pl); err == nil && pl.Status == "CREATED" {
			return plID.Hex(), plID
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("paylink %s never reached CREATED", plID.Hex())
	return "", lvm.Hash{}
}

// signedProof builds a proof for plID/amount signed by the adapter key.
func signedProof(t *testing.T, plID string, amount uint64) proof.Proof {
	t.Helper()
	p := proof.Proof{
		PayLinkID: plID,
		Rail:      "mpesa",
		TxID:      fmt.Sprintf("MPESA-%d", time.Now().UnixNano()),
		Amount:    amount,
		Timestamp: time.Now().Unix(),
		Sender:    "254700000000",
		Receiver:  "254711111111",
	}
	sig, err := lvm.Sign(lvm.SHA256Hash(proof.CanonicalBytes(p)), mustKey(t, adapterKey))
	if err != nil {
		t.Fatalf("sign proof: %v", err)
	}
	p.Signature = base64.StdEncoding.EncodeToString(sig)
	return p
}

// postProof POSTs a proof (optionally mutating amount on the wire to simulate tampering) and
// returns the HTTP status and decoded body.
func postProof(t *testing.T, p proof.Proof, wireAmount uint64, idemKey string) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"pl_id": p.PayLinkID, "rail": p.Rail, "tx_id": p.TxID, "amount": wireAmount,
		"timestamp": p.Timestamp, "sender": p.Sender, "receiver": p.Receiver, "proof_signature": p.Signature,
	})
	req, _ := http.NewRequest(http.MethodPost, svcURL()+"/v1/proofs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idemKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/proofs: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(svcURL() + "/internal/readyz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatal("proof-validator never became ready (auto-stake / validator active?)")
}

func TestE2E_ValidProofSettles(t *testing.T) {
	waitReady(t)
	plHex, plID := createPayLink(t, 1500)
	p := signedProof(t, plHex, 1500)

	status, body := postProof(t, p, 1500, "e2e-valid-"+p.TxID)
	if status != http.StatusAccepted {
		t.Fatalf("POST status = %d, want 202; body=%v", status, body)
	}
	wantPH := lvm.ProofHash(plID, p.TxID, p.Amount).Hex()
	if body["proof_hash"] != wantPH {
		t.Fatalf("proof_hash = %v, want %s", body["proof_hash"], wantPH)
	}
	if body["tx_hash"] == "" || body["status"] != "broadcast" {
		t.Fatalf("unexpected body: %v", body)
	}

	// The chain settles at quorum (1 validator) — poll for VERIFIED + proof used.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var pl struct {
			Status string `json:"status"`
		}
		_ = rpc(t, "paylink_getPayLink", map[string]string{"id": plHex}, &pl)
		if pl.Status == "VERIFIED" {
			var used bool
			_ = rpc(t, "paylink_isProofUsed", map[string]string{"proofHash": wantPH}, &used)
			if !used {
				t.Fatal("PayLink VERIFIED but proof not marked used")
			}
			// Replay: the same proof must not re-broadcast (A.7).
			rs, rb := postProof(t, p, 1500, "e2e-replay-"+p.TxID)
			if rs != http.StatusOK || rb["status"] != "already_settled" {
				t.Fatalf("replay status = %d body = %v, want 200 already_settled", rs, rb)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("PayLink never reached VERIFIED within the deadline")
}

func TestE2E_TamperedProofRejected(t *testing.T) {
	waitReady(t)
	plHex, _ := createPayLink(t, 1500)
	p := signedProof(t, plHex, 1500)

	// Send a different amount than what was signed → signature no longer matches.
	status, _ := postProof(t, p, 9999, "e2e-tampered-"+p.TxID)
	if status != http.StatusUnauthorized {
		t.Fatalf("tampered proof status = %d, want 401", status)
	}

	// Nothing should have been broadcast: the PayLink stays CREATED.
	var pl struct {
		Status string `json:"status"`
	}
	_ = rpc(t, "paylink_getPayLink", map[string]string{"id": plHex}, &pl)
	if pl.Status != "CREATED" {
		t.Fatalf("tampered proof must not settle; PayLink status = %s, want CREATED", pl.Status)
	}
}
