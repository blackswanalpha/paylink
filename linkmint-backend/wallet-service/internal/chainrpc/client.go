// Package chainrpc is a minimal JSON-RPC 2.0 client for the lVM (paylink-chain/internal/rpc). It is
// the wallet-service read-through to on-chain truth: live balance/nonce for GET /v1/wallets/{addr}
// and the nonce/chain-id for the unsigned-tx intent. It is transport-decoupled — it returns its own
// ErrUnavailable / ErrNotFound sentinels, which the domain maps to the HTTP envelope, so this
// package never imports the service's httpx layer.
//
// Copied in shape from the work02 payment-orchestrator chain client.
package chainrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrUnavailable wraps any transport/HTTP/decode failure reaching the chain — the chain RPC could
// not be consulted. Callers test it with errors.Is to fall back to the indexed read-side.
var ErrUnavailable = errors.New("chain rpc unavailable")

// ErrNotFound is returned when the chain reports the queried entity does not exist.
var ErrNotFound = errors.New("not found on chain")

// ── Result types (mirror paylink-chain/internal/rpc response structs) ──

// Account is paylink_getAccount → {address, balance, nonce}.
type Account struct {
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
	Nonce   uint64 `json:"nonce"`
}

// Validator is paylink_getValidator → ValidatorResponse.
type Validator struct {
	Address           string `json:"address"`
	StakedAmount      uint64 `json:"stakedAmount"`
	PendingWithdrawal uint64 `json:"pendingWithdrawal"`
	WithdrawableAt    int64  `json:"withdrawableAt"`
	TotalSlashed      uint64 `json:"totalSlashed"`
	TotalRewards      uint64 `json:"totalRewards"`
	IsActive          bool   `json:"isActive"`
	JoinedAt          int64  `json:"joinedAt"`
}

// StakingStats is paylink_stakingStats → StakingStatsResponse.
type StakingStats struct {
	TotalStaked          uint64 `json:"totalStaked"`
	ActiveValidatorCount int    `json:"activeValidatorCount"`
	TotalValidatorCount  int    `json:"totalValidatorCount"`
	MinimumStake         uint64 `json:"minimumStake"`
	WithdrawalCooldown   int64  `json:"withdrawalCooldown"`
}

// TokenStats is paylink_tokenStats → {totalSupply, maxSupply}.
type TokenStats struct {
	TotalSupply uint64 `json:"totalSupply"`
	MaxSupply   uint64 `json:"maxSupply"`
}

// ChainInfo is paylink_chainInfo → {chainId, height, tipHash}.
type ChainInfo struct {
	ChainID string `json:"chainId"`
	Height  uint64 `json:"height"`
	TipHash string `json:"tipHash"`
}

// Client is a JSON-RPC 2.0 client for the lVM RPC.
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

// notFound reports whether an rpcError signals a missing entity (the chain returns invalid-params
// with a "... not found" message rather than a typed code).
func (e *rpcError) notFound() bool { return strings.Contains(strings.ToLower(e.Message), "not found") }

func (c *Client) call(ctx context.Context, method string, params, out any) error {
	buf, err := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params, ID: 1})
	if err != nil {
		return fmt.Errorf("%w: marshal rpc request: %v", ErrUnavailable, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("%w: build rpc request: %v", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: chain rpc returned http %d", ErrUnavailable, resp.StatusCode)
	}
	var rr rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return fmt.Errorf("%w: decode rpc response: %v", ErrUnavailable, err)
	}
	if rr.Error != nil {
		return rr.Error
	}
	if out != nil && len(rr.Result) > 0 {
		if err := json.Unmarshal(rr.Result, out); err != nil {
			return fmt.Errorf("%w: decode rpc result: %v", ErrUnavailable, err)
		}
	}
	return nil
}

// asUnavailable maps a bare rpcError (RPC-level error) to ErrUnavailable; transport errors are
// already wrapped. Used by queries with no "not found" semantics.
func asUnavailable(err error) error {
	var re *rpcError
	if errors.As(err, &re) {
		return fmt.Errorf("%w: %s", ErrUnavailable, re.Message)
	}
	return err
}

// GetAccount returns the on-chain balance and nonce for addr. The chain returns zeros for an unknown
// address (never a not-found), so this only errors on transport failure (ErrUnavailable).
func (c *Client) GetAccount(ctx context.Context, addr string) (Account, error) {
	var acc Account
	if err := c.call(ctx, "paylink_getAccount", map[string]string{"address": addr}, &acc); err != nil {
		return Account{}, asUnavailable(err)
	}
	return acc, nil
}

// GetNonce returns the account's current nonce (paylink_getNonce returns a bare uint64).
func (c *Client) GetNonce(ctx context.Context, addr string) (uint64, error) {
	var nonce uint64
	if err := c.call(ctx, "paylink_getNonce", map[string]string{"address": addr}, &nonce); err != nil {
		return 0, asUnavailable(err)
	}
	return nonce, nil
}

// GetValidator returns the validator/staking record for addr. found is false (err nil) when the
// address is not a validator.
func (c *Client) GetValidator(ctx context.Context, addr string) (Validator, bool, error) {
	var v Validator
	err := c.call(ctx, "paylink_getValidator", map[string]string{"address": addr}, &v)
	if err != nil {
		var re *rpcError
		if errors.As(err, &re) {
			if re.notFound() {
				return Validator{}, false, nil
			}
			return Validator{}, false, fmt.Errorf("%w: %s", ErrUnavailable, re.Message)
		}
		return Validator{}, false, err
	}
	return v, true, nil
}

// StakingStats returns global staking parameters/totals.
func (c *Client) StakingStats(ctx context.Context) (StakingStats, error) {
	var s StakingStats
	if err := c.call(ctx, "paylink_stakingStats", struct{}{}, &s); err != nil {
		return StakingStats{}, asUnavailable(err)
	}
	return s, nil
}

// TokenStats returns the live PLN supply figures.
func (c *Client) TokenStats(ctx context.Context) (TokenStats, error) {
	var s TokenStats
	if err := c.call(ctx, "paylink_tokenStats", struct{}{}, &s); err != nil {
		return TokenStats{}, asUnavailable(err)
	}
	return s, nil
}

// ChainInfo returns the chain id, height, and tip hash.
func (c *Client) ChainInfo(ctx context.Context) (ChainInfo, error) {
	var ci ChainInfo
	if err := c.call(ctx, "paylink_chainInfo", struct{}{}, &ci); err != nil {
		return ChainInfo{}, asUnavailable(err)
	}
	return ci, nil
}

// ChainHeight returns the current block height.
func (c *Client) ChainHeight(ctx context.Context) (uint64, error) {
	var h uint64
	if err := c.call(ctx, "paylink_chainHeight", struct{}{}, &h); err != nil {
		return 0, asUnavailable(err)
	}
	return h, nil
}

// Ping checks chain reachability (cheap chainHeight call) for the readiness probe.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.ChainHeight(ctx)
	return err
}
