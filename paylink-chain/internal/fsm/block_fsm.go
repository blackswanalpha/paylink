package fsm

// Block states.
const (
	BlockPending   State = "PENDING"
	BlockExecuted  State = "EXECUTED"
	BlockCommitted State = "COMMITTED"
	BlockFailed    State = "FAILED"
)

// Block transition kinds.
const (
	BlockCompleteExec TransitionKind = "CompleteExecution"
	BlockCommit       TransitionKind = "Commit"
	BlockRevert       TransitionKind = "Revert"
)

// NewBlockFSM returns the Block production state machine.
func NewBlockFSM() *Machine {
	return New("Block", []Transition{
		{From: BlockPending, To: BlockExecuted, Kind: BlockCompleteExec},
		{From: BlockExecuted, To: BlockCommitted, Kind: BlockCommit},
		{From: BlockExecuted, To: BlockFailed, Kind: BlockRevert},
	})
}
