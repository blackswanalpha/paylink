package chain

import (
	"fmt"
	"log"

	"github.com/paylink/paylink-chain/internal/state"
)

// Replay re-executes every block from 1 to the persisted tip against s, rebuilding
// the in-memory state after a restart. The chain store only persists blocks —
// without replay a restarted node would sit at height N with genesis state.
//
// The executor must NOT be wired to an event bus: replayed history must not be
// re-published to subscribers.
//
// Replay fails (and the node must refuse to start) if any historical tx fails to
// re-execute or the rebuilt state root doesn't match the tip header — both mean
// the data directory does not represent a chain this binary can verify.
func Replay(bc *Blockchain, exec *Executor, s *state.StateDB) error {
	tip := bc.Tip()
	if tip == nil || tip.Header.Height == 0 {
		return nil // fresh chain: genesis state is already correct
	}

	height := tip.Header.Height
	log.Printf("Replaying %d blocks to rebuild state...", height)

	for h := uint64(1); h <= height; h++ {
		block, err := bc.GetBlockByHeight(h)
		if err != nil {
			return fmt.Errorf("replay: load block %d: %w", h, err)
		}
		if block == nil {
			return fmt.Errorf("replay: block %d missing from store", h)
		}

		receipts := exec.ExecuteBlock(block.Transactions, block.Header.Timestamp, block.Header.Height)
		for i := range receipts {
			if !receipts[i].Success {
				return fmt.Errorf("replay: block %d tx %s failed: %s",
					h, receipts[i].TxHash, receipts[i].Error)
			}
		}
		exec.DiscardEvents()
	}

	if root := s.ComputeStateRoot(); root != tip.Header.StateRoot {
		return fmt.Errorf("replay: state root mismatch at height %d: computed %s, header %s",
			height, root, tip.Header.StateRoot)
	}

	log.Printf("Replay complete: %d blocks, state root %s", height, tip.Header.StateRoot)
	return nil
}
