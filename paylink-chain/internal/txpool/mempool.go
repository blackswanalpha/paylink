package txpool

import (
	"bytes"
	"fmt"
	"sort"
	"sync"

	"github.com/paylink/paylink-chain/internal/types"
)

// Mempool holds pending transactions waiting to be included in a block.
type Mempool struct {
	mu      sync.RWMutex
	pending map[types.Hash]*types.Transaction      // hash -> tx
	byNonce map[types.Address][]*types.Transaction // sender -> txs sorted by nonce
	maxSize int
}

// NewMempool creates a new mempool with the given max size.
func NewMempool(maxSize int) *Mempool {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &Mempool{
		pending: make(map[types.Hash]*types.Transaction),
		byNonce: make(map[types.Address][]*types.Transaction),
		maxSize: maxSize,
	}
}

// Add adds a transaction to the mempool.
func (m *Mempool) Add(tx *types.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pending[tx.Hash]; exists {
		return fmt.Errorf("transaction already in mempool: %s", tx.Hash)
	}

	if len(m.pending) >= m.maxSize {
		return fmt.Errorf("mempool full: %d transactions", m.maxSize)
	}

	m.pending[tx.Hash] = tx

	// Insert sorted by nonce
	txs := m.byNonce[tx.From]
	txs = append(txs, tx)
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].Nonce < txs[j].Nonce
	})
	m.byNonce[tx.From] = txs

	return nil
}

// Remove removes a transaction from the mempool.
func (m *Mempool) Remove(hash types.Hash) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, exists := m.pending[hash]
	if !exists {
		return
	}

	delete(m.pending, hash)

	// Remove from sender's nonce-sorted list
	txs := m.byNonce[tx.From]
	for i, t := range txs {
		if t.Hash == hash {
			m.byNonce[tx.From] = append(txs[:i], txs[i+1:]...)
			break
		}
	}
	if len(m.byNonce[tx.From]) == 0 {
		delete(m.byNonce, tx.From)
	}
}

// Pending returns all pending transactions, ordered by sender nonce.
func (m *Mempool) Pending() []*types.Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*types.Transaction
	for _, txs := range m.byNonce {
		result = append(result, txs...)
	}
	return result
}

// DrainForBlock returns up to maxTxs transactions for block production and removes them.
// Senders are visited in address order (map iteration would randomize which txs make
// the block when it's full), with each sender's txs in nonce order.
func (m *Mempool) DrainForBlock(maxTxs int) []types.Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()

	senders := make([]types.Address, 0, len(m.byNonce))
	for from := range m.byNonce {
		senders = append(senders, from)
	}
	sort.Slice(senders, func(i, j int) bool {
		return bytes.Compare(senders[i][:], senders[j][:]) < 0
	})

	var result []types.Transaction
	var toRemove []types.Hash

	for _, from := range senders {
		for _, tx := range m.byNonce[from] {
			if len(result) >= maxTxs {
				break
			}
			result = append(result, *tx)
			toRemove = append(toRemove, tx.Hash)
		}
		if len(result) >= maxTxs {
			break
		}
	}

	// Remove drained txs
	for _, hash := range toRemove {
		tx := m.pending[hash]
		delete(m.pending, hash)
		if tx != nil {
			txs := m.byNonce[tx.From]
			for i, t := range txs {
				if t.Hash == hash {
					m.byNonce[tx.From] = append(txs[:i], txs[i+1:]...)
					break
				}
			}
			if len(m.byNonce[tx.From]) == 0 {
				delete(m.byNonce, tx.From)
			}
		}
	}

	return result
}

// Has checks if a transaction is in the mempool.
func (m *Mempool) Has(hash types.Hash) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.pending[hash]
	return exists
}

// Count returns the number of pending transactions.
func (m *Mempool) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pending)
}

// ReinsertAll adds transactions back to the mempool (for reverted blocks).
func (m *Mempool) ReinsertAll(txs []types.Transaction) {
	for i := range txs {
		_ = m.Add(&txs[i])
	}
}
