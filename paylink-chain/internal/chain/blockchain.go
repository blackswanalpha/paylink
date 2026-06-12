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
	tip     *types.Block // Current chain tip
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
	return bc.storeBlock(genesisBlock, nil)
}

// AddBlock validates and stores a new block.
func (bc *Blockchain) AddBlock(block *types.Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if err := bc.checkLinkage(block); err != nil {
		return err
	}
	return bc.storeBlock(block, nil)
}

// CommitBlock validates linkage and stores a block together with its receipts in a
// single atomic write batch, so a crash can never persist a block without its
// receipts and transaction index.
func (bc *Blockchain) CommitBlock(block *types.Block, receipts []TxReceipt) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if err := bc.checkLinkage(block); err != nil {
		return err
	}
	return bc.storeBlock(block, receipts)
}

// checkLinkage verifies the block extends the current tip and carries its true hash.
// The hash is VERIFIED, never recomputed-and-overwritten: a block whose declared hash
// doesn't match its header bytes is forged or corrupt and must be rejected.
func (bc *Blockchain) checkLinkage(block *types.Block) error {
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

	if expected := crypto.SHA256Hash(block.HeaderBytes()); block.Hash != expected {
		return fmt.Errorf("block hash mismatch: declared %s, computed %s", block.Hash, expected)
	}
	return nil
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

// ComputeTxRoot computes the transaction root committed in a block header:
// SHA256 over the concatenated tx hashes (ZeroHash for an empty block). Producer
// and validators must use this same function so headers verify everywhere.
func ComputeTxRoot(txs []types.Transaction) types.Hash {
	if len(txs) == 0 {
		return types.ZeroHash
	}
	combined := make([]byte, 0, len(txs)*32)
	for i := range txs {
		combined = append(combined, txs[i].Hash[:]...)
	}
	return crypto.SHA256Hash(combined)
}

// ── Internal helpers ──

func (bc *Blockchain) storeBlock(block *types.Block, receipts []TxReceipt) error {
	data, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("marshal block: %w", err)
	}

	batch := bc.store.NewBatch()
	batch.Set(storage.BlockKey(block.Header.Height), data)
	batch.Set(storage.BlockHashKey(block.Hash), uint64ToBytes(block.Header.Height))
	batch.Set(storage.ChainMetaKey("height"), uint64ToBytes(block.Header.Height))

	// Transactions and receipts go in the SAME batch as the block: a block must
	// never be persisted without its tx index and receipts.
	for i := range block.Transactions {
		txData, err := json.Marshal(&block.Transactions[i])
		if err != nil {
			return fmt.Errorf("marshal tx: %w", err)
		}
		batch.Set(storage.TxKey(block.Transactions[i].Hash), txData)
	}
	for i := range receipts {
		rData, err := json.Marshal(&receipts[i])
		if err != nil {
			return fmt.Errorf("marshal receipt: %w", err)
		}
		batch.Set(storage.ReceiptKey(receipts[i].TxHash), rData)
	}

	if err := batch.Flush(); err != nil {
		return fmt.Errorf("store block: %w", err)
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
