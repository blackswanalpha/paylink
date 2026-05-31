// Package lifecycle is the payment lifecycle state machine. It is a strict PROJECTION of the
// on-chain PayLink FSM (paylink-chain/internal/fsm/paylink_fsm.go) — it introduces no state the
// chain does not have. The chain is the single source of truth for settlement (invariant A.3);
// this machine only mirrors on-chain truth so the orchestrator never invents settlement.
//
// On-chain PayLink status  →  payment lifecycle state
//
//	CREATED    →  AWAITING_PAYMENT   (authorization minted, no quorum proof yet)
//	VERIFIED   →  SETTLED            (SubmitValidation reached quorum on-chain)
//	CANCELLED  →  CANCELLED
//	FAILED     →  FAILED
//	NONE       →  (no projection — not a payable state)
//
// The transitions below mirror the on-chain machine's edges out of CREATED exactly:
// CREATED→VERIFIED (Settle), CREATED→CANCELLED (Cancel), CREATED→FAILED (Fail). SETTLED,
// CANCELLED and FAILED are terminal here just as VERIFIED/CANCELLED/FAILED are on-chain.
package lifecycle

import (
	"errors"
	"fmt"
)

// State is a payment lifecycle state. Values are intentionally a superset-free mirror of the
// on-chain status strings where they correspond.
type State string

const (
	StateAwaitingPayment State = "AWAITING_PAYMENT" // on-chain CREATED
	StateSettled         State = "SETTLED"          // on-chain VERIFIED
	StateCancelled       State = "CANCELLED"        // on-chain CANCELLED
	StateFailed          State = "FAILED"           // on-chain FAILED
)

// ErrIllegalTransition means the requested move is not an edge in the FSM (e.g. regressing
// out of a terminal state). Callers treat this as "ignore + warn", never as a state change.
var ErrIllegalTransition = errors.New("illegal lifecycle transition")

// transitions is the set of valid edges, mirroring the on-chain PayLink FSM out of CREATED.
var transitions = map[State]map[State]bool{
	StateAwaitingPayment: {
		StateSettled:   true,
		StateCancelled: true,
		StateFailed:    true,
	},
	// terminal states have no outgoing edges
	StateSettled:   {},
	StateCancelled: {},
	StateFailed:    {},
}

// IsTerminal reports whether s admits no further transitions.
func IsTerminal(s State) bool {
	return s == StateSettled || s == StateCancelled || s == StateFailed
}

// Valid reports whether s is a known lifecycle state.
func Valid(s State) bool {
	_, ok := transitions[s]
	return ok
}

// FromChainStatus projects an on-chain PayLink status string onto a lifecycle state.
// ok is false for "NONE"/unknown statuses, which are not payable projections.
func FromChainStatus(status string) (State, bool) {
	switch status {
	case "CREATED":
		return StateAwaitingPayment, true
	case "VERIFIED":
		return StateSettled, true
	case "CANCELLED":
		return StateCancelled, true
	case "FAILED":
		return StateFailed, true
	default: // "NONE" or anything unrecognized
		return "", false
	}
}

// Project computes the next lifecycle state given the current state and the authoritative
// on-chain status. It is idempotent and safe to call repeatedly:
//
//   - changed=false, err=nil when the target equals current (a no-op replay).
//   - changed=true,  err=nil when the move is a valid FSM edge.
//   - changed=false, err=ErrIllegalTransition when target would regress a terminal state or
//     is otherwise not an edge — the caller keeps current and logs.
//
// Because terminal states reject all moves, applying the same settlement twice can never
// double-advance (invariant A.7 at the lifecycle layer).
func Project(current State, chainStatus string) (next State, changed bool, err error) {
	target, ok := FromChainStatus(chainStatus)
	if !ok {
		return current, false, fmt.Errorf("%w: chain status %q has no projection", ErrIllegalTransition, chainStatus)
	}
	if target == current {
		return current, false, nil
	}
	if !transitions[current][target] {
		return current, false, fmt.Errorf("%w: %s -> %s", ErrIllegalTransition, current, target)
	}
	return target, true, nil
}
