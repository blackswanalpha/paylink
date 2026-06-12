package fsm

import "testing"

func TestEscrowMachineTransitions(t *testing.T) {
	m := NewEscrowMachine()
	cases := []struct {
		name    string
		from    State
		kind    TransitionKind
		data    interface{}
		want    State
		wantErr bool
	}{
		{"conditions met when funded+satisfied", StateWaiting, KindConditionsMet, ConditionsMetInput{Funded: true, Satisfied: true}, StateConditionsMet, false},
		{"conditions met rejected when unfunded", StateWaiting, KindConditionsMet, ConditionsMetInput{Funded: false, Satisfied: true}, StateWaiting, true},
		{"conditions met rejected when unsatisfied", StateWaiting, KindConditionsMet, ConditionsMetInput{Funded: true, Satisfied: false}, StateWaiting, true},
		{"conditions met rejected on bad guard data", StateWaiting, KindConditionsMet, "junk", StateWaiting, true},
		{"release from conditions met", StateConditionsMet, KindRelease, nil, StateReleased, false},
		{"timeout refunds waiting", StateWaiting, KindTimeout, nil, StateRefunded, false},
		{"dispute from waiting", StateWaiting, KindDispute, nil, StateDisputed, false},
		{"dispute from conditions met", StateConditionsMet, KindDispute, nil, StateDisputed, false},
		// Terminal states accept nothing.
		{"no release from waiting", StateWaiting, KindRelease, nil, StateWaiting, true},
		{"no timeout from conditions met", StateConditionsMet, KindTimeout, nil, StateConditionsMet, true},
		{"released is terminal", StateReleased, KindDispute, nil, StateReleased, true},
		{"refunded is terminal", StateRefunded, KindConditionsMet, ConditionsMetInput{Funded: true, Satisfied: true}, StateRefunded, true},
		{"disputed is terminal", StateDisputed, KindTimeout, nil, StateDisputed, true},
		{"disputed blocks release", StateDisputed, KindRelease, nil, StateDisputed, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := m.Apply(tc.from, tc.kind, tc.data)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidTransitions(t *testing.T) {
	m := NewEscrowMachine()
	if kinds := m.ValidTransitions(StateWaiting); len(kinds) != 3 {
		t.Fatalf("WAITING should have 3 transitions, got %v", kinds)
	}
	if kinds := m.ValidTransitions(StateReleased); len(kinds) != 0 {
		t.Fatalf("RELEASED should be terminal, got %v", kinds)
	}
	if m.Name() != "escrow" {
		t.Fatalf("name = %q", m.Name())
	}
}

func TestValidState(t *testing.T) {
	for _, s := range []State{StateWaiting, StateConditionsMet, StateReleased, StateRefunded, StateDisputed} {
		if !ValidState(s) {
			t.Errorf("ValidState(%q) = false", s)
		}
	}
	if ValidState("BOGUS") {
		t.Error("ValidState(BOGUS) = true")
	}
}
