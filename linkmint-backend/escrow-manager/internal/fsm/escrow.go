// Package fsm is the escrow state machine. machine.go is copied byte-for-byte from
// paylink-chain/internal/fsm (the chain's PayLink FSM engine — reuse first); this file
// adds the escrow transition table (work20).
//
// States: WAITING → CONDITIONS_MET → RELEASED, WAITING → REFUNDED (timeout), and
// WAITING|CONDITIONS_MET → DISPUTED (terminal here; resolution is work22). `funded` is a
// column flag set by the chain.paylink.verified consumer, NOT a state: the ConditionsMet
// guard requires funded AND condition-satisfied, and the service applies ConditionsMet +
// Release together in one DB transaction (CONDITIONS_MET is never persisted).
package fsm

import "errors"

// Escrow states.
const (
	StateWaiting       State = "WAITING"
	StateConditionsMet State = "CONDITIONS_MET"
	StateReleased      State = "RELEASED"
	StateRefunded      State = "REFUNDED"
	StateDisputed      State = "DISPUTED"
)

// Escrow transition kinds.
const (
	KindConditionsMet TransitionKind = "ConditionsMet"
	KindRelease       TransitionKind = "Release"
	KindTimeout       TransitionKind = "Timeout"
	KindDispute       TransitionKind = "Dispute"
)

// ValidState reports whether s is a known escrow state.
func ValidState(s State) bool {
	switch s {
	case StateWaiting, StateConditionsMet, StateReleased, StateRefunded, StateDisputed:
		return true
	}
	return false
}

// ConditionsMetInput is the guard data for KindConditionsMet.
type ConditionsMetInput struct {
	Funded    bool
	Satisfied bool
}

var (
	errNotFunded    = errors.New("escrow is not funded")
	errNotSatisfied = errors.New("escrow condition is not satisfied")
)

// conditionsMetGuard allows ConditionsMet only when the escrow is funded AND its
// condition is satisfied.
func conditionsMetGuard(data interface{}) error {
	in, ok := data.(ConditionsMetInput)
	if !ok {
		return errors.New("ConditionsMet requires a ConditionsMetInput")
	}
	if !in.Funded {
		return errNotFunded
	}
	if !in.Satisfied {
		return errNotSatisfied
	}
	return nil
}

// NewEscrowMachine builds the escrow FSM.
func NewEscrowMachine() *Machine {
	return New("escrow", []Transition{
		{From: StateWaiting, To: StateConditionsMet, Kind: KindConditionsMet, Guard: conditionsMetGuard},
		{From: StateConditionsMet, To: StateReleased, Kind: KindRelease},
		{From: StateWaiting, To: StateRefunded, Kind: KindTimeout},
		{From: StateWaiting, To: StateDisputed, Kind: KindDispute},
		{From: StateConditionsMet, To: StateDisputed, Kind: KindDispute},
	})
}
