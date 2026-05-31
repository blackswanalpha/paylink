//go:build e2e

// Package e2e drives the real hybrid stack (docker compose --profile e2e: postgres + redis +
// paylink-chain + proof-validator + mpesa-adapter [Go core] + mpesa-daraja [Node rail, DARAJA_STUB])
// and proves the work04 acceptance criteria end-to-end: a charge + a (stubbed) Daraja STK callback
// flow receive→normalize→sign→broadcast and SETTLE the PayLink on-chain.
//
//	Run: docker compose --profile e2e up -d --build && \
//	     cd adapters/mpesa && go test -tags=e2e ./test/... -v
package e2e

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/pkg/lvm"
)

// Devnet values (well-known; see docker-compose.yml / .env.example).
const (
	merchantKey   = "1a2b3c4d5e6f70819293a4b5c6d7e8f9000102030405060708090a0b0c0d0e0f"
	callbackToken = "devnet-callback-token"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func chainURL() string   { return env("CHAIN_RPC_URL", "http://localhost:8545/") }
func adapterURL() string { return env("MPESA_ADAPTER_URL", "http://localhost:8082") }
func nodeURL() string    { return env("MPESA_DARAJA_URL", "http://localhost:8083") }

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

// createPayLink mints a PayLink on-chain from the merchant and returns its id (hex).
func createPayLink(t *testing.T, amount uint64) string {
	t.Helper()
	mk := mustKey(t, merchantKey)
	from := lvm.PrivateKeyToAddress(mk)

	var nonce uint64
	if err := rpc(t, "paylink_getNonce", map[string]string{"address": from.Hex()}, &nonce); err != nil {
		t.Fatalf("getNonce: %v", err)
	}
	plID := lvm.SHA256Hash([]byte(fmt.Sprintf("e2e-mpesa-%d", time.Now().UnixNano())))
	tx, err := lvm.BuildCreatePayLinkTx(from, nonce, lvm.CreatePayLinkPayload{
		PayLinkID: plID,
		Receiver:  from,
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
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var pl struct {
			Status string `json:"status"`
		}
		if err := rpc(t, "paylink_getPayLink", map[string]string{"id": plID.Hex()}, &pl); err == nil && pl.Status == "CREATED" {
			return plID.Hex()
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("paylink %s never reached CREATED", plID.Hex())
	return ""
}

// postCharge starts an STK push via the Go core /v1/charges and returns the CheckoutRequestID.
func postCharge(t *testing.T, plHex string, amount uint64) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"pl_id": plHex, "amount": amount, "payer_phone": "254700000000", "receiver_shortcode": "174379",
	})
	req, _ := http.NewRequest(http.MethodPost, adapterURL()+"/v1/charges", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "e2e-charge-"+plHex)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/charges: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("charge status = %d, want 202; body=%s", resp.StatusCode, raw)
	}
	var out struct {
		CheckoutRequestID string `json:"checkout_request_id"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.CheckoutRequestID == "" {
		t.Fatalf("no checkout_request_id in charge response: %s", raw)
	}
	return out.CheckoutRequestID
}

// postCallback posts a Daraja STK success callback to the Node rail service.
func postCallback(t *testing.T, checkoutID, receipt string, amount uint64) {
	t.Helper()
	cb := map[string]any{
		"Body": map[string]any{
			"stkCallback": map[string]any{
				"MerchantRequestID": "e2e-m",
				"CheckoutRequestID": checkoutID,
				"ResultCode":        0,
				"ResultDesc":        "ok",
				"CallbackMetadata": map[string]any{
					"Item": []map[string]any{
						{"Name": "Amount", "Value": amount},
						{"Name": "MpesaReceiptNumber", "Value": receipt},
						{"Name": "PhoneNumber", "Value": "254700000000"},
					},
				},
			},
		},
	}
	body, _ := json.Marshal(cb)
	resp, err := http.Post(nodeURL()+"/daraja/callback?t="+callbackToken, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /daraja/callback: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("callback status = %d, want 200; body=%s", resp.StatusCode, raw)
	}
}

func waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(adapterURL() + "/internal/readyz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatal("mpesa-adapter never became ready (redis / daraja_service / proof_validator down?)")
}

func TestE2E_MpesaPaymentSettles(t *testing.T) {
	waitReady(t)

	plHex := createPayLink(t, 1500)
	checkoutID := postCharge(t, plHex, 1500)
	receipt := fmt.Sprintf("E2E%d", time.Now().UnixNano())
	postCallback(t, checkoutID, receipt, 1500)

	// The proof flows core→validator→chain; poll for on-chain settlement.
	wantPH := lvm.ProofHash(lvm.HexToHash(plHex), receipt, 1500).Hex()
	deadline := time.Now().Add(20 * time.Second)
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
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("PayLink %s never reached VERIFIED", plHex)
}
