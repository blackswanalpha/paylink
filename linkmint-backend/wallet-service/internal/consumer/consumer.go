// Package consumer is the chain indexer: it bridges the work15 event bus to the wallet read-side. It
// subscribes to the `chain` topic and projects each chain.* event into the `wallet` schema:
//
//	chain.account.transfer            → transaction history (out on sender, in on receiver)
//	chain.validator.staked            → staking position + stake history
//	chain.validator.unstake_started   → move stake to pending + history
//	chain.validator.unstake_completed → clear pending + history
//	chain.validator.slashed           → reduce stake + slash history
//	chain.validator.rewarded          → cumulative rewards + reward history
//	chain.fee.collected               → treasury fee/validator/treasury deltas
//	chain.fee.distributed             → per-validator fee-share reward history
//	chain.token.burned                → authoritative cumulative burn
//
// Contract (eventbus-go): returning an error means "not handled" — the offset is NOT committed and
// the event redelivers, so handlers MUST be idempotent (DbDedupe in the store, work17). Unknown names
// are committed untouched; undecodable payloads are logged + skipped (poison-safe).
package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/big"
	"time"

	"github.com/paylink/wallet-service/internal/domain"
)

// Consumed event logical names.
const (
	EventTransfer         = "chain.account.transfer"
	EventStaked           = "chain.validator.staked"
	EventUnstakeStarted   = "chain.validator.unstake_started"
	EventUnstakeCompleted = "chain.validator.unstake_completed"
	EventSlashed          = "chain.validator.slashed"
	EventRewarded         = "chain.validator.rewarded"
	EventFeeCollected     = "chain.fee.collected"
	EventFeeDistributed   = "chain.fee.distributed"
	EventTokenBurned      = "chain.token.burned"
)

// Topics the consumer subscribes to.
var Topics = []string{"chain"}

// Service is the domain surface the consumer drives.
type Service interface {
	HandleTransfer(ctx context.Context, ev domain.TransferEvent) (string, error)
	HandleStaked(ctx context.Context, ev domain.StakedEvent) (string, error)
	HandleUnstakeStarted(ctx context.Context, ev domain.UnstakeStartedEvent) (string, error)
	HandleUnstakeCompleted(ctx context.Context, ev domain.UnstakeCompletedEvent) (string, error)
	HandleSlashed(ctx context.Context, ev domain.SlashedEvent) (string, error)
	HandleRewarded(ctx context.Context, ev domain.RewardedEvent) (string, error)
	HandleFeeCollected(ctx context.Context, ev domain.FeeCollectedEvent) (string, error)
	HandleFeeDistributed(ctx context.Context, ev domain.FeeDistributedEvent) (string, error)
	HandleTokenBurned(ctx context.Context, ev domain.TokenBurnedEvent) (string, error)
}

// Recorder records wallet_events_consumed_total{result} (nil-safe).
type Recorder interface {
	EventConsumed(result string)
}

// Handler filters and dispatches bus events. Its Handle matches eventbus.HandleFunc.
type Handler struct {
	svc Service
	m   Recorder
	log *slog.Logger
}

// New builds a Handler (log may be nil → slog.Default; m may be nil → no metrics).
func New(svc Service, m Recorder, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{svc: svc, m: m, log: log}
}

// chainPayload is the chain-event-mirror projected envelope payload (the fields the indexer needs).
type chainPayload struct {
	EntityID    string          `json:"entity_id"`
	TxHash      string          `json:"tx_hash"`
	BlockHeight uint64          `json:"block_height"`
	Timestamp   int64           `json:"timestamp"` // unix milliseconds (chain event time)
	Data        json.RawMessage `json:"data"`
}

type transferData struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
}

type stakedData struct {
	Amount      uint64 `json:"amount"`
	TotalStaked uint64 `json:"totalStaked"`
	IsActive    bool   `json:"isActive"`
}

type unstakeData struct {
	Amount         uint64 `json:"amount"`
	WithdrawableAt int64  `json:"withdrawableAt"`
}

type slashData struct {
	Amount    uint64 `json:"amount"`
	Reason    string `json:"reason"`
	Remaining uint64 `json:"remaining"`
}

type rewardData struct {
	Amount       uint64 `json:"amount"`
	TotalRewards uint64 `json:"totalRewards"`
}

type feeCollectedData struct {
	TotalFee       uint64 `json:"totalFee"`
	ValidatorShare uint64 `json:"validatorShare"`
	TreasuryShare  uint64 `json:"treasuryShare"`
	BurnAmount     uint64 `json:"burnAmount"`
}

type feeDistributedData struct {
	Validator string `json:"validator"`
	Amount    uint64 `json:"amount"`
}

type tokenBurnedData struct {
	Amount      uint64 `json:"amount"`
	TotalBurned uint64 `json:"totalBurned"`
}

// Handle processes one bus event. A decode failure is poison-safe (logged + committed); a service
// error propagates (no commit → redelivery).
func (h *Handler) Handle(ctx context.Context, name string, payload json.RawMessage) error {
	switch name {
	case EventTransfer:
		return h.handleTransfer(ctx, payload)
	case EventStaked:
		return h.handleStaked(ctx, payload)
	case EventUnstakeStarted:
		return h.handleUnstakeStarted(ctx, payload)
	case EventUnstakeCompleted:
		return h.handleUnstakeCompleted(ctx, payload)
	case EventSlashed:
		return h.handleSlashed(ctx, payload)
	case EventRewarded:
		return h.handleRewarded(ctx, payload)
	case EventFeeCollected:
		return h.handleFeeCollected(ctx, payload)
	case EventFeeDistributed:
		return h.handleFeeDistributed(ctx, payload)
	case EventTokenBurned:
		return h.handleTokenBurned(ctx, payload)
	default:
		return nil // not ours — commit untouched
	}
}

func (h *Handler) handleTransfer(ctx context.Context, payload json.RawMessage) error {
	p, d, ok := decode[transferData](h, "transfer", payload)
	if !ok {
		return nil
	}
	return h.dispatch(h.svc.HandleTransfer(ctx, domain.TransferEvent{
		From: d.From, To: d.To, Amount: u(d.Amount),
		TxHash: p.TxHash, BlockHeight: p.BlockHeight, OccurredAt: msToTime(p.Timestamp),
	}))
}

func (h *Handler) handleStaked(ctx context.Context, payload json.RawMessage) error {
	p, d, ok := decode[stakedData](h, "staked", payload)
	if !ok {
		return nil
	}
	return h.dispatch(h.svc.HandleStaked(ctx, domain.StakedEvent{
		Addr: p.EntityID, Amount: u(d.Amount), TotalStaked: u(d.TotalStaked), IsActive: d.IsActive,
		TxHash: p.TxHash, BlockHeight: p.BlockHeight, OccurredAt: msToTime(p.Timestamp),
	}))
}

func (h *Handler) handleUnstakeStarted(ctx context.Context, payload json.RawMessage) error {
	p, d, ok := decode[unstakeData](h, "unstake_started", payload)
	if !ok {
		return nil
	}
	return h.dispatch(h.svc.HandleUnstakeStarted(ctx, domain.UnstakeStartedEvent{
		Addr: p.EntityID, Amount: u(d.Amount), WithdrawableAt: secToTimePtr(d.WithdrawableAt),
		TxHash: p.TxHash, BlockHeight: p.BlockHeight, OccurredAt: msToTime(p.Timestamp),
	}))
}

func (h *Handler) handleUnstakeCompleted(ctx context.Context, payload json.RawMessage) error {
	p, d, ok := decode[unstakeData](h, "unstake_completed", payload)
	if !ok {
		return nil
	}
	return h.dispatch(h.svc.HandleUnstakeCompleted(ctx, domain.UnstakeCompletedEvent{
		Addr: p.EntityID, Amount: u(d.Amount),
		TxHash: p.TxHash, BlockHeight: p.BlockHeight, OccurredAt: msToTime(p.Timestamp),
	}))
}

func (h *Handler) handleSlashed(ctx context.Context, payload json.RawMessage) error {
	p, d, ok := decode[slashData](h, "slashed", payload)
	if !ok {
		return nil
	}
	return h.dispatch(h.svc.HandleSlashed(ctx, domain.SlashedEvent{
		Addr: p.EntityID, Amount: u(d.Amount), Remaining: u(d.Remaining), Reason: d.Reason,
		TxHash: p.TxHash, BlockHeight: p.BlockHeight, OccurredAt: msToTime(p.Timestamp),
	}))
}

func (h *Handler) handleRewarded(ctx context.Context, payload json.RawMessage) error {
	p, d, ok := decode[rewardData](h, "rewarded", payload)
	if !ok {
		return nil
	}
	return h.dispatch(h.svc.HandleRewarded(ctx, domain.RewardedEvent{
		Addr: p.EntityID, Amount: u(d.Amount), TotalRewards: u(d.TotalRewards),
		TxHash: p.TxHash, BlockHeight: p.BlockHeight, OccurredAt: msToTime(p.Timestamp),
	}))
}

func (h *Handler) handleFeeCollected(ctx context.Context, payload json.RawMessage) error {
	p, d, ok := decode[feeCollectedData](h, "fee_collected", payload)
	if !ok {
		return nil
	}
	return h.dispatch(h.svc.HandleFeeCollected(ctx, domain.FeeCollectedEvent{
		TotalFee: u(d.TotalFee), ValidatorShare: u(d.ValidatorShare), TreasuryShare: u(d.TreasuryShare),
		BurnAmount: u(d.BurnAmount), TxHash: p.TxHash, BlockHeight: p.BlockHeight, OccurredAt: msToTime(p.Timestamp),
	}))
}

func (h *Handler) handleFeeDistributed(ctx context.Context, payload json.RawMessage) error {
	p, d, ok := decode[feeDistributedData](h, "fee_distributed", payload)
	if !ok {
		return nil
	}
	validator := d.Validator
	if validator == "" {
		validator = p.EntityID
	}
	return h.dispatch(h.svc.HandleFeeDistributed(ctx, domain.FeeDistributedEvent{
		Validator: validator, Amount: u(d.Amount),
		TxHash: p.TxHash, BlockHeight: p.BlockHeight, OccurredAt: msToTime(p.Timestamp),
	}))
}

func (h *Handler) handleTokenBurned(ctx context.Context, payload json.RawMessage) error {
	p, d, ok := decode[tokenBurnedData](h, "token_burned", payload)
	if !ok {
		return nil
	}
	return h.dispatch(h.svc.HandleTokenBurned(ctx, domain.TokenBurnedEvent{
		Amount: u(d.Amount), TotalBurned: u(d.TotalBurned),
		TxHash: p.TxHash, BlockHeight: p.BlockHeight, OccurredAt: msToTime(p.Timestamp),
	}))
}

// decode parses the chain envelope and its kind-specific data. ok is false (poison-safe) on a
// malformed envelope; a missing/empty data blob decodes to the zero value.
func decode[T any](h *Handler, kind string, payload json.RawMessage) (chainPayload, T, bool) {
	var p chainPayload
	var d T
	if err := json.Unmarshal(payload, &p); err != nil {
		h.poison(kind, err)
		return chainPayload{}, d, false
	}
	if len(p.Data) > 0 {
		_ = json.Unmarshal(p.Data, &d)
	}
	return p, d, true
}

// dispatch records the result metric and returns the service error (→ redelivery) if any.
func (h *Handler) dispatch(result string, err error) error {
	if err != nil {
		h.record(domain.ResultError)
		return err
	}
	h.record(result)
	return nil
}

// poison logs an undecodable payload and records it as ignored (the caller commits/skips it).
func (h *Handler) poison(kind string, err error) {
	h.log.Warn("wallet_event_decode_failed", "kind", kind, "err", err.Error())
	h.record(domain.ResultIgnored)
}

func (h *Handler) record(result string) {
	if h.m != nil {
		h.m.EventConsumed(result)
	}
}

func u(v uint64) *big.Int { return new(big.Int).SetUint64(v) }

func msToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func secToTimePtr(sec int64) *time.Time {
	if sec <= 0 {
		return nil
	}
	t := time.Unix(sec, 0).UTC()
	return &t
}
