package chain_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paylink/paylink-chain/pkg/lvm"
	"github.com/paylink/proof-validator/internal/chain"
	"github.com/paylink/proof-validator/internal/httpx"
)

type rpcReq struct {
	Method string          `json:"method"`
	ID     any             `json:"id"`
	Params json.RawMessage `json:"params"`
}

// rpcServer returns canned results (by method) or rpc errors (by method). lastParams captures the
// most recent request params for assertion.
func rpcServer(t *testing.T, results map[string]any, errs map[string]string, lastParams *json.RawMessage) *chain.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		if lastParams != nil {
			*lastParams = req.Params
		}
		w.Header().Set("Content-Type", "application/json")
		if msg, ok := errs[req.Method]; ok {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32602, "message": msg}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": results[req.Method]})
	}))
	t.Cleanup(srv.Close)
	return chain.NewClient(srv.URL, srv.Client())
}

func TestGetNonce(t *testing.T) {
	c := rpcServer(t, map[string]any{"paylink_getNonce": 5}, nil, nil)
	n, err := c.GetNonce(context.Background(), "0xabc")
	if err != nil || n != 5 {
		t.Fatalf("GetNonce = %d, %v; want 5, nil", n, err)
	}
}

func TestIsProofUsed(t *testing.T) {
	c := rpcServer(t, map[string]any{"paylink_isProofUsed": true}, nil, nil)
	used, err := c.IsProofUsed(context.Background(), "0xph")
	if err != nil || !used {
		t.Fatalf("IsProofUsed = %v, %v; want true, nil", used, err)
	}
}

func TestGetPayLink(t *testing.T) {
	c := rpcServer(t, map[string]any{"paylink_getPayLink": map[string]any{
		"status": "CREATED", "amount": 1500, "expiry": 1730000000, "receiver": "0xr",
	}}, nil, nil)
	pl, found, err := c.GetPayLink(context.Background(), "0xpl")
	if err != nil || !found {
		t.Fatalf("GetPayLink found=%v err=%v", found, err)
	}
	if pl.Status != "CREATED" || pl.Amount != 1500 || pl.Expiry != 1730000000 {
		t.Fatalf("unexpected PayLink state: %+v", pl)
	}
}

func TestGetPayLink_NotFound(t *testing.T) {
	c := rpcServer(t, nil, map[string]string{"paylink_getPayLink": "paylink not found"}, nil)
	pl, found, err := c.GetPayLink(context.Background(), "0xpl")
	if err != nil || found || pl != nil {
		t.Fatalf("not-found should be (nil,false,nil); got %+v,%v,%v", pl, found, err)
	}
}

func TestGetValidator(t *testing.T) {
	c := rpcServer(t, map[string]any{"paylink_getValidator": map[string]any{"isActive": true, "stakedAmount": 1000}}, nil, nil)
	v, found, err := c.GetValidator(context.Background(), "0xv")
	if err != nil || !found || !v.IsActive || v.StakedAmount != 1000 {
		t.Fatalf("GetValidator = %+v,%v,%v", v, found, err)
	}
}

func TestGetValidator_NotFound(t *testing.T) {
	c := rpcServer(t, nil, map[string]string{"paylink_getValidator": "validator not found"}, nil)
	_, found, err := c.GetValidator(context.Background(), "0xv")
	if err != nil || found {
		t.Fatalf("not-found should be (nil,false,nil); got found=%v err=%v", found, err)
	}
}

func TestGetAccountAndStakingStats(t *testing.T) {
	c := rpcServer(t, map[string]any{
		"paylink_getAccount":   map[string]any{"balance": 42, "nonce": 3},
		"paylink_stakingStats": map[string]any{"minimumStake": 1000},
	}, nil, nil)
	acc, err := c.GetAccount(context.Background(), "0xa")
	if err != nil || acc.Balance != 42 || acc.Nonce != 3 {
		t.Fatalf("GetAccount = %+v, %v", acc, err)
	}
	ss, err := c.StakingStats(context.Background())
	if err != nil || ss.MinimumStake != 1000 {
		t.Fatalf("StakingStats = %+v, %v", ss, err)
	}
}

func TestSendTransaction(t *testing.T) {
	var params json.RawMessage
	c := rpcServer(t, map[string]any{"paylink_sendTransaction": map[string]any{"txHash": "0xdead"}}, nil, &params)
	tx, _ := lvm.BuildSubmitValidationTx(lvm.HexToAddress("0xabc"), 0, lvm.SHA256Hash([]byte("pl")), lvm.SHA256Hash([]byte("ph")))
	h, err := c.SendTransaction(context.Background(), tx)
	if err != nil || h != "0xdead" {
		t.Fatalf("SendTransaction = %q, %v; want 0xdead", h, err)
	}
	// The full tx object must have been sent as params.
	var sent map[string]any
	if err := json.Unmarshal(params, &sent); err != nil {
		t.Fatalf("params not a tx object: %v", err)
	}
	if _, ok := sent["payload"]; !ok {
		t.Fatalf("sent tx missing payload: %s", params)
	}
}

func TestPing(t *testing.T) {
	c := rpcServer(t, map[string]any{"paylink_chainHeight": 7}, nil, nil)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestRPCError_MapsToChainUnavailable(t *testing.T) {
	c := rpcServer(t, nil, map[string]string{"paylink_chainHeight": "boom"}, nil)
	err := c.Ping(context.Background())
	if c := codeOf(t, err); c != httpx.CodeChainUnavailable {
		t.Fatalf("code = %s, want CHAIN_UNAVAILABLE", c)
	}
}

func TestHTTPError_MapsToChainUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	c := chain.NewClient(srv.URL, srv.Client())
	if code := codeOf(t, c.Ping(context.Background())); code != httpx.CodeChainUnavailable {
		t.Fatalf("code = %s, want CHAIN_UNAVAILABLE", code)
	}
}

func codeOf(t *testing.T, err error) httpx.ErrorCode {
	t.Helper()
	ae := httpx.AsAppError(err)
	return ae.Code
}
