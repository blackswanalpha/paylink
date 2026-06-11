package chain

import (
	"fmt"
	"sync"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// BlockProcessor validates and applies blocks received from peers (gossip or sync).
// It is the follower-side counterpart of the BlockProducer: a block only enters the
// chain after its hash, proposer signature, tx root, every tx signature, and the
// post-execution state root have all been verified.
//
// CommitMu serializes state+chain commits between this processor and the local
// block producer; both must hold it around execute-and-commit.
type BlockProcessor struct {
	CommitMu sync.Mutex

	bc      *Blockchain
	exec    *Executor
	state   *state.StateDB
	genesis *types.GenesisConfig
}

// NewBlockProcessor creates a block processor.
func NewBlockProcessor(bc *Blockchain, exec *Executor, s *state.StateDB, genesis *types.GenesisConfig) *BlockProcessor {
	return &BlockProcessor{bc: bc, exec: exec, state: s, genesis: genesis}
}

// ProcessBlock validates block and, if valid, executes its transactions and commits
// it with receipts. Blocks at or below the current tip are ignored (returns nil) so
// gossip echoes of our own blocks are harmless.
func (p *BlockProcessor) ProcessBlock(block *types.Block) error {
	p.CommitMu.Lock()
	defer p.CommitMu.Unlock()

	tip := p.bc.Tip()
	if tip != nil && block.Header.Height <= tip.Header.Height {
		return nil // already have this height
	}

	if err := p.validate(block, tip); err != nil {
		return err
	}

	// Execute on a snapshot so a bad block can never leave residue in state.
	snapID := p.state.Snapshot()
	receipts := p.exec.ExecuteBlock(block.Transactions, block.Header.Timestamp, block.Header.Height)
	for i := range receipts {
		if !receipts[i].Success {
			_ = p.state.Revert(snapID)
			p.exec.DiscardEvents()
			return fmt.Errorf("block %d: tx %s failed: %s",
				block.Header.Height, receipts[i].TxHash, receipts[i].Error)
		}
		receipts[i].BlockHeight = block.Header.Height
		receipts[i].TxIndex = i
	}

	if got := p.state.ComputeStateRoot(); got != block.Header.StateRoot {
		_ = p.state.Revert(snapID)
		p.exec.DiscardEvents()
		return fmt.Errorf("block %d: state root mismatch: computed %s, header %s",
			block.Header.Height, got, block.Header.StateRoot)
	}

	if err := p.bc.CommitBlock(block, receipts); err != nil {
		_ = p.state.Revert(snapID)
		p.exec.DiscardEvents()
		return fmt.Errorf("commit block %d: %w", block.Header.Height, err)
	}

	p.state.DiscardSnapshot(snapID)
	p.exec.FlushEvents(block.Header.Height)
	return nil
}

// validate runs every stateless check on a received block.
func (p *BlockProcessor) validate(block *types.Block, tip *types.Block) error {
	h := block.Header

	if tip != nil {
		if h.PreviousHash != tip.Hash {
			return fmt.Errorf("block %d: previous hash mismatch", h.Height)
		}
		if h.Height != tip.Header.Height+1 {
			return fmt.Errorf("block %d: height gap (tip %d)", h.Height, tip.Header.Height)
		}
	}

	// Declared hash must match the header bytes — never trust it.
	if expected := crypto.SHA256Hash(block.HeaderBytes()); block.Hash != expected {
		return fmt.Errorf("block %d: hash mismatch", h.Height)
	}

	// Phase 1 single proposer: ONLY the genesis admin may propose. Accepting any
	// active validator here would let one race the canonical proposer and fork
	// followers (there is no fork choice). Loosen only when multi-proposer
	// consensus + fork choice land.
	if h.ProposerAddr != p.genesis.AdminAddress {
		return fmt.Errorf("block %d: proposer %s is not the canonical proposer", h.Height, h.ProposerAddr)
	}

	// Commit signature: pubkey present, derives the proposer address, signs the hash.
	c := block.Commit
	if c.ProposerAddr != h.ProposerAddr {
		return fmt.Errorf("block %d: commit proposer %s != header proposer %s",
			h.Height, c.ProposerAddr, h.ProposerAddr)
	}
	if len(c.PublicKey) == 0 {
		return fmt.Errorf("block %d: missing proposer public key", h.Height)
	}
	pub, err := crypto.UnmarshalPublicKey(c.PublicKey)
	if err != nil {
		return fmt.Errorf("block %d: invalid proposer public key: %w", h.Height, err)
	}
	if crypto.PubkeyToAddress(pub) != c.ProposerAddr {
		return fmt.Errorf("block %d: proposer public key does not derive %s", h.Height, c.ProposerAddr)
	}
	if !crypto.Verify(block.Hash, c.Signature, pub) {
		return fmt.Errorf("block %d: invalid proposer signature", h.Height)
	}

	if got := ComputeTxRoot(block.Transactions); got != h.TxRoot {
		return fmt.Errorf("block %d: tx root mismatch", h.Height)
	}

	// Every tx must authenticate (ExecuteBlock re-checks, but failing here is
	// cheaper than executing a forged block).
	for i := range block.Transactions {
		if err := crypto.VerifyTx(&block.Transactions[i]); err != nil {
			return fmt.Errorf("block %d: tx %d: %w", h.Height, i, err)
		}
	}

	return nil
}
