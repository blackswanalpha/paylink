package daraja

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/paylink/mpesa-adapter/internal/httpx"
)

// Client initiates STK pushes via the Node Daraja rail service.
type Client interface {
	STKPush(ctx context.Context, p STKPushParams) (STKPushResult, error)
}

// HTTPClient is the production Client: it POSTs to the Node rail service's /stk endpoint,
// authenticating with the shared internal token.
type HTTPClient struct {
	base  string
	token string
	http  *http.Client
}

// NewHTTPClient builds an HTTPClient. hc must be non-nil (use one with a timeout).
func NewHTTPClient(baseURL, internalToken string, hc *http.Client) *HTTPClient {
	return &HTTPClient{base: strings.TrimRight(baseURL, "/"), token: internalToken, http: hc}
}

// InternalTokenHeader carries the shared secret on the internal core↔rail hops.
const InternalTokenHeader = "X-Internal-Token"

// STKPush asks the rail service to start a charge and returns the CheckoutRequestID for correlation.
func (c *HTTPClient) STKPush(ctx context.Context, p STKPushParams) (STKPushResult, error) {
	body, _ := json.Marshal(p)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/stk", bytes.NewReader(body))
	if err != nil {
		return STKPushResult{}, httpx.NewError(httpx.CodeDarajaUnavailable, "build rail request: "+err.Error(), nil)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set(InternalTokenHeader, c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return STKPushResult{}, httpx.NewError(httpx.CodeDarajaUnavailable, "daraja rail service unreachable: "+err.Error(), nil)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return STKPushResult{}, httpx.NewError(httpx.CodeDarajaUnavailable,
			fmt.Sprintf("daraja rail service returned http %d", resp.StatusCode),
			map[string]any{"rail_status": resp.StatusCode})
	}
	var out STKPushResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return STKPushResult{}, httpx.NewError(httpx.CodeDarajaUnavailable, "decode rail response: "+err.Error(), nil)
	}
	return out, nil
}
