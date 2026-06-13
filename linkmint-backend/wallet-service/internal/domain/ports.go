package domain

import (
	"context"
	"errors"

	"github.com/paylink/wallet-service/internal/chainrpc"
)

// Domain sentinel errors. The server maps these to the HTTP envelope; the store/service return them.
var (
	// ErrNotFound is returned when a requested wallet/position/reward set has no rows.
	ErrNotFound = errors.New("not found")
	// ErrInvalidAddress is returned for a malformed 0x-address.
	ErrInvalidAddress = errors.New("invalid address")
	// ErrInvalidAmount is returned for a non-positive or out-of-range amount.
	ErrInvalidAmount = errors.New("invalid amount")
	// ErrInvalidAction is returned for an unknown staking-intent action.
	ErrInvalidAction = errors.New("invalid action")
	// ErrChainUnavailable is returned when the chain RPC is required but unreachable.
	ErrChainUnavailable = errors.New("chain unavailable")
)

// Store is the persistence port: read queries over the indexed `wallet` schema plus the idempotent
// projection writes the consumer drives. Each Record* returns ran=false when the event was a
// duplicate (already applied, suppressed by DbDedupe) and ran=true when it was applied this call.
type Store interface {
	// Read-side queries.
	GetAccountCache(ctx context.Context, addr string) (Account, bool, error)
	UpsertAccountCache(ctx context.Context, a Account) error
	ListTransactions(ctx context.Context, addr string, limit int, cursor string) ([]Transaction, string, error)
	GetPosition(ctx context.Context, addr string) (Position, bool, error)
	ListRewards(ctx context.Context, addr string, limit int, cursor string) ([]Reward, string, error)
	GetTreasuryStats(ctx context.Context) (TreasuryStats, error)

	// Indexer projections (idempotent via DbDedupe on the same tx as the write).
	RecordTransfer(ctx context.Context, ev TransferEvent) (bool, error)
	RecordStaked(ctx context.Context, ev StakedEvent) (bool, error)
	RecordUnstakeStarted(ctx context.Context, ev UnstakeStartedEvent) (bool, error)
	RecordUnstakeCompleted(ctx context.Context, ev UnstakeCompletedEvent) (bool, error)
	RecordSlashed(ctx context.Context, ev SlashedEvent) (bool, error)
	RecordRewarded(ctx context.Context, ev RewardedEvent) (bool, error)
	RecordFeeCollected(ctx context.Context, ev FeeCollectedEvent) (bool, error)
	RecordFeeDistributed(ctx context.Context, ev FeeDistributedEvent) (bool, error)
	RecordTokenBurned(ctx context.Context, ev TokenBurnedEvent) (bool, error)

	Ping(ctx context.Context) error
}

// ChainReader is the read-through to on-chain truth (the subset the service needs). Both
// *chainrpc.Client and *chainrpc.FakeClient satisfy it.
type ChainReader interface {
	GetAccount(ctx context.Context, addr string) (chainrpc.Account, error)
	GetNonce(ctx context.Context, addr string) (uint64, error)
	ChainInfo(ctx context.Context) (chainrpc.ChainInfo, error)
	TokenStats(ctx context.Context) (chainrpc.TokenStats, error)
	Ping(ctx context.Context) error
}

// Recorder records service metrics (nil-safe at the call sites).
type Recorder interface {
	IntentBuilt(action string)
}
