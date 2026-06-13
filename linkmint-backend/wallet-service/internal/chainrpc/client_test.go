package chainrpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// rpcServer builds an httptest server that dispatches by method to a canned result, or returns a
// JSON-RPC error when the method is in errs.
func rpcServer(t *testing.T, results map[string]any, errs map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			ID     int    `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if msg, ok := errs[req.Method]; ok {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID,
				"error": map[string]any{"code": -32602, "message": msg},
			})
			return
		}
		res, ok := results[req.Method]
		if !ok {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID,
				"error": map[string]any{"code": -32601, "message": "method not found"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": res})
	}))
}

func TestClientHappyPath(t *testing.T) {
	srv := rpcServer(t, map[string]any{
		"paylink_getAccount":   map[string]any{"address": "0xabc", "balance": 1000, "nonce": 7},
		"paylink_getNonce":     7,
		"paylink_getValidator": map[string]any{"address": "0xabc", "stakedAmount": 50, "isActive": true, "totalRewards": 3},
		"paylink_stakingStats": map[string]any{"totalStaked": 100, "minimumStake": 10, "withdrawalCooldown": 600},
		"paylink_tokenStats":   map[string]any{"totalSupply": 999, "maxSupply": 2000},
		"paylink_chainInfo":    map[string]any{"chainId": "paylink-devnet", "height": 42, "tipHash": "0xtip"},
		"paylink_chainHeight":  42,
	}, nil)
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())
	ctx := context.Background()

	acc, err := c.GetAccount(ctx, "0xabc")
	if err != nil || acc.Balance != 1000 || acc.Nonce != 7 {
		t.Fatalf("GetAccount = %+v, err %v", acc, err)
	}
	nonce, err := c.GetNonce(ctx, "0xabc")
	if err != nil || nonce != 7 {
		t.Fatalf("GetNonce = %d, err %v", nonce, err)
	}
	v, found, err := c.GetValidator(ctx, "0xabc")
	if err != nil || !found || v.StakedAmount != 50 || !v.IsActive {
		t.Fatalf("GetValidator = %+v found=%v err=%v", v, found, err)
	}
	ss, err := c.StakingStats(ctx)
	if err != nil || ss.TotalStaked != 100 || ss.MinimumStake != 10 {
		t.Fatalf("StakingStats = %+v err %v", ss, err)
	}
	ts, err := c.TokenStats(ctx)
	if err != nil || ts.TotalSupply != 999 || ts.MaxSupply != 2000 {
		t.Fatalf("TokenStats = %+v err %v", ts, err)
	}
	ci, err := c.ChainInfo(ctx)
	if err != nil || ci.ChainID != "paylink-devnet" || ci.Height != 42 {
		t.Fatalf("ChainInfo = %+v err %v", ci, err)
	}
	h, err := c.ChainHeight(ctx)
	if err != nil || h != 42 {
		t.Fatalf("ChainHeight = %d err %v", h, err)
	}
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestClientValidatorNotFound(t *testing.T) {
	srv := rpcServer(t, nil, map[string]string{"paylink_getValidator": "validator not found"})
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	v, found, err := c.GetValidator(context.Background(), "0xabc")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if found {
		t.Fatalf("expected not-found, got %+v", v)
	}
}

func TestClientRPCErrorMapsUnavailable(t *testing.T) {
	srv := rpcServer(t, nil, map[string]string{"paylink_getAccount": "boom internal"})
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.GetAccount(context.Background(), "0xabc")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestClientTransportErrorUnavailable(t *testing.T) {
	// Point at a closed server to force a transport error.
	srv := rpcServer(t, map[string]any{}, nil)
	url := srv.URL
	srv.Close()
	c := NewClient(url, &http.Client{})

	_, err := c.GetNonce(context.Background(), "0xabc")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestClientNon200Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())
	if err := c.Ping(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}
