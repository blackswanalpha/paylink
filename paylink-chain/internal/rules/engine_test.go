package rules

import (
	"encoding/json"
	"testing"

	"github.com/paylink/paylink-chain/internal/types"
)

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestEvaluate_EmptyRules(t *testing.T) {
	err := Evaluate(nil, &EvalContext{Action: ActionSettle})
	if err != nil {
		t.Fatalf("empty rules should pass: %v", err)
	}
	err = Evaluate([]Rule{}, &EvalContext{Action: ActionSettle})
	if err != nil {
		t.Fatalf("zero-length rules should pass: %v", err)
	}
}

func TestEvaluate_UnknownType(t *testing.T) {
	rules := []Rule{{Type: "FooBar", Params: json.RawMessage(`{}`)}}
	err := Evaluate(rules, &EvalContext{Action: ActionSettle})
	if err == nil {
		t.Fatal("unknown rule type should fail")
	}
}

// ── TimeLock ──

func TestTimeLock_BeforeWindow(t *testing.T) {
	rules := []Rule{{
		Type: RuleTimeLock,
		Params: mustJSON(TimeLockParams{
			NotBefore: 1000,
			Actions:   []ActionKind{ActionSettle},
		}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionSettle, BlockTimestamp: 500})
	if err == nil {
		t.Fatal("should reject before NotBefore")
	}
}

func TestTimeLock_AfterWindow(t *testing.T) {
	rules := []Rule{{
		Type: RuleTimeLock,
		Params: mustJSON(TimeLockParams{
			NotAfter: 1000,
			Actions:  []ActionKind{ActionSettle},
		}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionSettle, BlockTimestamp: 2000})
	if err == nil {
		t.Fatal("should reject after NotAfter")
	}
}

func TestTimeLock_WithinWindow(t *testing.T) {
	rules := []Rule{{
		Type: RuleTimeLock,
		Params: mustJSON(TimeLockParams{
			NotBefore: 1000,
			NotAfter:  2000,
			Actions:   []ActionKind{ActionSettle},
		}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionSettle, BlockTimestamp: 1500})
	if err != nil {
		t.Fatalf("should pass within window: %v", err)
	}
}

func TestTimeLock_DifferentAction(t *testing.T) {
	rules := []Rule{{
		Type: RuleTimeLock,
		Params: mustJSON(TimeLockParams{
			NotBefore: 1000,
			Actions:   []ActionKind{ActionSettle},
		}),
	}}
	// Transfer should not be blocked by settle-only time lock
	err := Evaluate(rules, &EvalContext{Action: ActionTransfer, BlockTimestamp: 500})
	if err != nil {
		t.Fatalf("different action should not be blocked: %v", err)
	}
}

// ── MultiApproval ──

func TestMultiApproval_Sufficient(t *testing.T) {
	addr1 := types.HexToAddress("0x0000000000000000000000000000000000000001")
	addr2 := types.HexToAddress("0x0000000000000000000000000000000000000002")
	rules := []Rule{{
		Type: RuleMultiApproval,
		Params: mustJSON(MultiApprovalParams{
			Required:  2,
			Approvers: []types.Address{addr1, addr2},
		}),
	}}
	err := Evaluate(rules, &EvalContext{
		Action:    ActionSettle,
		Approvals: []types.Address{addr1, addr2},
	})
	if err != nil {
		t.Fatalf("should pass with sufficient approvals: %v", err)
	}
}

func TestMultiApproval_Insufficient(t *testing.T) {
	addr1 := types.HexToAddress("0x0000000000000000000000000000000000000001")
	addr2 := types.HexToAddress("0x0000000000000000000000000000000000000002")
	rules := []Rule{{
		Type: RuleMultiApproval,
		Params: mustJSON(MultiApprovalParams{
			Required:  2,
			Approvers: []types.Address{addr1, addr2},
		}),
	}}
	err := Evaluate(rules, &EvalContext{
		Action:    ActionSettle,
		Approvals: []types.Address{addr1},
	})
	if err == nil {
		t.Fatal("should reject with insufficient approvals")
	}
}

func TestMultiApproval_SkipsNonSettle(t *testing.T) {
	addr1 := types.HexToAddress("0x0000000000000000000000000000000000000001")
	rules := []Rule{{
		Type: RuleMultiApproval,
		Params: mustJSON(MultiApprovalParams{
			Required:  2,
			Approvers: []types.Address{addr1},
		}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionTransfer})
	if err != nil {
		t.Fatalf("should skip for non-settle: %v", err)
	}
}

// ── AmountThreshold ──

func TestAmountThreshold_BelowMin(t *testing.T) {
	rules := []Rule{{
		Type:   RuleAmountThreshold,
		Params: mustJSON(AmountThresholdParams{MinAmount: 100}),
	}}
	err := Evaluate(rules, &EvalContext{Amount: 50})
	if err == nil {
		t.Fatal("should reject below min")
	}
}

func TestAmountThreshold_AboveMax(t *testing.T) {
	rules := []Rule{{
		Type:   RuleAmountThreshold,
		Params: mustJSON(AmountThresholdParams{MaxAmount: 100}),
	}}
	err := Evaluate(rules, &EvalContext{Amount: 200})
	if err == nil {
		t.Fatal("should reject above max")
	}
}

func TestAmountThreshold_WithinRange(t *testing.T) {
	rules := []Rule{{
		Type:   RuleAmountThreshold,
		Params: mustJSON(AmountThresholdParams{MinAmount: 50, MaxAmount: 200}),
	}}
	err := Evaluate(rules, &EvalContext{Amount: 100})
	if err != nil {
		t.Fatalf("should pass within range: %v", err)
	}
}

// ── AddressWhitelist ──

func TestAddressWhitelist_Allowed(t *testing.T) {
	addr := types.HexToAddress("0x0000000000000000000000000000000000000001")
	rules := []Rule{{
		Type: RuleAddressWhitelist,
		Params: mustJSON(AddressWhitelistParams{
			Addresses: []types.Address{addr},
			Actions:   []ActionKind{ActionCancel},
		}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionCancel, Sender: addr})
	if err != nil {
		t.Fatalf("whitelisted address should pass: %v", err)
	}
}

func TestAddressWhitelist_Blocked(t *testing.T) {
	addr := types.HexToAddress("0x0000000000000000000000000000000000000001")
	other := types.HexToAddress("0x0000000000000000000000000000000000000002")
	rules := []Rule{{
		Type: RuleAddressWhitelist,
		Params: mustJSON(AddressWhitelistParams{
			Addresses: []types.Address{addr},
			Actions:   []ActionKind{ActionCancel},
		}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionCancel, Sender: other})
	if err == nil {
		t.Fatal("non-whitelisted address should be rejected")
	}
}

func TestAddressWhitelist_DifferentAction(t *testing.T) {
	addr := types.HexToAddress("0x0000000000000000000000000000000000000001")
	other := types.HexToAddress("0x0000000000000000000000000000000000000002")
	rules := []Rule{{
		Type: RuleAddressWhitelist,
		Params: mustJSON(AddressWhitelistParams{
			Addresses: []types.Address{addr},
			Actions:   []ActionKind{ActionCancel},
		}),
	}}
	// Settle action should not be blocked by cancel-only whitelist
	err := Evaluate(rules, &EvalContext{Action: ActionSettle, Sender: other})
	if err != nil {
		t.Fatalf("different action should not be blocked: %v", err)
	}
}

// ── ReceiverWhitelist ──

func TestReceiverWhitelist_Allowed(t *testing.T) {
	addr := types.HexToAddress("0x0000000000000000000000000000000000000001")
	rules := []Rule{{
		Type: RuleReceiverWhitelist,
		Params: mustJSON(ReceiverWhitelistParams{
			Addresses: []types.Address{addr},
		}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionTransfer, Receiver: addr})
	if err != nil {
		t.Fatalf("whitelisted receiver should pass: %v", err)
	}
}

func TestReceiverWhitelist_Blocked(t *testing.T) {
	addr := types.HexToAddress("0x0000000000000000000000000000000000000001")
	other := types.HexToAddress("0x0000000000000000000000000000000000000002")
	rules := []Rule{{
		Type: RuleReceiverWhitelist,
		Params: mustJSON(ReceiverWhitelistParams{
			Addresses: []types.Address{addr},
		}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionTransfer, Receiver: other})
	if err == nil {
		t.Fatal("non-whitelisted receiver should be rejected")
	}
}

func TestReceiverWhitelist_SkipsNonTransfer(t *testing.T) {
	rules := []Rule{{
		Type: RuleReceiverWhitelist,
		Params: mustJSON(ReceiverWhitelistParams{
			Addresses: []types.Address{},
		}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionSettle})
	if err != nil {
		t.Fatalf("should skip for non-transfer: %v", err)
	}
}

// ── MaxTransfers ──

func TestMaxTransfers_BelowLimit(t *testing.T) {
	rules := []Rule{{
		Type:   RuleMaxTransfers,
		Params: mustJSON(MaxTransfersParams{Max: 3}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionTransfer, TransferCount: 2})
	if err != nil {
		t.Fatalf("should pass below limit: %v", err)
	}
}

func TestMaxTransfers_AtLimit(t *testing.T) {
	rules := []Rule{{
		Type:   RuleMaxTransfers,
		Params: mustJSON(MaxTransfersParams{Max: 3}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionTransfer, TransferCount: 3})
	if err == nil {
		t.Fatal("should reject at limit")
	}
}

func TestMaxTransfers_SkipsNonTransfer(t *testing.T) {
	rules := []Rule{{
		Type:   RuleMaxTransfers,
		Params: mustJSON(MaxTransfersParams{Max: 0}),
	}}
	err := Evaluate(rules, &EvalContext{Action: ActionSettle, TransferCount: 100})
	if err != nil {
		t.Fatalf("should skip for non-transfer: %v", err)
	}
}

// ── Multiple rules ──

func TestMultipleRules_AllPass(t *testing.T) {
	rules := []Rule{
		{Type: RuleAmountThreshold, Params: mustJSON(AmountThresholdParams{MinAmount: 50})},
		{Type: RuleMaxTransfers, Params: mustJSON(MaxTransfersParams{Max: 5})},
	}
	err := Evaluate(rules, &EvalContext{Action: ActionTransfer, Amount: 100, TransferCount: 2})
	if err != nil {
		t.Fatalf("all rules should pass: %v", err)
	}
}

func TestMultipleRules_FirstFails(t *testing.T) {
	rules := []Rule{
		{Type: RuleAmountThreshold, Params: mustJSON(AmountThresholdParams{MinAmount: 200})},
		{Type: RuleMaxTransfers, Params: mustJSON(MaxTransfersParams{Max: 5})},
	}
	err := Evaluate(rules, &EvalContext{Action: ActionTransfer, Amount: 100, TransferCount: 2})
	if err == nil {
		t.Fatal("should fail when first rule fails")
	}
}
