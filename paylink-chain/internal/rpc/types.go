package rpc

import "encoding/json"

// JSON-RPC 2.0 request/response types.

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC error codes
const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

func NewResponse(id interface{}, result interface{}) *Response {
	return &Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
}

func NewErrorResponse(id interface{}, code int, msg string) *Response {
	return &Response{
		JSONRPC: "2.0",
		Error:   &RPCError{Code: code, Message: msg},
		ID:      id,
	}
}

// ── RPC-specific response types ──

type BlockResponse struct {
	Height       uint64 `json:"height"`
	Timestamp    int64  `json:"timestamp"`
	PreviousHash string `json:"previousHash"`
	StateRoot    string `json:"stateRoot"`
	TxRoot       string `json:"txRoot"`
	Proposer     string `json:"proposer"`
	Hash         string `json:"hash"`
	TxCount      int    `json:"txCount"`
}

type AccountResponse struct {
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
	Nonce   uint64 `json:"nonce"`
}

type PayLinkResponse struct {
	ID            string          `json:"id"`
	Creator       string          `json:"creator"`
	Receiver      string          `json:"receiver"`
	Owner         string          `json:"owner"`
	Approved      string          `json:"approved"`
	Amount        uint64          `json:"amount"`
	Expiry        int64           `json:"expiry"`
	Status        string          `json:"status"`
	MetadataHash  string          `json:"metadataHash"`
	CreatedAt     int64           `json:"createdAt"`
	VoteCount     uint64          `json:"voteCount"`
	TransferCount uint64          `json:"transferCount"`
	Rules         json.RawMessage `json:"rules,omitempty"`
}

type ValidatorResponse struct {
	Address           string `json:"address"`
	StakedAmount      uint64 `json:"stakedAmount"`
	PendingWithdrawal uint64 `json:"pendingWithdrawal"`
	WithdrawableAt    int64  `json:"withdrawableAt"`
	TotalSlashed      uint64 `json:"totalSlashed"`
	TotalRewards      uint64 `json:"totalRewards"`
	IsActive          bool   `json:"isActive"`
	JoinedAt          int64  `json:"joinedAt"`
}

type ChainInfoResponse struct {
	ChainID string `json:"chainId"`
	Height  uint64 `json:"height"`
	TipHash string `json:"tipHash"`
}

type SendTxResponse struct {
	TxHash string `json:"txHash"`
}

type TxReceiptResponse struct {
	TxHash      string `json:"txHash"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
	BlockHeight uint64 `json:"blockHeight"`
	TxIndex     int    `json:"txIndex"`
}

type StakingStatsResponse struct {
	TotalStaked          uint64 `json:"totalStaked"`
	ActiveValidatorCount int    `json:"activeValidatorCount"`
	TotalValidatorCount  int    `json:"totalValidatorCount"`
	MinimumStake         uint64 `json:"minimumStake"`
	WithdrawalCooldown   int64  `json:"withdrawalCooldown"`
}

type TokenStatsResponse struct {
	TotalSupply uint64 `json:"totalSupply"`
	MaxSupply   uint64 `json:"maxSupply"`
}

type ChainStatsResponse struct {
	ChainID             string `json:"chainId"`
	Height              uint64 `json:"height"`
	TotalAccounts       int    `json:"totalAccounts"`
	TotalPayLinks       int    `json:"totalPayLinks"`
	TotalValidators     int    `json:"totalValidators"`
	TotalProofsUsed     int    `json:"totalProofsUsed"`
	RequiredValidations uint64 `json:"requiredValidations"`
}

type NodeInfoResponse struct {
	ChainID     string `json:"chainId"`
	NodeVersion string `json:"nodeVersion"`
	Height      uint64 `json:"height"`
	AdminAddr   string `json:"adminAddress"`
	RPCAddr     string `json:"rpcAddr"`
}
