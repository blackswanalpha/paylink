package fsm

// Validator states.
const (
	ValidatorNonExistent     State = "NON_EXISTENT"
	ValidatorActive          State = "ACTIVE"
	ValidatorInactive        State = "INACTIVE"
	ValidatorPendingWithdraw State = "PENDING_WITHDRAW"
	ValidatorSlashed         State = "SLASHED"
)

// Validator transition kinds.
const (
	ValidatorStake           TransitionKind = "Stake"
	ValidatorActivate        TransitionKind = "Activate"
	ValidatorDeactivate      TransitionKind = "Deactivate"
	ValidatorInitUnstake     TransitionKind = "InitiateUnstake"
	ValidatorCompleteUnstake TransitionKind = "CompleteUnstake"
	ValidatorSlash           TransitionKind = "Slash"
	ValidatorReward          TransitionKind = "Reward"
)

// NewValidatorFSM returns the Validator lifecycle state machine.
// Note: Some transitions have multiple possible targets depending on runtime
// state (e.g., stake amount vs minimum). The FSM registers the primary path;
// the executor determines the actual outcome and emits the correct event.
func NewValidatorFSM() *Machine {
	return New("Validator", []Transition{
		// Staking (additional stake keeps current state)
		{From: ValidatorNonExistent, To: ValidatorInactive, Kind: ValidatorStake},
		{From: ValidatorInactive, To: ValidatorInactive, Kind: ValidatorStake},
		{From: ValidatorActive, To: ValidatorActive, Kind: ValidatorStake},
		{From: ValidatorSlashed, To: ValidatorInactive, Kind: ValidatorStake},
		// Activation threshold crossed
		{From: ValidatorInactive, To: ValidatorActive, Kind: ValidatorActivate},
		{From: ValidatorSlashed, To: ValidatorActive, Kind: ValidatorActivate},
		// Deactivation (stake drops below minimum)
		{From: ValidatorActive, To: ValidatorInactive, Kind: ValidatorDeactivate},
		// Unstake initiation
		{From: ValidatorActive, To: ValidatorPendingWithdraw, Kind: ValidatorInitUnstake},
		{From: ValidatorInactive, To: ValidatorPendingWithdraw, Kind: ValidatorInitUnstake},
		// Complete unstake (returns to inactive; executor may emit activation separately)
		{From: ValidatorPendingWithdraw, To: ValidatorInactive, Kind: ValidatorCompleteUnstake},
		// Slashing
		{From: ValidatorActive, To: ValidatorSlashed, Kind: ValidatorSlash},
		{From: ValidatorInactive, To: ValidatorSlashed, Kind: ValidatorSlash},
		// Rewards (no state change, event only)
		{From: ValidatorActive, To: ValidatorActive, Kind: ValidatorReward},
		{From: ValidatorInactive, To: ValidatorInactive, Kind: ValidatorReward},
	})
}
