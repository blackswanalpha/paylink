// Package chain is the proof-validator's boundary to the lVM: a JSON-RPC 2.0 client for reading
// state (nonce, proof-usage, PayLink, validator, staking) and broadcasting signed transactions.
// It speaks the chain's JSON wire format and reuses paylink-chain/pkg/lvm for the tx encoding.
package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/paylink/paylink-chain/pkg/lvm"
	"github.com/paylink/proof-validator/internal/httpx"
)

// Client is a minimal JSON-RPC 2.0 client for the lVM (paylink-chain/internal/rpc).
type Client struct {
	base string
	http *http.Client
}

// NewClient builds a Client. hc must be non-nil (use one with a timeout).
func NewClient(baseURL string, hc *http.Client) *Client {
	return &Client{base: baseURL, http: hc}
}

// ── On-chain read DTOs (only the fields the validator needs) ──

// PayLinkState is the subset of paylink_getPayLink the validator cross-checks against. (Receiver
// is intentionally omitted: the proof's receiver is a rail-level identifier, not the on-chain
// address, so it is not cross-checked here — see domain.crossCheckPayLink.)
type PayLinkState struct {
	Status string
	Amount uint64
	Expiry int64
}

// ValidatorState is the subset of paylink_getValidator used by the auto-stake bootstrap.
type ValidatorState struct {
	IsActive     bool
	StakedAmount uint64
}

// AccountState is the subset of paylink_getAccount used by the auto-stake bootstrap.
type AccountState struct {
	Balance uint64
	Nonce   uint64
}

// StakingStats is the subset of paylink_stakingStats used by the auto-stake bootstrap.
type StakingStats struct {
	MinimumStake uint64
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

// chainErr maps an rpc error to CHAIN_UNAVAILABLE; non-rpc errors (already AppErrors) pass through.
func chainErr(err error) error {
	var re *rpcError
	if errors.As(err, &re) {
		return httpx.NewError(httpx.CodeChainUnavailable, re.Message, nil)
	}
	return err
}

func isNotFound(err error) bool {
	var re *rpcError
	return errors.As(err, &re) && strings.Contains(strings.ToLower(re.Message), "not found")
}

// GetNonce returns the account's current nonce.
func (c *Client) GetNonce(ctx context.Context, address string) (uint64, error) {
	var n uint64
	if err := c.call(ctx, "paylink_getNonce", map[string]string{"address": address}, &n); err != nil {
		return 0, chainErr(err)
	}
	return n, nil
}

// IsProofUsed reports whether a proof hash has already settled a PayLink on-chain (A.7 truth).
func (c *Client) IsProofUsed(ctx context.Context, proofHash string) (bool, error) {
	var used bool
	if err := c.call(ctx, "paylink_isProofUsed", map[string]string{"proofHash": proofHash}, &used); err != nil {
		return false, chainErr(err)
	}
	return used, nil
}

// GetPayLink returns the on-chain PayLink state; found=false when it is unknown on-chain.
func (c *Client) GetPayLink(ctx context.Context, id string) (*PayLinkState, bool, error) {
	var resp struct {
		Status string `json:"status"`
		Amount uint64 `json:"amount"`
		Expiry int64  `json:"expiry"`
	}
	if err := c.call(ctx, "paylink_getPayLink", map[string]string{"id": id}, &resp); err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, chainErr(err)
	}
	return &PayLinkState{Status: resp.Status, Amount: resp.Amount, Expiry: resp.Expiry}, true, nil
}

// GetValidator returns the validator's state; found=false when the address is not a validator.
func (c *Client) GetValidator(ctx context.Context, address string) (*ValidatorState, bool, error) {
	var resp struct {
		IsActive     bool   `json:"isActive"`
		StakedAmount uint64 `json:"stakedAmount"`
	}
	if err := c.call(ctx, "paylink_getValidator", map[string]string{"address": address}, &resp); err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, chainErr(err)
	}
	return &ValidatorState{IsActive: resp.IsActive, StakedAmount: resp.StakedAmount}, true, nil
}

// GetAccount returns the account balance/nonce (zero values for an unknown account).
func (c *Client) GetAccount(ctx context.Context, address string) (*AccountState, error) {
	var resp struct {
		Balance uint64 `json:"balance"`
		Nonce   uint64 `json:"nonce"`
	}
	if err := c.call(ctx, "paylink_getAccount", map[string]string{"address": address}, &resp); err != nil {
		return nil, chainErr(err)
	}
	return &AccountState{Balance: resp.Balance, Nonce: resp.Nonce}, nil
}

// StakingStats returns staking parameters (the minimum stake to become active).
func (c *Client) StakingStats(ctx context.Context) (*StakingStats, error) {
	var resp struct {
		MinimumStake uint64 `json:"minimumStake"`
	}
	if err := c.call(ctx, "paylink_stakingStats", struct{}{}, &resp); err != nil {
		return nil, chainErr(err)
	}
	return &StakingStats{MinimumStake: resp.MinimumStake}, nil
}

// SendTransaction broadcasts a signed transaction and returns its hash.
func (c *Client) SendTransaction(ctx context.Context, tx *lvm.Transaction) (string, error) {
	var resp struct {
		TxHash string `json:"txHash"`
	}
	if err := c.call(ctx, "paylink_sendTransaction", tx, &resp); err != nil {
		return "", chainErr(err)
	}
	return resp.TxHash, nil
}

// Ping checks chain reachability for readiness (cheap chainHeight call).
func (c *Client) Ping(ctx context.Context) error {
	var height uint64
	if err := c.call(ctx, "paylink_chainHeight", struct{}{}, &height); err != nil {
		return chainErr(err)
	}
	return nil
}
