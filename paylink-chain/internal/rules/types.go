package rules

import (
	"encoding/json"

	"github.com/paylink/paylink-chain/internal/types"
)

// RuleType identifies a built-in rule.
type RuleType string

const (
	RuleTimeLock          RuleType = "TimeLock"
	RuleMultiApproval     RuleType = "MultiApproval"
	RuleAmountThreshold   RuleType = "AmountThreshold"
	RuleAddressWhitelist  RuleType = "AddressWhitelist"
	RuleReceiverWhitelist RuleType = "ReceiverWhitelist"
	RuleMaxTransfers      RuleType = "MaxTransfers"
)

// Rule is a single condition attached to a PayLink.
type Rule struct {
	Type   RuleType        `json:"type"`
	Params json.RawMessage `json:"params"`
}

// ActionKind identifies which operation is being guarded.
type ActionKind string

const (
	ActionSettle   ActionKind = "settle"
	ActionCancel   ActionKind = "cancel"
	ActionTransfer ActionKind = "transfer"
)

// EvalContext provides all the data a rule needs for evaluation.
type EvalContext struct {
	Action         ActionKind
	BlockTimestamp int64
	Sender         types.Address
	PayLinkOwner   types.Address
	PayLinkCreator types.Address
	Receiver       types.Address // "to" address for transfers
	Amount         uint64
	TransferCount  uint64
	Approvals      []types.Address // addresses that have approved (for MultiApproval)
}
