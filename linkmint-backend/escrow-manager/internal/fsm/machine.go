package fsm

import "fmt"

// State is a named state in a finite state machine.
type State string

// TransitionKind is a named trigger for a state transition.
type TransitionKind string

// Guard is a function that returns nil if the transition is allowed.
type Guard func(data interface{}) error

// Transition defines a valid state change.
type Transition struct {
	From  State
	To    State
	Kind  TransitionKind
	Guard Guard // nil means always allowed
}

// Machine defines a finite state machine with registered transitions.
type Machine struct {
	name        string
	transitions map[transitionKey]*Transition
}

type transitionKey struct {
	from State
	kind TransitionKind
}

// New creates a new state machine with the given name and transitions.
func New(name string, transitions []Transition) *Machine {
	m := &Machine{
		name:        name,
		transitions: make(map[transitionKey]*Transition),
	}
	for i := range transitions {
		t := &transitions[i]
		key := transitionKey{from: t.From, kind: t.Kind}
		m.transitions[key] = t
	}
	return m
}

// Apply attempts to apply a transition. Returns the new state, or error if invalid.
func (m *Machine) Apply(current State, kind TransitionKind, data interface{}) (State, error) {
	key := transitionKey{from: current, kind: kind}
	t, ok := m.transitions[key]
	if !ok {
		return current, fmt.Errorf("fsm %s: no transition %q from state %q", m.name, kind, current)
	}
	if t.Guard != nil {
		if err := t.Guard(data); err != nil {
			return current, fmt.Errorf("fsm %s: guard rejected %q from %q: %w", m.name, kind, current, err)
		}
	}
	return t.To, nil
}

// ValidTransitions returns all valid transition kinds from a given state.
func (m *Machine) ValidTransitions(current State) []TransitionKind {
	var kinds []TransitionKind
	for key := range m.transitions {
		if key.from == current {
			kinds = append(kinds, key.kind)
		}
	}
	return kinds
}

// Name returns the machine name.
func (m *Machine) Name() string {
	return m.name
}
