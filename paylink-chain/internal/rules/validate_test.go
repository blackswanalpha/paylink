package rules

import (
	"encoding/json"
	"testing"

	"github.com/paylink/paylink-chain/internal/types"
)

func TestValidateRules_Empty(t *testing.T) {
	if err := ValidateRules(nil); err != nil {
		t.Fatalf("empty rules should validate: %v", err)
	}
}

func TestValidateRules_UnknownType(t *testing.T) {
	rules := []Rule{{Type: "FooBar", Params: json.RawMessage(`{}`)}}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("unknown type should fail validation")
	}
}

func TestValidateRules_TimeLock_Valid(t *testing.T) {
	rules := []Rule{{
		Type: RuleTimeLock,
		Params: mustJSON(TimeLockParams{
			NotBefore: 1000,
			Actions:   []ActionKind{ActionSettle},
		}),
	}}
	if err := ValidateRules(rules); err != nil {
		t.Fatalf("valid TimeLock should pass: %v", err)
	}
}

func TestValidateRules_TimeLock_NoActions(t *testing.T) {
	rules := []Rule{{
		Type: RuleTimeLock,
		Params: mustJSON(TimeLockParams{
			NotBefore: 1000,
			Actions:   []ActionKind{},
		}),
	}}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("TimeLock with empty actions should fail")
	}
}

func TestValidateRules_MultiApproval_Valid(t *testing.T) {
	addr1 := types.HexToAddress("0x0000000000000000000000000000000000000001")
	addr2 := types.HexToAddress("0x0000000000000000000000000000000000000002")
	rules := []Rule{{
		Type: RuleMultiApproval,
		Params: mustJSON(MultiApprovalParams{
			Required:  2,
			Approvers: []types.Address{addr1, addr2},
		}),
	}}
	if err := ValidateRules(rules); err != nil {
		t.Fatalf("valid MultiApproval should pass: %v", err)
	}
}

func TestValidateRules_MultiApproval_ZeroRequired(t *testing.T) {
	rules := []Rule{{
		Type: RuleMultiApproval,
		Params: mustJSON(MultiApprovalParams{
			Required:  0,
			Approvers: []types.Address{},
		}),
	}}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("MultiApproval with 0 required should fail")
	}
}

func TestValidateRules_MultiApproval_FewerApprovers(t *testing.T) {
	addr1 := types.HexToAddress("0x0000000000000000000000000000000000000001")
	rules := []Rule{{
		Type: RuleMultiApproval,
		Params: mustJSON(MultiApprovalParams{
			Required:  3,
			Approvers: []types.Address{addr1},
		}),
	}}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("MultiApproval with fewer approvers than required should fail")
	}
}

func TestValidateRules_AmountThreshold_Valid(t *testing.T) {
	rules := []Rule{{
		Type:   RuleAmountThreshold,
		Params: mustJSON(AmountThresholdParams{MinAmount: 50, MaxAmount: 200}),
	}}
	if err := ValidateRules(rules); err != nil {
		t.Fatalf("valid AmountThreshold should pass: %v", err)
	}
}

func TestValidateRules_AmountThreshold_MinGtMax(t *testing.T) {
	rules := []Rule{{
		Type:   RuleAmountThreshold,
		Params: mustJSON(AmountThresholdParams{MinAmount: 200, MaxAmount: 50}),
	}}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("AmountThreshold with min > max should fail")
	}
}

func TestValidateRules_AddressWhitelist_Valid(t *testing.T) {
	addr := types.HexToAddress("0x0000000000000000000000000000000000000001")
	rules := []Rule{{
		Type: RuleAddressWhitelist,
		Params: mustJSON(AddressWhitelistParams{
			Addresses: []types.Address{addr},
			Actions:   []ActionKind{ActionCancel},
		}),
	}}
	if err := ValidateRules(rules); err != nil {
		t.Fatalf("valid AddressWhitelist should pass: %v", err)
	}
}

func TestValidateRules_AddressWhitelist_Empty(t *testing.T) {
	rules := []Rule{{
		Type: RuleAddressWhitelist,
		Params: mustJSON(AddressWhitelistParams{
			Addresses: []types.Address{},
			Actions:   []ActionKind{ActionCancel},
		}),
	}}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("AddressWhitelist with empty addresses should fail")
	}
}

func TestValidateRules_ReceiverWhitelist_Empty(t *testing.T) {
	rules := []Rule{{
		Type: RuleReceiverWhitelist,
		Params: mustJSON(ReceiverWhitelistParams{
			Addresses: []types.Address{},
		}),
	}}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("ReceiverWhitelist with empty addresses should fail")
	}
}

func TestValidateRules_MaxTransfers_Zero(t *testing.T) {
	rules := []Rule{{
		Type:   RuleMaxTransfers,
		Params: mustJSON(MaxTransfersParams{Max: 0}),
	}}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("MaxTransfers with 0 max should fail")
	}
}

func TestValidateRules_MaxTransfers_Valid(t *testing.T) {
	rules := []Rule{{
		Type:   RuleMaxTransfers,
		Params: mustJSON(MaxTransfersParams{Max: 5}),
	}}
	if err := ValidateRules(rules); err != nil {
		t.Fatalf("valid MaxTransfers should pass: %v", err)
	}
}
