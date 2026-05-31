// Package paylinks is an HTTP client for paylink-service (the PayLink record owner, work01).
// It implements domain.PayLinkLookup. The orchestrator validates a PayLink against this service
// on initiate; settlement truth still comes from the chain (invariant A.3), not from here.
package paylinks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/httpx"
)

// Client talks to paylink-service's /v1/paylinks API.
type Client struct {
	base string
	http *http.Client
}

// NewClient builds a Client. hc must be non-nil (use one with a timeout).
func NewClient(baseURL string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(baseURL, "/"), http: hc}
}

// paylinkResponse mirrors paylink-service's PayLinkResponse (only the fields we need).
type paylinkResponse struct {
	PLID   string `json:"pl_id"`
	Status string `json:"status"`
	Expiry string `json:"expiry"`
}

// GetPayLink fetches a PayLink record, returning (nil, nil) when it does not exist (404).
func (c *Client) GetPayLink(ctx context.Context, plID string) (*domain.PayLinkRecord, error) {
	url := c.base + "/v1/paylinks/" + plID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, httpx.NewError(httpx.CodePayLinkSvcUnavail, "build paylink-service request: "+err.Error(), nil)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, httpx.NewError(httpx.CodePayLinkSvcUnavail, "paylink-service unreachable: "+err.Error(), nil)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, nil
	case resp.StatusCode != http.StatusOK:
		return nil, httpx.NewError(httpx.CodePayLinkSvcUnavail,
			fmt.Sprintf("paylink-service returned http %d", resp.StatusCode), nil)
	}

	var body paylinkResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, httpx.NewError(httpx.CodePayLinkSvcUnavail, "decode paylink-service response: "+err.Error(), nil)
	}

	rec := &domain.PayLinkRecord{ID: body.PLID, Status: body.Status}
	if body.Expiry != "" {
		if t, perr := time.Parse(time.RFC3339, body.Expiry); perr == nil {
			rec.Expiry = t
		} else {
			// A malformed expiry leaves rec.Expiry zero (treated as "no expiry"); surface it so
			// the contract mismatch is observable rather than silently swallowed.
			slog.Warn("paylink_expiry_unparseable", "paylink_id", body.PLID, "expiry", body.Expiry)
		}
	}
	return rec, nil
}
