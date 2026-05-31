package chain

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paylink/payment-orchestrator/internal/httpx"
)

func rpcServer(t *testing.T, handler func(method string) (any, *struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}, int)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		result, rpcErr, status := handler(req.Method)
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": 1}
		if rpcErr != nil {
			resp["error"] = rpcErr
		} else {
			resp["result"] = result
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

type rpcErr = struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func TestPayLinkStatusFound(t *testing.T) {
	srv := rpcServer(t, func(string) (any, *rpcErr, int) {
		return map[string]any{"status": "CREATED"}, nil, 0
	})
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	status, found, err := c.PayLinkStatus(t.Context(), "0xabc")
	if err != nil || !found || status != "CREATED" {
		t.Fatalf("got (%q,%v,%v)", status, found, err)
	}
}

func TestPayLinkStatusNotFound(t *testing.T) {
	srv := rpcServer(t, func(string) (any, *rpcErr, int) {
		return nil, &rpcErr{Code: -32602, Message: "paylink not found"}, 0
	})
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	status, found, err := c.PayLinkStatus(t.Context(), "0xabc")
	if err != nil || found || status != "" {
		t.Fatalf("not-found should be (\"\",false,nil); got (%q,%v,%v)", status, found, err)
	}
}

func TestPayLinkStatusRPCError(t *testing.T) {
	srv := rpcServer(t, func(string) (any, *rpcErr, int) {
		return nil, &rpcErr{Code: -32603, Message: "internal boom"}, 0
	})
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, _, err := c.PayLinkStatus(t.Context(), "0xabc")
	var ae *httpx.AppError
	if !errors.As(err, &ae) || ae.Code != httpx.CodeChainUnavailable {
		t.Fatalf("want CHAIN_UNAVAILABLE, got %v", err)
	}
}

func TestPayLinkStatusHTTPError(t *testing.T) {
	srv := rpcServer(t, func(string) (any, *rpcErr, int) {
		return nil, nil, http.StatusInternalServerError
	})
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, _, err := c.PayLinkStatus(t.Context(), "0xabc")
	if httpx.AsAppError(err).Code != httpx.CodeChainUnavailable {
		t.Fatalf("want CHAIN_UNAVAILABLE, got %v", err)
	}
}

func TestPayLinkStatusTransportError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1/", http.DefaultClient) // unroutable port
	_, _, err := c.PayLinkStatus(t.Context(), "0xabc")
	if httpx.AsAppError(err).Code != httpx.CodeChainUnavailable {
		t.Fatalf("want CHAIN_UNAVAILABLE, got %v", err)
	}
}

func TestPing(t *testing.T) {
	srv := rpcServer(t, func(method string) (any, *rpcErr, int) {
		if method != "paylink_chainHeight" {
			t.Errorf("unexpected method %q", method)
		}
		return 42, nil, 0
	})
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())
	if err := c.Ping(t.Context()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
