package consensus

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/paylink/paylink-chain/internal/chain"
	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/events"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/txpool"
	"github.com/paylink/paylink-chain/internal/types"
)

const maxTxsPerBlock = 500

// BlockProducer continuously produces blocks at a configured interval.
type BlockProducer struct {
	blockchain *chain.Blockchain
	executor   *chain.Executor
	state      *state.StateDB
	mempool    *txpool.Mempool
	pov        *PoV
	interval   time.Duration
	proposer   types.Address
	signingKey []byte // private key bytes for signing blocks
	eventBus   *events.Bus
	p2pHost    p2pHost     // optional P2P host for block broadcast
	commitMu   *sync.Mutex // serializes commits with the follower BlockProcessor
}

// p2pHost is the subset of p2p.Host we need (avoids import cycle).
type p2pHost interface {
	BroadcastBlock(block *types.Block) error
}

// NewBlockProducer creates a new block producer.
func NewBlockProducer(
	bc *chain.Blockchain,
	exec *chain.Executor,
	s *state.StateDB,
	mp *txpool.Mempool,
	pov *PoV,
	interval time.Duration,
	proposer types.Address,
	signingKey []byte,
	bus ...*events.Bus,
) *BlockProducer {
	bp := &BlockProducer{
		blockchain: bc,
		executor:   exec,
		state:      s,
		mempool:    mp,
		pov:        pov,
		interval:   interval,
		proposer:   proposer,
		signingKey: signingKey,
	}
	if len(bus) > 0 {
		bp.eventBus = bus[0]
	}
	return bp
}

// SetP2PHost sets the P2P host for block broadcasting.
func (bp *BlockProducer) SetP2PHost(host p2pHost) {
	bp.p2pHost = host
}

// SetCommitLock shares a mutex with the follower-side BlockProcessor so that local
// production and peer-block application never execute/commit concurrently.
func (bp *BlockProducer) SetCommitLock(mu *sync.Mutex) {
	bp.commitMu = mu
}

// Start begins producing blocks at the configured interval.
func (bp *BlockProducer) Start(ctx context.Context) {
	ticker := time.NewTicker(bp.interval)
	defer ticker.Stop()

	log.Printf("Block producer started (interval: %s, proposer: %s)", bp.interval, bp.proposer)

	for {
		select {
		case <-ctx.Done():
			log.Println("Block producer stopped")
			return
		case <-ticker.C:
			bp.produceBlock()
		}
	}
}

func (bp *BlockProducer) produceBlock() {
	// Phase 1 single proposer: only the canonical proposer produces blocks.
	// Every other node runs as a follower and applies blocks via the BlockProcessor.
	if bp.pov != nil && bp.proposer != bp.pov.Proposer() {
		return
	}

	if bp.commitMu != nil {
		bp.commitMu.Lock()
		defer bp.commitMu.Unlock()
	}

	// Drain transactions from mempool
	txs := bp.mempool.DrainForBlock(maxTxsPerBlock)

	// Skip empty blocks (unless we want heartbeat blocks)
	if len(txs) == 0 {
		return
	}

	timestamp := time.Now().Unix()

	// Compute height before block creation for receipt tagging
	tip := bp.blockchain.Tip()
	blockHeight := uint64(0)
	if tip != nil {
		blockHeight = tip.Header.Height + 1
	}

	// Execute transactions on a snapshot: if the block fails to commit (or sign),
	// state must roll back to exactly the pre-block state.
	snapID := bp.state.Snapshot()
	receipts := bp.executor.ExecuteBlock(txs, timestamp, blockHeight)

	// Collect successful transactions and tag receipts
	var successTxs []types.Transaction
	for i, r := range receipts {
		if r.Success {
			successTxs = append(successTxs, txs[i])
		} else {
			log.Printf("Tx %s failed: %s", txs[i].Hash, r.Error)
		}
	}

	// Tag receipts with block height and tx index
	txIdx := 0
	for i := range receipts {
		receipts[i].BlockHeight = blockHeight
		if receipts[i].Success {
			receipts[i].TxIndex = txIdx
			txIdx++
		}
	}

	// All txs failed: nothing to commit (state already rolled back per tx).
	if len(successTxs) == 0 {
		bp.state.DiscardSnapshot(snapID)
		bp.executor.DiscardEvents()
		if err := bp.blockchain.StoreReceipts(receipts); err != nil {
			log.Printf("Failed to store receipts: %v", err)
		}
		return
	}

	revert := func(reason string, err error) {
		log.Printf("Block %d aborted (%s): %v", blockHeight, reason, err)
		if rerr := bp.state.Revert(snapID); rerr != nil {
			log.Printf("CRITICAL: state revert failed for block %d: %v", blockHeight, rerr)
		}
		bp.executor.DiscardEvents()
		bp.mempool.ReinsertAll(successTxs)
	}

	// Compute state root and tx root
	stateRoot := bp.state.ComputeStateRoot()
	txRoot := chain.ComputeTxRoot(successTxs)

	// Get previous hash
	prevHash := types.ZeroHash
	if tip != nil {
		prevHash = tip.Hash
	}

	// Create block
	block := &types.Block{
		Header: types.BlockHeader{
			Height:       blockHeight,
			Timestamp:    timestamp,
			PreviousHash: prevHash,
			StateRoot:    stateRoot,
			TxRoot:       txRoot,
			ProposerAddr: bp.proposer,
		},
		Transactions: successTxs,
	}

	// Compute block hash
	block.Hash = pcrypto.SHA256Hash(block.HeaderBytes())

	// Sign block — mandatory: validators reject unsigned blocks, so producing one
	// without a valid key would fork us off our own network.
	if bp.signingKey == nil {
		revert("no signing key", fmt.Errorf("block producer has no signing key"))
		return
	}
	key, err := pcrypto.UnmarshalPrivateKey(bp.signingKey)
	if err != nil {
		revert("invalid signing key", err)
		return
	}
	sig, err := pcrypto.Sign(block.Hash, key)
	if err != nil {
		revert("signing failed", err)
		return
	}
	block.Commit = types.BlockCommit{
		ProposerAddr: bp.proposer,
		PublicKey:    pcrypto.MarshalPublicKey(&key.PublicKey),
		Signature:    sig,
	}

	// Commit block + receipts in one atomic batch
	if err := bp.blockchain.CommitBlock(block, receipts); err != nil {
		revert("commit failed", err)
		return
	}
	bp.state.DiscardSnapshot(snapID)
	bp.executor.FlushEvents(blockHeight)

	log.Printf("Block %d produced: %d txs, hash: %s", blockHeight, len(successTxs), block.Hash)

	// Broadcast block via P2P
	if bp.p2pHost != nil {
		if err := bp.p2pHost.BroadcastBlock(block); err != nil {
			log.Printf("P2P broadcast failed for block %d: %v", blockHeight, err)
		}
	}

	// Emit block produced event
	if bp.eventBus != nil {
		evt := events.NewEvent(events.EventBlockProduced, events.EntityBlock, block.Hash.Hex(), blockHeight).
			WithData(events.BlockProducedData{
				Hash:         block.Hash.Hex(),
				Height:       blockHeight,
				TxCount:      len(successTxs),
				Proposer:     bp.proposer.Hex(),
				StateRoot:    stateRoot.Hex(),
				PreviousHash: prevHash.Hex(),
			})
		bp.eventBus.Publish(evt)
	}
}
