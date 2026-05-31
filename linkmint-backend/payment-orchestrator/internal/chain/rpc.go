package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/paylink/payment-orchestrator/internal/httpx"
)

// Client is a minimal JSON-RPC 2.0 client for the lVM (paylink-chain/internal/rpc). It satisfies
// domain.ChainReader.
type Client struct {
	base string
	http *http.Client
}

// NewClient builds a Client. hc must be non-nil (use one with a timeout).
func NewClient(baseURL string, hc *http.Client) *Client {
	return &Client{base: baseURL, http: hc}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	ID      int    `json:"id"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message) }

func (c *Client) call(ctx context.Context, method string, params, out any) error {
	buf, err := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params, ID: 1})
	if err != nil {
		return httpx.NewError(httpx.CodeChainUnavailable, "marshal rpc request: "+err.Error(), nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base, bytes.NewReader(buf))
	if err != nil {
		return httpx.NewError(httpx.CodeChainUnavailable, "build rpc request: "+err.Error(), nil)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return httpx.NewError(httpx.CodeChainUnavailable, "chain rpc unreachable: "+err.Error(), nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httpx.NewError(httpx.CodeChainUnavailable, fmt.Sprintf("chain rpc returned http %d", resp.StatusCode), nil)
	}
	var rr rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return httpx.NewError(httpx.CodeChainUnavailable, "decode rpc response: "+err.Error(), nil)
	}
	if rr.Error != nil {
		return rr.Error
	}
	if out != nil && len(rr.Result) > 0 {
		if err := json.Unmarshal(rr.Result, out); err != nil {
			return httpx.NewError(httpx.CodeChainUnavailable, "decode rpc result: "+err.Error(), nil)
		}
	}
	return nil
}

// PayLinkStatus returns the authoritative on-chain status for plID. found is false when the
// PayLink is unknown on-chain. Transport/HTTP errors surface as CHAIN_UNAVAILABLE.
func (c *Client) PayLinkStatus(ctx context.Context, plID string) (string, bool, error) {
	var pl struct {
		Status string `json:"status"`
	}
	err := c.call(ctx, "paylink_getPayLink", map[string]string{"id": plID}, &pl)
	if err != nil {
		var re *rpcError
		if errors.As(err, &re) {
			if strings.Contains(strings.ToLower(re.Message), "not found") {
				return "", false, nil
			}
			return "", false, httpx.NewError(httpx.CodeChainUnavailable, re.Message, nil)
		}
		return "", false, err
	}
	return pl.Status, true, nil
}

// Ping checks chain reachability for readiness (cheap chainHeight call).
func (c *Client) Ping(ctx context.Context) error {
	var height uint64
	return c.call(ctx, "paylink_chainHeight", struct{}{}, &height)
}
