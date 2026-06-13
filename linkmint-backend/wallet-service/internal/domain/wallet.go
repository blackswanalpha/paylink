// Package domain holds the wallet-service business logic: the read orchestration over the indexed
// `wallet` schema (transactions, staking positions/rewards, treasury stats), the read-through
// balance cache backed by the chain RPC, and the non-custodial unsigned staking-intent builder.
//
// NON-CUSTODIAL (A.1): the service never holds keys or funds. Balances are chain truth (read-through
// cached); staking intents are returned UNSIGNED for the client to sign.
package domain

import (
	"encoding/json"
	"math/big"
	"time"

	lvm "github.com/paylink/paylink-chain/pkg/lvm"
)

// Consumer result labels (wallet_events_consumed_total{result}).
const (
	ResultProcessed = "processed"
	ResultDuplicate = "duplicate"
	ResultIgnored   = "ignored"
	ResultError     = "error"
)

// Transaction kinds projected into wallet.transactions.
const (
	KindTransfer        = "transfer"
	KindStake           = "stake"
	KindUnstakeStart    = "unstake_start"
	KindUnstakeComplete = "unstake_complete"
	KindReward          = "reward"
	KindSlash           = "slash"
)

// Reward sources for wallet.staking_rewards.
const (
	SourceValidatorReward = "validator_reward"
	SourceFeeShare        = "fee_share"
)

// ── Read views ──

// Account is the read-through balance/nonce view of an address (cached from chain truth).
type Account struct {
	Addr        string
	Balance     *big.Int
	Nonce       uint64
	BlockHeight uint64
	FetchedAt   time.Time
	// Stale is true when the chain RPC was unreachable and this row was served from cache.
	Stale bool
}

// Transaction is one projected ledger movement touching an address.
type Transaction struct {
	ID           string
	Addr         string
	Counterparty string
	Direction    string // in|out|self
	Kind         string
	Amount       *big.Int
	TxHash       string
	BlockHeight  uint64
	OccurredAt   time.Time
}

// Position is an address's staking position (projected from chain.validator.* events).
type Position struct {
	Addr              string
	StakedAmount      *big.Int
	PendingWithdrawal *big.Int
	TotalRewards      *big.Int
	TotalSlashed      *big.Int
	WithdrawableAt    *time.Time
	IsActive          bool
	UpdatedAt         time.Time
}

// Reward is one entry in an address's append-only reward history.
type Reward struct {
	ID           string
	Addr         string
	Amount       *big.Int
	TotalRewards *big.Int
	Source       string
	TxHash       string
	BlockHeight  uint64
	OccurredAt   time.Time
}

// TreasuryStats is the public running aggregate of supply, burn, fees, and treasury/validator share.
type TreasuryStats struct {
	TotalSupply      *big.Int
	MaxSupply        *big.Int
	TotalBurned      *big.Int
	FeesCollected    *big.Int
	ValidatorRewards *big.Int
	TreasuryAmount   *big.Int
	ChainHeight      uint64
	UpdatedAt        time.Time
}

// ── Staking intent (unsigned) ──

// IntentRequest is the input to BuildIntent.
type IntentRequest struct {
	Addr   string
	Action string // stake|unstake
	Amount *big.Int
}

// FeeEstimate is the structured fee surface of an intent. Stake/unstake txs carry no protocol fee
// today (the only fee is the 0.5% settlement inflation fee on PayLink verification), so Amount is 0,
// with Policy documenting why — the shape is forward-compatible with a real estimate later.
type FeeEstimate struct {
	Amount   *big.Int
	Currency string
	Policy   string
}

// Intent is an UNSIGNED transaction plus the bytes the client must sign and a fee estimate. No key
// material is ever attached (A.1).
type Intent struct {
	Tx            *lvm.Transaction
	SignableBytes []byte
	Nonce         uint64
	ChainID       string
	FeeEstimate   FeeEstimate
}

// UnsignedJSON marshals the unsigned transaction for the API response.
func (i Intent) UnsignedJSON() (json.RawMessage, error) {
	b, err := json.Marshal(i.Tx)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// ── Chain event inputs (decoded by the consumer, applied by the store) ──

// TransferEvent is chain.account.transfer.
type TransferEvent struct {
	From        string
	To          string
	Amount      *big.Int
	TxHash      string
	BlockHeight uint64
	OccurredAt  time.Time
}

// StakedEvent is chain.validator.staked.
type StakedEvent struct {
	Addr        string
	Amount      *big.Int
	TotalStaked *big.Int
	IsActive    bool
	TxHash      string
	BlockHeight uint64
	OccurredAt  time.Time
}

// UnstakeStartedEvent is chain.validator.unstake_started.
type UnstakeStartedEvent struct {
	Addr           string
	Amount         *big.Int
	WithdrawableAt *time.Time
	TxHash         string
	BlockHeight    uint64
	OccurredAt     time.Time
}

// UnstakeCompletedEvent is chain.validator.unstake_completed.
type UnstakeCompletedEvent struct {
	Addr        string
	Amount      *big.Int
	TxHash      string
	BlockHeight uint64
	OccurredAt  time.Time
}

// SlashedEvent is chain.validator.slashed.
type SlashedEvent struct {
	Addr        string
	Amount      *big.Int
	Remaining   *big.Int
	Reason      string
	TxHash      string
	BlockHeight uint64
	OccurredAt  time.Time
}

// RewardedEvent is chain.validator.rewarded.
type RewardedEvent struct {
	Addr         string
	Amount       *big.Int
	TotalRewards *big.Int
	TxHash       string
	BlockHeight  uint64
	OccurredAt   time.Time
}

// FeeCollectedEvent is chain.fee.collected.
type FeeCollectedEvent struct {
	TotalFee       *big.Int
	ValidatorShare *big.Int
	TreasuryShare  *big.Int
	BurnAmount     *big.Int
	TxHash         string
	BlockHeight    uint64
	OccurredAt     time.Time
}

// FeeDistributedEvent is chain.fee.distributed.
type FeeDistributedEvent struct {
	Validator   string
	Amount      *big.Int
	TxHash      string
	BlockHeight uint64
	OccurredAt  time.Time
}

// TokenBurnedEvent is chain.token.burned (TotalBurned is the authoritative cumulative figure).
type TokenBurnedEvent struct {
	Amount      *big.Int
	TotalBurned *big.Int
	TxHash      string
	BlockHeight uint64
	OccurredAt  time.Time
}
