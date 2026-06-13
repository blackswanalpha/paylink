package server_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paylink/wallet-service/internal/chainrpc"
	"github.com/paylink/wallet-service/internal/domain"
	"github.com/paylink/wallet-service/internal/metrics"
	"github.com/paylink/wallet-service/internal/server"
	"github.com/paylink/wallet-service/internal/store/memory"
)

const (
	addr1 = "0x" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	addr2 = "0x" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type harness struct {
	srv   *server.Server
	store *memory.Store
	chain *chainrpc.FakeClient
}

func newHarness(t *testing.T, ready []server.ReadyCheck, chainPing func(context.Context) error) *harness {
	t.Helper()
	st := memory.New()
	fake := chainrpc.NewFake()
	svc := domain.NewService(st, fake, discardLogger(), domain.WithChainID("paylink-test"))
	if chainPing == nil {
		chainPing = fake.Ping
	}
	srv := server.New(svc, metrics.New(), discardLogger(), ready, chainPing, "")
	return &harness{srv: srv, store: st, chain: fake}
}

// do issues a request with the gateway-injected X-Creator-Addr set to `caller` (empty = none).
func (h *harness) do(method, target, caller, body string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rdr)
	if caller != "" {
		req.Header.Set("X-Creator-Addr", caller)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.srv.Handler().ServeHTTP(rr, req)
	return rr
}

func TestHealthz(t *testing.T) {
	h := newHarness(t, nil, nil)
	rr := h.do(http.MethodGet, "/internal/healthz", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rr.Code)
	}
}

func TestReadyzPostgresDown(t *testing.T) {
	ready := []server.ReadyCheck{{Name: "postgres", Check: func(context.Context) error { return errors.New("conn refused") }}}
	h := newHarness(t, ready, nil)
	rr := h.do(http.MethodGet, "/internal/readyz", "", "")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz with pg down = %d, want 503", rr.Code)
	}
}

func TestReadyzChainDegradedStays200(t *testing.T) {
	ready := []server.ReadyCheck{{Name: "postgres", Check: func(context.Context) error { return nil }}}
	h := newHarness(t, ready, func(context.Context) error { return errors.New("chain down") })
	rr := h.do(http.MethodGet, "/internal/readyz", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("readyz with chain down = %d, want 200", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["degraded"] == nil {
		t.Fatalf("expected degraded field, got %v", body)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	h := newHarness(t, nil, nil)
	// One prior request through the middleware so http_requests_total has a sample to scrape.
	h.do(http.MethodGet, "/internal/healthz", "", "")
	rr := h.do(http.MethodGet, "/metrics", "", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "http_requests_total") {
		t.Fatalf("metrics endpoint = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetWalletSelfScope(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.chain.Accounts[addr1] = chainrpc.Account{Address: addr1, Balance: 4242, Nonce: 3}

	// Unauthenticated → 401.
	if rr := h.do(http.MethodGet, "/v1/wallets/"+addr1, "", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no caller = %d, want 401", rr.Code)
	}
	// Caller != target → 403.
	if rr := h.do(http.MethodGet, "/v1/wallets/"+addr1, addr2, ""); rr.Code != http.StatusForbidden {
		t.Fatalf("mismatched caller = %d, want 403", rr.Code)
	}
	// Self → 200 with balance.
	rr := h.do(http.MethodGet, "/v1/wallets/"+addr1, addr1, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("self = %d", rr.Code)
	}
	var v struct {
		Balance string `json:"balance"`
		Nonce   uint64 `json:"nonce"`
		Stale   bool   `json:"stale"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &v)
	if v.Balance != "4242" || v.Nonce != 3 || v.Stale {
		t.Fatalf("wallet view = %+v", v)
	}
}

func TestGetWalletInvalidAddr(t *testing.T) {
	h := newHarness(t, nil, nil)
	rr := h.do(http.MethodGet, "/v1/wallets/0xnothex", "0xnothex", "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid addr = %d, want 400", rr.Code)
	}
}

func TestGetWalletChainUnavailable(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.chain.Err = chainrpc.ErrUnavailable // no cache + chain down
	rr := h.do(http.MethodGet, "/v1/wallets/"+addr1, addr1, "")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("chain unavailable = %d, want 503", rr.Code)
	}
}

func TestPositionsAndRewards(t *testing.T) {
	h := newHarness(t, nil, nil)
	ctx := context.Background()
	at := time.Unix(1000, 0)
	_, _ = h.store.RecordStaked(ctx, domain.StakedEvent{Addr: addr1, Amount: big.NewInt(100), TotalStaked: big.NewInt(100), IsActive: true, TxHash: "0x1", BlockHeight: 1, OccurredAt: at})
	_, _ = h.store.RecordRewarded(ctx, domain.RewardedEvent{Addr: addr1, Amount: big.NewInt(7), TotalRewards: big.NewInt(7), TxHash: "0x2", BlockHeight: 2, OccurredAt: at})

	// Position 200.
	rr := h.do(http.MethodGet, "/v1/staking/positions?addr="+addr1, addr1, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("positions = %d", rr.Code)
	}
	var pos struct {
		StakedAmount string `json:"staked_amount"`
		IsActive     bool   `json:"is_active"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &pos)
	if pos.StakedAmount != "100" || !pos.IsActive {
		t.Fatalf("position view = %+v", pos)
	}

	// Position 404 for a different (self) address with no stake.
	if rr := h.do(http.MethodGet, "/v1/staking/positions?addr="+addr2, addr2, ""); rr.Code != http.StatusNotFound {
		t.Fatalf("no-position = %d, want 404", rr.Code)
	}

	// Rewards list.
	rr = h.do(http.MethodGet, "/v1/staking/rewards?addr="+addr1, addr1, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("rewards = %d", rr.Code)
	}
	var rewards struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &rewards)
	if len(rewards.Items) != 1 {
		t.Fatalf("rewards items = %d", len(rewards.Items))
	}
}

func TestTransactionsList(t *testing.T) {
	h := newHarness(t, nil, nil)
	at := time.Unix(1000, 0)
	_, _ = h.store.RecordTransfer(context.Background(), domain.TransferEvent{From: addr2, To: addr1, Amount: big.NewInt(5), TxHash: "0xt", BlockHeight: 1, OccurredAt: at})
	rr := h.do(http.MethodGet, "/v1/wallets/"+addr1+"/transactions", addr1, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("transactions = %d", rr.Code)
	}
	var out struct {
		Items []struct {
			Direction string `json:"direction"`
			Kind      string `json:"kind"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if len(out.Items) != 1 || out.Items[0].Direction != "in" || out.Items[0].Kind != "transfer" {
		t.Fatalf("tx items = %+v", out.Items)
	}
}

func TestTreasuryPublic(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.chain.Tokens = chainrpc.TokenStats{TotalSupply: 500, MaxSupply: 1000}
	// No X-Creator-Addr — treasury is public.
	rr := h.do(http.MethodGet, "/v1/treasury/stats", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("treasury = %d", rr.Code)
	}
	var v struct {
		TotalSupply string `json:"total_supply"`
		MaxSupply   string `json:"max_supply"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &v)
	if v.TotalSupply != "500" || v.MaxSupply != "1000" {
		t.Fatalf("treasury view = %+v", v)
	}
}

func TestPostIntentUnsigned(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.chain.Nonces[addr1] = 11

	rr := h.do(http.MethodPost, "/v1/staking/intent", addr1, `{"addr":"`+addr1+`","action":"stake","amount":"1000"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("intent = %d body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		UnsignedTx struct {
			Type      int    `json:"type"`
			Signature []byte `json:"signature"`
			Hash      string `json:"hash"`
		} `json:"unsigned_tx"`
		SignableBytes string `json:"signable_bytes"`
		Nonce         uint64 `json:"nonce"`
		ChainID       string `json:"chain_id"`
		FeeEstimate   struct {
			Amount string `json:"amount"`
		} `json:"fee_estimate"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.UnsignedTx.Type != 6 || out.Nonce != 11 || out.ChainID != "paylink-test" {
		t.Fatalf("intent view = %+v", out)
	}
	if len(out.UnsignedTx.Signature) != 0 {
		t.Fatal("A.1: signature must be empty")
	}
	if out.UnsignedTx.Hash != "0x0000000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("A.1: hash must be zero, got %s", out.UnsignedTx.Hash)
	}
	if out.FeeEstimate.Amount != "0" {
		t.Fatalf("fee = %s", out.FeeEstimate.Amount)
	}
	if _, err := base64.StdEncoding.DecodeString(out.SignableBytes); err != nil {
		t.Fatalf("signable_bytes not base64: %v", err)
	}
}

func TestPostIntentValidation(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.chain.Nonces[addr1] = 1

	// Bad amount (non-integer) → 400.
	if rr := h.do(http.MethodPost, "/v1/staking/intent", addr1, `{"addr":"`+addr1+`","action":"stake","amount":"abc"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("bad amount = %d", rr.Code)
	}
	// Bad action → 400.
	if rr := h.do(http.MethodPost, "/v1/staking/intent", addr1, `{"addr":"`+addr1+`","action":"send","amount":"1"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("bad action = %d", rr.Code)
	}
	// Caller mismatch → 403.
	if rr := h.do(http.MethodPost, "/v1/staking/intent", addr2, `{"addr":"`+addr1+`","action":"stake","amount":"1"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("mismatch = %d", rr.Code)
	}
	// Malformed JSON → 400.
	if rr := h.do(http.MethodPost, "/v1/staking/intent", addr1, `{not json`); rr.Code != http.StatusBadRequest {
		t.Fatalf("bad json = %d", rr.Code)
	}
}
