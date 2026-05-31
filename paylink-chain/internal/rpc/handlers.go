package rpc

import (
	"encoding/json"
	"fmt"

	"github.com/paylink/paylink-chain/internal/chain"
	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/txpool"
	"github.com/paylink/paylink-chain/internal/types"
)

// Handlers implements all JSON-RPC method handlers.
type Handlers struct {
	blockchain  *chain.Blockchain
	state       *state.StateDB
	mempool     *txpool.Mempool
	rpcAddr     string
	nodeVersion string
}

// NewHandlers creates a new RPC handler set.
func NewHandlers(bc *chain.Blockchain, s *state.StateDB, mp *txpool.Mempool) *Handlers {
	return &Handlers{
		blockchain:  bc,
		state:       s,
		mempool:     mp,
		nodeVersion: "0.1.0",
	}
}

// SetRPCAddr sets the RPC address for node info.
func (h *Handlers) SetRPCAddr(addr string) {
	h.rpcAddr = addr
}

// Dispatch routes a JSON-RPC request to the appropriate handler.
func (h *Handlers) Dispatch(req *Request) *Response {
	switch req.Method {
	case "paylink_sendTransaction":
		return h.sendTransaction(req)
	case "paylink_getTransaction":
		return h.getTransaction(req)
	case "paylink_getBlock":
		return h.getBlock(req)
	case "paylink_getLatestBlock":
		return h.getLatestBlock(req)
	case "paylink_getPayLink":
		return h.getPayLink(req)
	case "paylink_getAccount":
		return h.getAccount(req)
	case "paylink_getValidator":
		return h.getValidator(req)
	case "paylink_getValidators":
		return h.getValidators(req)
	case "paylink_chainHeight":
		return h.chainHeight(req)
	case "paylink_chainInfo":
		return h.chainInfo(req)
	case "paylink_isProofUsed":
		return h.isProofUsed(req)
	case "paylink_getVoteCount":
		return h.getVoteCount(req)
	case "paylink_pendingTransactions":
		return h.pendingTransactions(req)
	case "paylink_getTransactionReceipt":
		return h.getTransactionReceipt(req)
	case "paylink_getNonce":
		return h.getNonce(req)
	case "paylink_getPayLinksByCreator":
		return h.getPayLinksByCreator(req)
	case "paylink_getPayLinksByReceiver":
		return h.getPayLinksByReceiver(req)
	case "paylink_getPayLinksByStatus":
		return h.getPayLinksByStatus(req)
	case "paylink_getBlockRange":
		return h.getBlockRange(req)
	case "paylink_getBlockTransactions":
		return h.getBlockTransactions(req)
	case "paylink_hasVoted":
		return h.hasVoted(req)
	case "paylink_getVoters":
		return h.getVoters(req)
	case "paylink_stakingStats":
		return h.stakingStats(req)
	case "paylink_tokenStats":
		return h.tokenStats(req)
	case "paylink_chainStats":
		return h.chainStats(req)
	case "paylink_nodeInfo":
		return h.nodeInfo(req)
	case "paylink_getPayLinksByOwner":
		return h.getPayLinksByOwner(req)
	case "paylink_ownerOf":
		return h.ownerOf(req)
	case "paylink_balanceOf":
		return h.balanceOf(req)
	case "paylink_getApproved":
		return h.getApproved(req)
	case "paylink_isApprovedForAll":
		return h.isApprovedForAll(req)
	case "paylink_getPayLinkRules":
		return h.getPayLinkRules(req)
	default:
		return NewErrorResponse(req.ID, ErrCodeMethodNotFound, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (h *Handlers) sendTransaction(req *Request) *Response {
	var tx types.Transaction
	if err := json.Unmarshal(req.Params, &tx); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid transaction: "+err.Error())
	}

	// Compute tx hash
	tx.Hash = pcrypto.SHA256Hash(tx.SignableBytes())

	if err := h.mempool.Add(&tx); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	return NewResponse(req.ID, SendTxResponse{TxHash: tx.Hash.Hex()})
}

func (h *Handlers) getTransaction(req *Request) *Response {
	var params struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	hash := types.HexToHash(params.Hash)
	tx, err := h.blockchain.GetTx(hash)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	if tx == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "transaction not found")
	}

	return NewResponse(req.ID, tx)
}

func (h *Handlers) getBlock(req *Request) *Response {
	var params struct {
		Height *uint64 `json:"height,omitempty"`
		Hash   *string `json:"hash,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	var block *types.Block
	var err error

	if params.Height != nil {
		block, err = h.blockchain.GetBlockByHeight(*params.Height)
	} else if params.Hash != nil {
		hash := types.HexToHash(*params.Hash)
		block, err = h.blockchain.GetBlock(hash)
	} else {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "specify height or hash")
	}

	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	if block == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "block not found")
	}

	return NewResponse(req.ID, toBlockResponse(block))
}

func (h *Handlers) getLatestBlock(req *Request) *Response {
	tip := h.blockchain.Tip()
	if tip == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no blocks yet")
	}
	return NewResponse(req.ID, toBlockResponse(tip))
}

func (h *Handlers) getPayLink(req *Request) *Response {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	id := types.HexToHash(params.ID)
	pl := h.state.GetPayLink(id)
	if pl == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "paylink not found")
	}

	return NewResponse(req.ID, PayLinkResponse{
		ID:            pl.ID.Hex(),
		Creator:       pl.Creator.Hex(),
		Receiver:      pl.Receiver.Hex(),
		Owner:         pl.Owner.Hex(),
		Approved:      pl.Approved.Hex(),
		Amount:        pl.Amount,
		Expiry:        pl.Expiry,
		Status:        pl.Status.String(),
		MetadataHash:  pl.MetadataHash.Hex(),
		CreatedAt:     pl.CreatedAt,
		VoteCount:     pl.VoteCount,
		TransferCount: pl.TransferCount,
		Rules:         pl.Rules,
	})
}

func (h *Handlers) getAccount(req *Request) *Response {
	var params struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	addr := types.HexToAddress(params.Address)
	acc := h.state.GetAccount(addr)
	balance := uint64(0)
	nonce := uint64(0)
	if acc != nil {
		balance = acc.Balance
		nonce = acc.Nonce
	}

	return NewResponse(req.ID, AccountResponse{
		Address: addr.Hex(),
		Balance: balance,
		Nonce:   nonce,
	})
}

func (h *Handlers) getValidator(req *Request) *Response {
	var params struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	addr := types.HexToAddress(params.Address)
	v := h.state.GetValidator(addr)
	if v == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "validator not found")
	}

	return NewResponse(req.ID, toValidatorResponse(v))
}

func (h *Handlers) getValidators(req *Request) *Response {
	addrs := h.state.GetAllValidators()
	var validators []ValidatorResponse
	for _, addr := range addrs {
		v := h.state.GetValidator(addr)
		if v != nil {
			validators = append(validators, toValidatorResponse(v))
		}
	}
	if validators == nil {
		validators = []ValidatorResponse{}
	}
	return NewResponse(req.ID, validators)
}

func (h *Handlers) chainHeight(req *Request) *Response {
	return NewResponse(req.ID, h.blockchain.Height())
}

func (h *Handlers) chainInfo(req *Request) *Response {
	tip := h.blockchain.Tip()
	tipHash := ""
	if tip != nil {
		tipHash = tip.Hash.Hex()
	}
	return NewResponse(req.ID, ChainInfoResponse{
		ChainID: h.blockchain.Genesis().ChainID,
		Height:  h.blockchain.Height(),
		TipHash: tipHash,
	})
}

func (h *Handlers) isProofUsed(req *Request) *Response {
	var params struct {
		ProofHash string `json:"proofHash"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	hash := types.HexToHash(params.ProofHash)
	return NewResponse(req.ID, h.state.IsProofUsed(hash))
}

func (h *Handlers) getVoteCount(req *Request) *Response {
	var params struct {
		PayLinkID string `json:"paylinkId"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	id := types.HexToHash(params.PayLinkID)
	return NewResponse(req.ID, h.state.GetVoteCount(id))
}

func (h *Handlers) pendingTransactions(req *Request) *Response {
	pending := h.mempool.Pending()
	if pending == nil {
		pending = []*types.Transaction{}
	}
	return NewResponse(req.ID, pending)
}

// ── New RPC handlers ──

func (h *Handlers) getTransactionReceipt(req *Request) *Response {
	var params struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	hash := types.HexToHash(params.Hash)
	receipt, err := h.blockchain.GetReceipt(hash)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	if receipt == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "receipt not found")
	}

	return NewResponse(req.ID, TxReceiptResponse{
		TxHash:      receipt.TxHash.Hex(),
		Success:     receipt.Success,
		Error:       receipt.Error,
		BlockHeight: receipt.BlockHeight,
		TxIndex:     receipt.TxIndex,
	})
}

func (h *Handlers) getNonce(req *Request) *Response {
	var params struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	addr := types.HexToAddress(params.Address)
	nonce := h.state.GetNonce(addr)
	return NewResponse(req.ID, nonce)
}

func (h *Handlers) getPayLinksByCreator(req *Request) *Response {
	var params struct {
		Creator string `json:"creator"`
		Limit   int    `json:"limit,omitempty"`
		Offset  int    `json:"offset,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	addr := types.HexToAddress(params.Creator)
	ids := h.state.GetPayLinksByCreator(addr)
	paylinks := h.resolvePayLinks(ids, params.Limit, params.Offset)
	return NewResponse(req.ID, paylinks)
}

func (h *Handlers) getPayLinksByReceiver(req *Request) *Response {
	var params struct {
		Receiver string `json:"receiver"`
		Limit    int    `json:"limit,omitempty"`
		Offset   int    `json:"offset,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	addr := types.HexToAddress(params.Receiver)
	ids := h.state.GetPayLinksByReceiver(addr)
	paylinks := h.resolvePayLinks(ids, params.Limit, params.Offset)
	return NewResponse(req.ID, paylinks)
}

func (h *Handlers) getPayLinksByStatus(req *Request) *Response {
	var params struct {
		Status string `json:"status"`
		Limit  int    `json:"limit,omitempty"`
		Offset int    `json:"offset,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	status := parseStatus(params.Status)
	if status == types.StatusNone && params.Status != "NONE" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams,
			fmt.Sprintf("invalid status: %s (use CREATED, VERIFIED, FAILED, CANCELLED)", params.Status))
	}

	ids := h.state.GetPayLinksByStatus(status)
	paylinks := h.resolvePayLinks(ids, params.Limit, params.Offset)
	return NewResponse(req.ID, paylinks)
}

func (h *Handlers) getBlockRange(req *Request) *Response {
	var params struct {
		FromHeight uint64 `json:"fromHeight"`
		ToHeight   uint64 `json:"toHeight"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	if params.ToHeight < params.FromHeight {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "toHeight must be >= fromHeight")
	}
	// Cap at 100 blocks per request
	if params.ToHeight-params.FromHeight > 100 {
		params.ToHeight = params.FromHeight + 100
	}

	var blocks []BlockResponse
	for height := params.FromHeight; height <= params.ToHeight; height++ {
		block, err := h.blockchain.GetBlockByHeight(height)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInternal, err.Error())
		}
		if block == nil {
			break
		}
		blocks = append(blocks, toBlockResponse(block))
	}

	if blocks == nil {
		blocks = []BlockResponse{}
	}
	return NewResponse(req.ID, blocks)
}

func (h *Handlers) getBlockTransactions(req *Request) *Response {
	var params struct {
		Height uint64 `json:"height"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	block, err := h.blockchain.GetBlockByHeight(params.Height)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	if block == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "block not found")
	}

	return NewResponse(req.ID, block.Transactions)
}

func (h *Handlers) hasVoted(req *Request) *Response {
	var params struct {
		PayLinkID string `json:"paylinkId"`
		Validator string `json:"validator"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	plId := types.HexToHash(params.PayLinkID)
	validator := types.HexToAddress(params.Validator)
	return NewResponse(req.ID, h.state.HasVoted(plId, validator))
}

func (h *Handlers) getVoters(req *Request) *Response {
	var params struct {
		PayLinkID string `json:"paylinkId"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}

	plId := types.HexToHash(params.PayLinkID)
	voters := h.state.GetVotersForPayLink(plId)

	voterHexes := make([]string, len(voters))
	for i, v := range voters {
		voterHexes[i] = v.Hex()
	}
	return NewResponse(req.ID, voterHexes)
}

func (h *Handlers) stakingStats(req *Request) *Response {
	return NewResponse(req.ID, StakingStatsResponse{
		TotalStaked:          h.state.TotalStaked(),
		ActiveValidatorCount: h.state.ActiveValidatorCount(),
		TotalValidatorCount:  h.state.GetValidatorCount(),
		MinimumStake:         h.state.MinimumStake(),
		WithdrawalCooldown:   h.state.WithdrawalCooldown(),
	})
}

func (h *Handlers) tokenStats(req *Request) *Response {
	return NewResponse(req.ID, TokenStatsResponse{
		TotalSupply: h.state.TotalSupply(),
		MaxSupply:   h.state.MaxSupply(),
	})
}

func (h *Handlers) chainStats(req *Request) *Response {
	return NewResponse(req.ID, ChainStatsResponse{
		ChainID:             h.blockchain.Genesis().ChainID,
		Height:              h.blockchain.Height(),
		TotalAccounts:       h.state.AccountCount(),
		TotalPayLinks:       h.state.PayLinkCount(),
		TotalValidators:     h.state.GetValidatorCount(),
		TotalProofsUsed:     h.state.UsedProofCount(),
		RequiredValidations: h.state.RequiredValidations(),
	})
}

func (h *Handlers) nodeInfo(req *Request) *Response {
	return NewResponse(req.ID, NodeInfoResponse{
		ChainID:     h.blockchain.Genesis().ChainID,
		NodeVersion: h.nodeVersion,
		Height:      h.blockchain.Height(),
		AdminAddr:   h.blockchain.Genesis().AdminAddress.Hex(),
		RPCAddr:     h.rpcAddr,
	})
}

// ── Query helpers ──

func (h *Handlers) resolvePayLinks(ids []types.Hash, limit, offset int) []PayLinkResponse {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// Apply pagination
	if offset >= len(ids) {
		return []PayLinkResponse{}
	}
	end := offset + limit
	if end > len(ids) {
		end = len(ids)
	}
	ids = ids[offset:end]

	result := make([]PayLinkResponse, 0, len(ids))
	for _, id := range ids {
		pl := h.state.GetPayLink(id)
		if pl != nil {
			result = append(result, PayLinkResponse{
				ID:            pl.ID.Hex(),
				Creator:       pl.Creator.Hex(),
				Receiver:      pl.Receiver.Hex(),
				Owner:         pl.Owner.Hex(),
				Approved:      pl.Approved.Hex(),
				Amount:        pl.Amount,
				Expiry:        pl.Expiry,
				Status:        pl.Status.String(),
				MetadataHash:  pl.MetadataHash.Hex(),
				CreatedAt:     pl.CreatedAt,
				VoteCount:     pl.VoteCount,
				TransferCount: pl.TransferCount,
				Rules:         pl.Rules,
			})
		}
	}
	return result
}

func parseStatus(s string) types.Status {
	switch s {
	case "NONE":
		return types.StatusNone
	case "CREATED":
		return types.StatusCreated
	case "VERIFIED":
		return types.StatusVerified
	case "FAILED":
		return types.StatusFailed
	case "CANCELLED":
		return types.StatusCancelled
	default:
		return types.StatusNone
	}
}

// ── Conversion helpers ──

func toBlockResponse(b *types.Block) BlockResponse {
	return BlockResponse{
		Height:       b.Header.Height,
		Timestamp:    b.Header.Timestamp,
		PreviousHash: b.Header.PreviousHash.Hex(),
		StateRoot:    b.Header.StateRoot.Hex(),
		TxRoot:       b.Header.TxRoot.Hex(),
		Proposer:     b.Header.ProposerAddr.Hex(),
		Hash:         b.Hash.Hex(),
		TxCount:      len(b.Transactions),
	}
}

// ── NFT-style PayLink queries ──

func (h *Handlers) getPayLinksByOwner(req *Request) *Response {
	var params struct {
		Owner  string `json:"owner"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	owner := types.HexToAddress(params.Owner)
	ids := h.state.GetPayLinksByOwner(owner)
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	return NewResponse(req.ID, h.resolvePayLinks(ids, limit, params.Offset))
}

func (h *Handlers) ownerOf(req *Request) *Response {
	var params struct {
		PayLinkID string `json:"paylinkId"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	id := types.HexToHash(params.PayLinkID)
	owner, ok := h.state.GetPayLinkOwner(id)
	if !ok {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "paylink not found")
	}
	return NewResponse(req.ID, map[string]string{"owner": owner.Hex()})
}

func (h *Handlers) balanceOf(req *Request) *Response {
	var params struct {
		Owner string `json:"owner"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	owner := types.HexToAddress(params.Owner)
	count := h.state.OwnerPayLinkCount(owner)
	return NewResponse(req.ID, map[string]int{"balance": count})
}

func (h *Handlers) getApproved(req *Request) *Response {
	var params struct {
		PayLinkID string `json:"paylinkId"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	id := types.HexToHash(params.PayLinkID)
	pl := h.state.GetPayLink(id)
	if pl == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "paylink not found")
	}
	return NewResponse(req.ID, map[string]string{"approved": pl.Approved.Hex()})
}

func (h *Handlers) isApprovedForAll(req *Request) *Response {
	var params struct {
		Owner    string `json:"owner"`
		Operator string `json:"operator"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	owner := types.HexToAddress(params.Owner)
	operator := types.HexToAddress(params.Operator)
	approved := h.state.IsOperatorApproved(owner, operator)
	return NewResponse(req.ID, map[string]bool{"approved": approved})
}

func (h *Handlers) getPayLinkRules(req *Request) *Response {
	var params struct {
		PayLinkID string `json:"paylinkId"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	id := types.HexToHash(params.PayLinkID)
	pl := h.state.GetPayLink(id)
	if pl == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "paylink not found")
	}
	return NewResponse(req.ID, map[string]json.RawMessage{"rules": pl.Rules})
}

func toValidatorResponse(v *types.ValidatorInfo) ValidatorResponse {
	return ValidatorResponse{
		Address:           v.Address.Hex(),
		StakedAmount:      v.StakedAmount,
		PendingWithdrawal: v.PendingWithdrawal,
		WithdrawableAt:    v.WithdrawableAt,
		TotalSlashed:      v.TotalSlashed,
		TotalRewards:      v.TotalRewards,
		IsActive:          v.IsActive,
		JoinedAt:          v.JoinedAt,
	}
}
