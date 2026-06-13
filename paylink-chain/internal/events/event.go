package events

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/paylink/paylink-chain/internal/fsm"
)

// EventKind identifies the category of event.
type EventKind string

const (
	// PayLink events
	EventPayLinkCreated        EventKind = "paylink.created"
	EventPayLinkVoted          EventKind = "paylink.voted"
	EventPayLinkVerified       EventKind = "paylink.verified"
	EventPayLinkCancelled      EventKind = "paylink.cancelled"
	EventPayLinkFailed         EventKind = "paylink.failed"
	EventPayLinkTransferred    EventKind = "paylink.transferred"
	EventPayLinkApproved       EventKind = "paylink.approved"
	EventPayLinkApprovalForAll EventKind = "paylink.approval_for_all"

	// Validator events
	EventValidatorStaked           EventKind = "validator.staked"
	EventValidatorActivated        EventKind = "validator.activated"
	EventValidatorDeactivated      EventKind = "validator.deactivated"
	EventValidatorUnstakeStarted   EventKind = "validator.unstake_started"
	EventValidatorUnstakeComplete  EventKind = "validator.unstake_completed"
	EventValidatorSlashed          EventKind = "validator.slashed"
	EventValidatorRewarded         EventKind = "validator.rewarded"
	EventValidatorVRFKeyRegistered EventKind = "validator.vrf_key_registered"

	// Fee events (Phase 2)
	EventFeeCollected   EventKind = "fee.collected"
	EventFeeDistributed EventKind = "fee.distributed"
	EventTokenBurned    EventKind = "token.burned"

	// Account events
	EventTransfer EventKind = "account.transfer"

	// Block events
	EventBlockProduced EventKind = "block.produced"

	// Transaction events
	EventTxExecuted EventKind = "tx.executed"
	EventTxFailed   EventKind = "tx.failed"
)

// EntityType categorizes which domain an event belongs to.
type EntityType string

const (
	EntityPayLink   EntityType = "paylink"
	EntityValidator EntityType = "validator"
	EntityAccount   EntityType = "account"
	EntityBlock     EntityType = "block"
	EntityTx        EntityType = "tx"
)

// global monotonic sequence counter
var eventSeq uint64

// Event is the unified event structure emitted by the datastream.
type Event struct {
	Sequence    uint64             `json:"seq"`
	Kind        EventKind          `json:"kind"`
	EntityType  EntityType         `json:"entityType"`
	EntityID    string             `json:"entityId"`
	Timestamp   int64              `json:"timestamp"`
	BlockHeight uint64             `json:"blockHeight"`
	TxHash      string             `json:"txHash,omitempty"`
	FromState   fsm.State          `json:"fromState,omitempty"`
	ToState     fsm.State          `json:"toState,omitempty"`
	Transition  fsm.TransitionKind `json:"transition,omitempty"`
	Data        json.RawMessage    `json:"data,omitempty"`
}

// NewEvent creates a new event with a monotonic sequence number.
func NewEvent(kind EventKind, entityType EntityType, entityID string, blockHeight uint64) *Event {
	return &Event{
		Sequence:    atomic.AddUint64(&eventSeq, 1),
		Kind:        kind,
		EntityType:  entityType,
		EntityID:    entityID,
		Timestamp:   time.Now().UnixMilli(),
		BlockHeight: blockHeight,
	}
}

// WithTransition attaches FSM transition metadata.
func (e *Event) WithTransition(from, to fsm.State, kind fsm.TransitionKind) *Event {
	e.FromState = from
	e.ToState = to
	e.Transition = kind
	return e
}

// WithTx attaches a transaction hash.
func (e *Event) WithTx(txHash string) *Event {
	e.TxHash = txHash
	return e
}

// WithData attaches arbitrary JSON payload.
func (e *Event) WithData(v interface{}) *Event {
	data, _ := json.Marshal(v)
	e.Data = data
	return e
}

// ── Event-specific data payloads ──

type PayLinkCreatedData struct {
	Creator      string `json:"creator"`
	Receiver     string `json:"receiver"`
	Amount       uint64 `json:"amount"`
	Expiry       int64  `json:"expiry"`
	MetadataHash string `json:"metadataHash"`
}

type PayLinkVotedData struct {
	Validator string `json:"validator"`
	ProofHash string `json:"proofHash"`
	VoteCount uint64 `json:"voteCount"`
	Required  uint64 `json:"required"`
}

type PayLinkSettledData struct {
	ProofHash string `json:"proofHash"`
	VoteCount uint64 `json:"voteCount"`
	// Payee is the PayLink Receiver address (hex). It identifies the merchant the settled
	// PayLink pays out to, so off-chain consumers (settlement-service, work23) can aggregate
	// verified PayLinks into a per-merchant settlement without a separate lookup. Observability
	// metadata only — not part of consensus state or block/state hashing.
	Payee string `json:"payee"`
	// Amount is the PayLink's gross amount in minor units, mirrored here so settlement can
	// record gross/net the moment a PayLink verifies (the fee.collected event carries the fee).
	Amount uint64 `json:"amount"`
}

type TransferData struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
}

type ValidatorStakeData struct {
	Amount      uint64 `json:"amount"`
	TotalStaked uint64 `json:"totalStaked"`
	IsActive    bool   `json:"isActive"`
}

type ValidatorSlashData struct {
	Amount    uint64 `json:"amount"`
	Reason    string `json:"reason"`
	Remaining uint64 `json:"remaining"`
}

type ValidatorRewardData struct {
	Amount       uint64 `json:"amount"`
	TotalRewards uint64 `json:"totalRewards"`
}

type ValidatorUnstakeData struct {
	Amount         uint64 `json:"amount"`
	WithdrawableAt int64  `json:"withdrawableAt"`
}

type BlockProducedData struct {
	Hash         string `json:"hash"`
	Height       uint64 `json:"height"`
	TxCount      int    `json:"txCount"`
	Proposer     string `json:"proposer"`
	StateRoot    string `json:"stateRoot"`
	PreviousHash string `json:"previousHash"`
}

type TxExecutedData struct {
	TxType  uint8  `json:"txType"`
	From    string `json:"from"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	TxIndex int    `json:"txIndex"`
}

// Phase 2 fee event data

type FeeCollectedData struct {
	PayLinkID      string `json:"paylinkId"`
	Amount         uint64 `json:"amount"`
	TotalFee       uint64 `json:"totalFee"`
	ValidatorShare uint64 `json:"validatorShare"`
	TreasuryShare  uint64 `json:"treasuryShare"`
	BurnAmount     uint64 `json:"burnAmount"`
}

type FeeDistributedData struct {
	Validator string `json:"validator"`
	Amount    uint64 `json:"amount"`
}

type TokenBurnedData struct {
	Amount      uint64 `json:"amount"`
	TotalBurned uint64 `json:"totalBurned"`
}

// NFT ownership event data

type PayLinkTransferredData struct {
	From          string `json:"from"`
	To            string `json:"to"`
	Operator      string `json:"operator,omitempty"` // set if transferred by approved/operator
	TransferCount uint64 `json:"transferCount"`
}

type PayLinkApprovedData struct {
	Owner    string `json:"owner"`
	Approved string `json:"approved"`
}

type PayLinkApprovalForAllData struct {
	Owner    string `json:"owner"`
	Operator string `json:"operator"`
	Approved bool   `json:"approved"`
}
