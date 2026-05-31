// Package broadcast POSTs a signed proof to the proof-validator (work03) POST /v1/proofs and
// interprets its response. The validator settles asynchronously via on-chain quorum (A.3); a fresh
// broadcast returns 202, an already-settled proof returns 200 (A.7 — not an error).
package broadcast

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/paylink/mpesa-adapter/internal/httpx"
	"github.com/paylink/mpesa-adapter/internal/proof"
)

// Result is the validator's accepted outcome for a submitted proof.
type Result struct {
	ProofHash string
	TxHash    string
	Status    string // "broadcast" | "already_settled" | "settled"
}

// Client talks to the proof-validator's /v1/proofs API.
type Client struct {
	base string
	http *http.Client
}

// NewClient builds a Client. hc must be non-nil (use one with a timeout).
func NewClient(baseURL string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(baseURL, "/"), http: hc}
}

// submitView mirrors the validator's POST /v1/proofs success body.
type submitView struct {
	ProofHash string `json:"proof_hash"`
	TxHash    string `json:"tx_hash"`
	Status    string `json:"status"`
}

// envelope mirrors the validator's error envelope.
type envelope struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

// Broadcast submits p to the validator with the given Idempotency-Key. A 202 (broadcast) or 200
// (already settled / settled) is success; a 4xx proof rejection becomes PROOF_REJECTED; transport
// or 5xx failures become VALIDATOR_UNAVAILABLE (the caller may retry — the proof stays unsettled).
func (c *Client) Broadcast(ctx context.Context, p proof.Proof, idemKey string) (Result, error) {
	body, err := proof.MarshalWire(p)
	if err != nil {
		return Result{}, httpx.NewError(httpx.CodeInternalError, "marshal proof: "+err.Error(), nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/proofs", bytes.NewReader(body))
	if err != nil {
		return Result{}, httpx.NewError(httpx.CodeValidatorUnavailable, "build validator request: "+err.Error(), nil)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idemKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, httpx.NewError(httpx.CodeValidatorUnavailable, "proof-validator unreachable: "+err.Error(), nil)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	switch {
	case resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK:
		var v submitView
		if err := json.Unmarshal(raw, &v); err != nil {
			return Result{}, httpx.NewError(httpx.CodeValidatorUnavailable, "decode validator response: "+err.Error(), nil)
		}
		return Result{ProofHash: v.ProofHash, TxHash: v.TxHash, Status: v.Status}, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// The validator refused the proof (bad signature, amount mismatch, expired, …). Surface its
		// code/message so the operator can see why; this is not retryable without a new proof.
		var env envelope
		_ = json.Unmarshal(raw, &env)
		return Result{}, httpx.NewError(httpx.CodeProofRejected,
			fmt.Sprintf("proof-validator rejected the proof: %s", validatorMsg(env)),
			map[string]any{"validator_code": env.Error.Code, "validator_status": resp.StatusCode})
	default:
		return Result{}, httpx.NewError(httpx.CodeValidatorUnavailable,
			fmt.Sprintf("proof-validator returned http %d", resp.StatusCode),
			map[string]any{"validator_status": resp.StatusCode})
	}
}

func validatorMsg(e envelope) string {
	if e.Error.Code != "" {
		return e.Error.Code + " — " + e.Error.Message
	}
	return "unknown error"
}
