package fsm

import (
	"fmt"
	"testing"
)

// ── Generic Machine Tests ──

func TestApplyValidTransition(t *testing.T) {
	m := New("test", []Transition{
		{From: "A", To: "B", Kind: "go"},
	})

	next, err := m.Apply("A", "go", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != "B" {
		t.Fatalf("expected B, got %s", next)
	}
}

func TestApplyInvalidTransition(t *testing.T) {
	m := New("test", []Transition{
		{From: "A", To: "B", Kind: "go"},
	})

	_, err := m.Apply("B", "go", nil)
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
}

func TestApplyWithGuardPass(t *testing.T) {
	m := New("test", []Transition{
		{From: "A", To: "B", Kind: "go", Guard: func(data interface{}) error {
			return nil
		}},
	})

	next, err := m.Apply("A", "go", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != "B" {
		t.Fatalf("expected B, got %s", next)
	}
}

func TestApplyWithGuardReject(t *testing.T) {
	m := New("test", []Transition{
		{From: "A", To: "B", Kind: "go", Guard: func(data interface{}) error {
			return fmt.Errorf("blocked")
		}},
	})

	_, err := m.Apply("A", "go", nil)
	if err == nil {
		t.Fatal("expected guard rejection error")
	}
}

func TestValidTransitions(t *testing.T) {
	m := New("test", []Transition{
		{From: "A", To: "B", Kind: "go"},
		{From: "A", To: "C", Kind: "jump"},
		{From: "B", To: "C", Kind: "hop"},
	})

	kinds := m.ValidTransitions("A")
	if len(kinds) != 2 {
		t.Fatalf("expected 2 valid transitions from A, got %d", len(kinds))
	}

	kinds = m.ValidTransitions("B")
	if len(kinds) != 1 {
		t.Fatalf("expected 1 valid transition from B, got %d", len(kinds))
	}

	kinds = m.ValidTransitions("C")
	if len(kinds) != 0 {
		t.Fatalf("expected 0 valid transitions from C (terminal), got %d", len(kinds))
	}
}

func TestMachineName(t *testing.T) {
	m := New("myMachine", nil)
	if m.Name() != "myMachine" {
		t.Fatalf("expected myMachine, got %s", m.Name())
	}
}

// ── PayLink FSM Tests ──

func TestPayLinkFSM_CreateTransition(t *testing.T) {
	m := NewPayLinkFSM()
	next, err := m.Apply(PayLinkNone, PayLinkCreate, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != PayLinkCreated {
		t.Fatalf("expected CREATED, got %s", next)
	}
}

func TestPayLinkFSM_SettleTransition(t *testing.T) {
	m := NewPayLinkFSM()
	next, err := m.Apply(PayLinkCreated, PayLinkSettle, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != PayLinkVerified {
		t.Fatalf("expected VERIFIED, got %s", next)
	}
}

func TestPayLinkFSM_CancelTransition(t *testing.T) {
	m := NewPayLinkFSM()
	next, err := m.Apply(PayLinkCreated, PayLinkCancel, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != PayLinkCancelled {
		t.Fatalf("expected CANCELLED, got %s", next)
	}
}

func TestPayLinkFSM_FailFromCreated(t *testing.T) {
	m := NewPayLinkFSM()
	next, err := m.Apply(PayLinkCreated, PayLinkFail, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != PayLinkFailed {
		t.Fatalf("expected FAILED, got %s", next)
	}
}

func TestPayLinkFSM_FailFromNone(t *testing.T) {
	m := NewPayLinkFSM()
	next, err := m.Apply(PayLinkNone, PayLinkFail, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != PayLinkFailed {
		t.Fatalf("expected FAILED, got %s", next)
	}
}

func TestPayLinkFSM_TransferSelfTransition(t *testing.T) {
	m := NewPayLinkFSM()
	next, err := m.Apply(PayLinkCreated, PayLinkTransfer, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != PayLinkCreated {
		t.Fatalf("expected CREATED (self-transition), got %s", next)
	}
}

func TestPayLinkFSM_TransferFromVerifiedFails(t *testing.T) {
	m := NewPayLinkFSM()
	_, err := m.Apply(PayLinkVerified, PayLinkTransfer, nil)
	if err == nil {
		t.Fatal("expected error: can't transfer a verified paylink")
	}
}

func TestPayLinkFSM_TransferFromCancelledFails(t *testing.T) {
	m := NewPayLinkFSM()
	_, err := m.Apply(PayLinkCancelled, PayLinkTransfer, nil)
	if err == nil {
		t.Fatal("expected error: can't transfer a cancelled paylink")
	}
}

func TestPayLinkFSM_TransferFromFailedFails(t *testing.T) {
	m := NewPayLinkFSM()
	_, err := m.Apply(PayLinkFailed, PayLinkTransfer, nil)
	if err == nil {
		t.Fatal("expected error: can't transfer a failed paylink")
	}
}

func TestPayLinkFSM_InvalidTransitionFromVerified(t *testing.T) {
	m := NewPayLinkFSM()
	_, err := m.Apply(PayLinkVerified, PayLinkCancel, nil)
	if err == nil {
		t.Fatal("expected error: can't cancel a verified paylink")
	}
}

func TestPayLinkFSM_InvalidTransitionFromCancelled(t *testing.T) {
	m := NewPayLinkFSM()
	_, err := m.Apply(PayLinkCancelled, PayLinkSettle, nil)
	if err == nil {
		t.Fatal("expected error: can't settle a cancelled paylink")
	}
}

// ── Validator FSM Tests ──

func TestValidatorFSM_StakeFromNonExistent(t *testing.T) {
	m := NewValidatorFSM()
	next, err := m.Apply(ValidatorNonExistent, ValidatorStake, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Staking from non-existent goes to inactive (executor emits activation separately if threshold met)
	if next != ValidatorInactive {
		t.Fatalf("expected INACTIVE, got %s", next)
	}
}

func TestValidatorFSM_InitUnstake(t *testing.T) {
	m := NewValidatorFSM()
	next, err := m.Apply(ValidatorActive, ValidatorInitUnstake, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != ValidatorPendingWithdraw {
		t.Fatalf("expected PENDING_WITHDRAW, got %s", next)
	}
}

func TestValidatorFSM_CompleteUnstake(t *testing.T) {
	m := NewValidatorFSM()
	next, err := m.Apply(ValidatorPendingWithdraw, ValidatorCompleteUnstake, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Goes to Inactive; executor may emit activation if remaining stake sufficient
	if next != ValidatorInactive {
		t.Fatalf("expected INACTIVE, got %s", next)
	}
}

func TestValidatorFSM_Slash(t *testing.T) {
	m := NewValidatorFSM()
	next, err := m.Apply(ValidatorActive, ValidatorSlash, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != ValidatorSlashed {
		t.Fatalf("expected SLASHED, got %s", next)
	}
}

func TestValidatorFSM_Deactivate(t *testing.T) {
	m := NewValidatorFSM()
	next, err := m.Apply(ValidatorActive, ValidatorDeactivate, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != ValidatorInactive {
		t.Fatalf("expected INACTIVE, got %s", next)
	}
}

func TestValidatorFSM_RewardNoStateChange(t *testing.T) {
	m := NewValidatorFSM()
	next, err := m.Apply(ValidatorActive, ValidatorReward, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != ValidatorActive {
		t.Fatalf("expected ACTIVE (no state change), got %s", next)
	}
}

func TestValidatorFSM_InvalidSlashNonExistent(t *testing.T) {
	m := NewValidatorFSM()
	_, err := m.Apply(ValidatorNonExistent, ValidatorSlash, nil)
	if err == nil {
		t.Fatal("expected error: can't slash non-existent validator")
	}
}

// ── Block FSM Tests ──

func TestBlockFSM_FullLifecycle(t *testing.T) {
	m := NewBlockFSM()

	next, err := m.Apply(BlockPending, BlockCompleteExec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != BlockExecuted {
		t.Fatalf("expected EXECUTED, got %s", next)
	}

	next, err = m.Apply(BlockExecuted, BlockCommit, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != BlockCommitted {
		t.Fatalf("expected COMMITTED, got %s", next)
	}
}

func TestBlockFSM_Revert(t *testing.T) {
	m := NewBlockFSM()

	next, err := m.Apply(BlockPending, BlockCompleteExec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	next, err = m.Apply(next, BlockRevert, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != BlockFailed {
		t.Fatalf("expected FAILED, got %s", next)
	}
}

func TestBlockFSM_InvalidCommitFromPending(t *testing.T) {
	m := NewBlockFSM()
	_, err := m.Apply(BlockPending, BlockCommit, nil)
	if err == nil {
		t.Fatal("expected error: can't commit from pending")
	}
}
