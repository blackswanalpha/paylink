package chain

import (
	"context"
	"sync"
)

// NonceReader reads the on-chain nonce for an address (satisfied by *Client).
type NonceReader interface {
	GetNonce(ctx context.Context, address string) (uint64, error)
}

// NonceManager serializes nonce assignment for the validator's signer so concurrent submissions
// never collide. It mirrors paylink-service/app/chain/nonce.py: take max(chainNonce, localNext)
// and advance the local counter only when the submission succeeds (a failed submit leaves no gap).
//
// Reserve holds the lock until the returned commit func is called, so submissions are serialized;
// for the single-validator MVP there is exactly one signer, so this is intra-process only.
//
// The local counter advances on mempool-accept (send success), not on block-commit, so two
// back-to-back settlements can be in the mempool with nonces N and N+1 before either is mined.
// That is safe on this chain: the mempool stores each sender's txs nonce-sorted and the block
// producer drains all pending (up to 500) per block in order, so the executor applies N (bumping
// the committed nonce) then N+1 within the same block — both settle. (Verified against
// paylink-chain/internal/txpool + internal/consensus/block_producer.go.)
type NonceManager struct {
	chain NonceReader
	mu    sync.Mutex
	next  map[string]uint64
}

// NewNonceManager builds a NonceManager over a chain nonce reader.
func NewNonceManager(chain NonceReader) *NonceManager {
	return &NonceManager{chain: chain, next: map[string]uint64{}}
}

// Reserve locks, reads the chain nonce, and returns the nonce to use plus a commit func. The
// caller MUST call commit(true) after a successful submit (advances the local counter and unlocks)
// or commit(false) on failure (unlocks without advancing, so the nonce is reused). On error the
// lock is already released and commit is nil.
func (m *NonceManager) Reserve(ctx context.Context, address string) (uint64, func(bool), error) {
	m.mu.Lock()
	chainNonce, err := m.chain.GetNonce(ctx, address)
	if err != nil {
		m.mu.Unlock()
		return 0, nil, err
	}
	n := chainNonce
	if local := m.next[address]; local > n {
		n = local
	}
	var once sync.Once
	commit := func(ok bool) {
		once.Do(func() {
			if ok {
				m.next[address] = n + 1
			}
			m.mu.Unlock()
		})
	}
	return n, commit, nil
}
