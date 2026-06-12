package fsm

// PayLink states mirror types.Status values.
const (
	PayLinkNone      State = "NONE"
	PayLinkCreated   State = "CREATED"
	PayLinkVerified  State = "VERIFIED"
	PayLinkFailed    State = "FAILED"
	PayLinkCancelled State = "CANCELLED"
)

// PayLink transition kinds.
const (
	PayLinkCreate   TransitionKind = "Create"
	PayLinkVote     TransitionKind = "Vote"   // SubmitValidation (no status change, tracked as event only)
	PayLinkSettle   TransitionKind = "Settle" // SubmitValidation reaching quorum
	PayLinkCancel   TransitionKind = "Cancel"
	PayLinkFail     TransitionKind = "Fail"
	PayLinkTransfer TransitionKind = "Transfer"
)

// NewPayLinkFSM returns the PayLink lifecycle state machine.
func NewPayLinkFSM() *Machine {
	return New("PayLink", []Transition{
		{From: PayLinkNone, To: PayLinkCreated, Kind: PayLinkCreate},
		{From: PayLinkCreated, To: PayLinkVerified, Kind: PayLinkSettle},
		{From: PayLinkCreated, To: PayLinkCancelled, Kind: PayLinkCancel},
		{From: PayLinkCreated, To: PayLinkFailed, Kind: PayLinkFail},
		// Admin can fail from any non-terminal state
		{From: PayLinkNone, To: PayLinkFailed, Kind: PayLinkFail},
		// Transfer is only valid from CREATED (self-transition, status unchanged)
		{From: PayLinkCreated, To: PayLinkCreated, Kind: PayLinkTransfer},
	})
}
