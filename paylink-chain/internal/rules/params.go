package rules

import "github.com/paylink/paylink-chain/internal/types"

// TimeLockParams prevents actions before/after certain timestamps.
type TimeLockParams struct {
	NotBefore int64        `json:"notBefore,omitempty"` // Unix timestamp
	NotAfter  int64        `json:"notAfter,omitempty"`  // Unix timestamp
	Actions   []ActionKind `json:"actions"`             // which actions this lock applies to
}

// MultiApprovalParams requires N-of-M addresses to approve before settlement.
type MultiApprovalParams struct {
	Required  uint64          `json:"required"`  // minimum approvals needed
	Approvers []types.Address `json:"approvers"` // allowed approver addresses
}

// AmountThresholdParams requires the PayLink amount to be within bounds.
type AmountThresholdParams struct {
	MinAmount uint64 `json:"minAmount,omitempty"`
	MaxAmount uint64 `json:"maxAmount,omitempty"`
}

// AddressWhitelistParams restricts which addresses can trigger actions.
type AddressWhitelistParams struct {
	Addresses []types.Address `json:"addresses"` // allowed addresses
	Actions   []ActionKind    `json:"actions"`   // which actions are restricted
}

// ReceiverWhitelistParams restricts transfer destinations.
type ReceiverWhitelistParams struct {
	Addresses []types.Address `json:"addresses"` // allowed receiver addresses
}

// MaxTransfersParams limits the number of ownership transfers.
type MaxTransfersParams struct {
	Max uint64 `json:"max"` // maximum transfer count
}
