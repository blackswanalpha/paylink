package domain

import (
	"context"
	"errors"
	"time"

	"github.com/paylink/paylink-chain/pkg/lvm"
	"github.com/paylink/proof-validator/internal/chain"
	"github.com/paylink/proof-validator/internal/proof"
)

// Store errors.
var (
	// ErrNotFound is returned when no proof matches the lookup.
	ErrNotFound = errors.New("proof not found")
	// ErrProofExists is returned by InsertProof when the proof_hash is already recorded
	// (one proof settles a PayLink exactly once — invariant A.7).
	ErrProofExists = errors.New("proof already recorded")
)

// ProofRecord is the persisted off-chain record of a submitted proof. proof_hash is the on-chain
// A.7 identity (lvm.ProofHash); the record is an audit trail + local anti-double-broadcast guard,
// not the source of truth (the chain is).
type ProofRecord struct {
	ProofHash string
	PayLinkID string
	Rail      string
	TxID      string
	Amount    uint64
	Status    string
	TxHash    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store persists proof records. Implementations: store/memory (tests, dev), store/postgres (prod).
type Store interface {
	// InsertProof inserts a new record, returning ErrProofExists on a duplicate proof_hash.
	InsertProof(ctx context.Context, r ProofRecord) error
	// MarkBroadcast records the settlement tx hash and advances the status.
	MarkBroadcast(ctx context.Context, proofHash, txHash, status string) error
	// GetByProofHash returns the record, or ErrNotFound.
	GetByProofHash(ctx context.Context, proofHash string) (ProofRecord, error)
	Ping(ctx context.Context) error
}

// ChainClient reads on-chain truth and broadcasts settlement transactions (the lVM JSON-RPC).
type ChainClient interface {
	// IsProofUsed reports whether the proof already settled a PayLink on-chain (A.7 truth).
	IsProofUsed(ctx context.Context, proofHash string) (bool, error)
	// GetPayLink returns on-chain PayLink state; found=false when unknown on-chain.
	GetPayLink(ctx context.Context, id string) (*chain.PayLinkState, bool, error)
	// SendTransaction broadcasts a signed tx and returns its hash.
	SendTransaction(ctx context.Context, tx *lvm.Transaction) (string, error)
}

// NonceReserver hands out the next nonce for an address, serialized across submissions.
type NonceReserver interface {
	Reserve(ctx context.Context, address string) (nonce uint64, commit func(ok bool), err error)
}

// Signer signs the settlement transaction and exposes the validator's address (the tx From).
type Signer interface {
	Address() lvm.Address
	SignTx(tx *lvm.Transaction) error
}

// ProofVerifier verifies the proof's signature (off-chain trust anchor).
type ProofVerifier interface {
	Verify(p proof.Proof) error
}

// Publisher emits domain events by logical name (transport seam — Kafka/SQS, ADR-004).
type Publisher interface {
	Publish(ctx context.Context, name, key string, payload any) error
}

// ProofMetrics is an optional metrics hook (nil-safe).
type ProofMetrics interface {
	ProofReceived(result string)
	SettlementTx(result string)
}
