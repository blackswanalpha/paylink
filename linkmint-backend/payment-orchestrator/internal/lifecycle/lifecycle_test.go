package lifecycle

import (
	"errors"
	"testing"
)

func TestFromChainStatus(t *testing.T) {
	cases := []struct {
		status string
		want   State
		ok     bool
	}{
		{"CREATED", StateAwaitingPayment, true},
		{"VERIFIED", StateSettled, true},
		{"CANCELLED", StateCancelled, true},
		{"FAILED", StateFailed, true},
		{"NONE", "", false},
		{"BOGUS", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := FromChainStatus(c.status)
		if got != c.want || ok != c.ok {
			t.Errorf("FromChainStatus(%q) = (%q,%v), want (%q,%v)", c.status, got, ok, c.want, c.ok)
		}
	}
}

func TestIsTerminalAndValid(t *testing.T) {
	terminal := []State{StateSettled, StateCancelled, StateFailed}
	for _, s := range terminal {
		if !IsTerminal(s) {
			t.Errorf("IsTerminal(%q) = false, want true", s)
		}
	}
	if IsTerminal(StateAwaitingPayment) {
		t.Error("AWAITING_PAYMENT should not be terminal")
	}
	for _, s := range []State{StateAwaitingPayment, StateSettled, StateCancelled, StateFailed} {
		if !Valid(s) {
			t.Errorf("Valid(%q) = false, want true", s)
		}
	}
	if Valid("BOGUS") {
		t.Error("Valid(BOGUS) = true, want false")
	}
}

func TestProject(t *testing.T) {
	cases := []struct {
		name        string
		current     State
		chainStatus string
		wantNext    State
		wantChanged bool
		wantErr     bool
	}{
		{"awaiting->settled", StateAwaitingPayment, "VERIFIED", StateSettled, true, false},
		{"awaiting->cancelled", StateAwaitingPayment, "CANCELLED", StateCancelled, true, false},
		{"awaiting->failed", StateAwaitingPayment, "FAILED", StateFailed, true, false},
		{"awaiting->awaiting noop", StateAwaitingPayment, "CREATED", StateAwaitingPayment, false, false},
		{"settled replay noop", StateSettled, "VERIFIED", StateSettled, false, false},
		{"cancelled replay noop", StateCancelled, "CANCELLED", StateCancelled, false, false},
		{"settled cannot regress to cancelled", StateSettled, "CANCELLED", StateSettled, false, true},
		{"settled cannot regress to created", StateSettled, "CREATED", StateSettled, false, true},
		{"unknown status is illegal", StateAwaitingPayment, "NONE", StateAwaitingPayment, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next, changed, err := Project(c.current, c.chainStatus)
			if next != c.wantNext || changed != c.wantChanged {
				t.Errorf("Project(%q,%q) = (%q,%v), want (%q,%v)", c.current, c.chainStatus, next, changed, c.wantNext, c.wantChanged)
			}
			if c.wantErr {
				if !errors.Is(err, ErrIllegalTransition) {
					t.Errorf("expected ErrIllegalTransition, got %v", err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
