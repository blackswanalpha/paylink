package rules

import (
	"encoding/json"
	"fmt"
)

// ValidateRules checks that all rules in a set are well-formed.
// Called at PayLink creation time. Rejects unknown types or malformed params.
func ValidateRules(ruleSet []Rule) error {
	for i, rule := range ruleSet {
		if err := validateOne(rule); err != nil {
			return fmt.Errorf("rule[%d]: %w", i, err)
		}
	}
	return nil
}

func validateOne(rule Rule) error {
	switch rule.Type {
	case RuleTimeLock:
		var p TimeLockParams
		if err := json.Unmarshal(rule.Params, &p); err != nil {
			return err
		}
		if len(p.Actions) == 0 {
			return fmt.Errorf("TimeLock: actions list must not be empty")
		}
		return nil

	case RuleMultiApproval:
		var p MultiApprovalParams
		if err := json.Unmarshal(rule.Params, &p); err != nil {
			return err
		}
		if p.Required == 0 {
			return fmt.Errorf("MultiApproval: required must be > 0")
		}
		if uint64(len(p.Approvers)) < p.Required {
			return fmt.Errorf("MultiApproval: fewer approvers (%d) than required (%d)", len(p.Approvers), p.Required)
		}
		return nil

	case RuleAmountThreshold:
		var p AmountThresholdParams
		if err := json.Unmarshal(rule.Params, &p); err != nil {
			return err
		}
		if p.MinAmount > 0 && p.MaxAmount > 0 && p.MinAmount > p.MaxAmount {
			return fmt.Errorf("AmountThreshold: minAmount (%d) > maxAmount (%d)", p.MinAmount, p.MaxAmount)
		}
		return nil

	case RuleAddressWhitelist:
		var p AddressWhitelistParams
		if err := json.Unmarshal(rule.Params, &p); err != nil {
			return err
		}
		if len(p.Addresses) == 0 {
			return fmt.Errorf("AddressWhitelist: addresses must not be empty")
		}
		if len(p.Actions) == 0 {
			return fmt.Errorf("AddressWhitelist: actions must not be empty")
		}
		return nil

	case RuleReceiverWhitelist:
		var p ReceiverWhitelistParams
		if err := json.Unmarshal(rule.Params, &p); err != nil {
			return err
		}
		if len(p.Addresses) == 0 {
			return fmt.Errorf("ReceiverWhitelist: addresses must not be empty")
		}
		return nil

	case RuleMaxTransfers:
		var p MaxTransfersParams
		if err := json.Unmarshal(rule.Params, &p); err != nil {
			return err
		}
		if p.Max == 0 {
			return fmt.Errorf("MaxTransfers: max must be > 0")
		}
		return nil

	default:
		return fmt.Errorf("unknown rule type: %s", rule.Type)
	}
}
