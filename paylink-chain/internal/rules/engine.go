package rules

import (
	"encoding/json"
	"fmt"
)

// Evaluate checks all rules against the given context.
// Returns nil if all rules pass, or the first rule violation error.
// Deterministic: same rules + same context = same result on all nodes.
func Evaluate(ruleSet []Rule, ctx *EvalContext) error {
	for i, rule := range ruleSet {
		if err := evaluateOne(rule, ctx); err != nil {
			return fmt.Errorf("rule[%d] %s: %w", i, rule.Type, err)
		}
	}
	return nil
}

func evaluateOne(rule Rule, ctx *EvalContext) error {
	switch rule.Type {
	case RuleTimeLock:
		return evalTimeLock(rule.Params, ctx)
	case RuleMultiApproval:
		return evalMultiApproval(rule.Params, ctx)
	case RuleAmountThreshold:
		return evalAmountThreshold(rule.Params, ctx)
	case RuleAddressWhitelist:
		return evalAddressWhitelist(rule.Params, ctx)
	case RuleReceiverWhitelist:
		return evalReceiverWhitelist(rule.Params, ctx)
	case RuleMaxTransfers:
		return evalMaxTransfers(rule.Params, ctx)
	default:
		return fmt.Errorf("unknown rule type: %s", rule.Type)
	}
}

func evalTimeLock(params json.RawMessage, ctx *EvalContext) error {
	var p TimeLockParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}
	if !actionInList(ctx.Action, p.Actions) {
		return nil
	}
	if p.NotBefore > 0 && ctx.BlockTimestamp < p.NotBefore {
		return fmt.Errorf("action not allowed before %d (current: %d)", p.NotBefore, ctx.BlockTimestamp)
	}
	if p.NotAfter > 0 && ctx.BlockTimestamp > p.NotAfter {
		return fmt.Errorf("action not allowed after %d (current: %d)", p.NotAfter, ctx.BlockTimestamp)
	}
	return nil
}

func evalMultiApproval(params json.RawMessage, ctx *EvalContext) error {
	var p MultiApprovalParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}
	if ctx.Action != ActionSettle {
		return nil // multi-approval only gates settlement
	}
	count := uint64(0)
	for _, approval := range ctx.Approvals {
		for _, approver := range p.Approvers {
			if approval == approver {
				count++
				break
			}
		}
	}
	if count < p.Required {
		return fmt.Errorf("need %d approvals, have %d", p.Required, count)
	}
	return nil
}

func evalAmountThreshold(params json.RawMessage, ctx *EvalContext) error {
	var p AmountThresholdParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}
	if p.MinAmount > 0 && ctx.Amount < p.MinAmount {
		return fmt.Errorf("amount below minimum: %d < %d", ctx.Amount, p.MinAmount)
	}
	if p.MaxAmount > 0 && ctx.Amount > p.MaxAmount {
		return fmt.Errorf("amount above maximum: %d > %d", ctx.Amount, p.MaxAmount)
	}
	return nil
}

func evalAddressWhitelist(params json.RawMessage, ctx *EvalContext) error {
	var p AddressWhitelistParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}
	if !actionInList(ctx.Action, p.Actions) {
		return nil
	}
	for _, addr := range p.Addresses {
		if addr == ctx.Sender {
			return nil
		}
	}
	return fmt.Errorf("address %s not in whitelist", ctx.Sender.Hex())
}

func evalReceiverWhitelist(params json.RawMessage, ctx *EvalContext) error {
	var p ReceiverWhitelistParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}
	if ctx.Action != ActionTransfer {
		return nil
	}
	for _, addr := range p.Addresses {
		if addr == ctx.Receiver {
			return nil
		}
	}
	return fmt.Errorf("receiver %s not in whitelist", ctx.Receiver.Hex())
}

func evalMaxTransfers(params json.RawMessage, ctx *EvalContext) error {
	var p MaxTransfersParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}
	if ctx.Action != ActionTransfer {
		return nil
	}
	if ctx.TransferCount >= p.Max {
		return fmt.Errorf("max transfers reached: %d >= %d", ctx.TransferCount, p.Max)
	}
	return nil
}

func actionInList(action ActionKind, list []ActionKind) bool {
	for _, a := range list {
		if a == action {
			return true
		}
	}
	return false
}
