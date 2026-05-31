package chain

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/storage"
	"github.com/paylink/paylink-chain/internal/types"
)

// Blockchain manages the chain of blocks.
type Blockchain struct {
	mu      sync.RWMutex
	store   storage.KVStore
	tip     *types.Block   // Current chain tip
	height  uint64
	genesis *types.GenesisConfig
}

// NewBlockchain creates a new blockchain with the given store and genesis.
func NewBlockchain(store storage.KVStore, genesis *types.GenesisConfig) *Blockchain {
	return &Blockchain{
		store:   store,
		genesis: genesis,
	}
}

// Init initializes the blockchain. If no blocks exist, creates genesis.
func (bc *Blockchain) Init(genesisBlock *types.Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Check if chain already exists
	heightBytes, err := bc.store.Get(storage.ChainMetaKey("height"))
	if err != nil {
		return fmt.Errorf("check chain height: %w", err)
	}

	if heightBytes != nil {
		// Load existing tip
		height := bytesToUint64(heightBytes)
		block, err := bc.getBlockByHeight(height)
		if err != nil {
			return fmt.Errorf("load tip block: %w", err)
		}
		bc.tip = block
		bc.height = height
		return nil
	}

	// Store genesis block
	return bc.storeBlock(genesisBlock)
}

// AddBlock validates and stores a new block.
func (bc *Blockchain) AddBlock(block *types.Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Validate block links to current tip
	if bc.tip != nil && block.Header.PreviousHash != bc.tip.Hash {
		return fmt.Errorf("block previous hash mismatch: expected %s, got %s",
			bc.tip.Hash, block.Header.PreviousHash)
	}

	expectedHeight := bc.height + 1
	if bc.tip == nil {
		expectedHeight = 0
	}
	if block.Header.Height != expectedHeight {
		return fmt.Errorf("block height mismatch: expected %d, got %d",
			expectedHeight, block.Header.Height)
	}

	// Compute and set block hash
	block.Hash = crypto.SHA256Hash(block.HeaderBytes())

	return bc.storeBlock(block)
}

// GetBlock returns a block by hash.
func (bc *Blockchain) GetBlock(hash types.Hash) (*types.Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	// Look up height by hash
	heightBytes, err := bc.store.Get(storage.BlockHashKey(hash))
	if err != nil {
		return nil, err
	}
	if heightBytes == nil {
		return nil, nil
	}
	height := bytesToUint64(heightBytes)
	return bc.getBlockByHeight(height)
}

// GetBlockByHeight returns a block by height.
func (bc *Blockchain) GetBlockByHeight(height uint64) (*types.Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getBlockByHeight(height)
}

// Tip returns the current chain tip block.
func (bc *Blockchain) Tip() *types.Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.tip
}

// Height returns the current chain height.
func (bc *Blockchain) Height() uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.height
}

// Genesis returns the genesis config.
func (bc *Blockchain) Genesis() *types.GenesisConfig {
	return bc.genesis
}

// StoreTx stores a transaction indexed by its hash.
func (bc *Blockchain) StoreTx(tx *types.Transaction) error {
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return bc.store.Set(storage.TxKey(tx.Hash), data)
}

// GetTx retrieves a transaction by hash.
func (bc *Blockchain) GetTx(hash types.Hash) (*types.Transaction, error) {
	data, err := bc.store.Get(storage.TxKey(hash))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var tx types.Transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// StoreReceipt stores a transaction receipt.
func (bc *Blockchain) StoreReceipt(receipt *TxReceipt) error {
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	return bc.store.Set(storage.ReceiptKey(receipt.TxHash), data)
}

// GetReceipt retrieves a transaction receipt by tx hash.
func (bc *Blockchain) GetReceipt(txHash types.Hash) (*TxReceipt, error) {
	data, err := bc.store.Get(storage.ReceiptKey(txHash))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var receipt TxReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

// StoreReceipts stores all receipts for a block.
func (bc *Blockchain) StoreReceipts(receipts []TxReceipt) error {
	for i := range receipts {
		if err := bc.StoreReceipt(&receipts[i]); err != nil {
			return err
		}
	}
	return nil
}

// ── Internal helpers ──

func (bc *Blockchain) storeBlock(block *types.Block) error {
	data, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("marshal block: %w", err)
	}

	batch := bc.store.NewBatch()
	batch.Set(storage.BlockKey(block.Header.Height), data)
	batch.Set(storage.BlockHashKey(block.Hash), uint64ToBytes(block.Header.Height))
	batch.Set(storage.ChainMetaKey("height"), uint64ToBytes(block.Header.Height))

	if err := batch.Flush(); err != nil {
		return fmt.Errorf("store block: %w", err)
	}

	// Store individual transactions
	for i := range block.Transactions {
		if err := bc.StoreTx(&block.Transactions[i]); err != nil {
			return fmt.Errorf("store tx: %w", err)
		}
	}

	bc.tip = block
	bc.height = block.Header.Height
	return nil
}

func (bc *Blockchain) getBlockByHeight(height uint64) (*types.Block, error) {
	data, err := bc.store.Get(storage.BlockKey(height))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var block types.Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

func uint64ToBytes(v uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
	return b
}

func bytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}
