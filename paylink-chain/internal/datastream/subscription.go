package datastream

import "github.com/paylink/paylink-chain/internal/events"

// Subscription holds the active filter for a WebSocket connection.
type Subscription struct {
	entityTypes map[events.EntityType]bool
	entityIDs   map[string]bool
	eventKinds  map[events.EventKind]bool
	transitions map[string]bool
	matchAll    bool
}

// NewSubscription creates a subscription from a filter.
// A nil/empty filter subscribes to all events.
func NewSubscription(filter *SubscribeFilter) *Subscription {
	s := &Subscription{
		entityTypes: make(map[events.EntityType]bool),
		entityIDs:   make(map[string]bool),
		eventKinds:  make(map[events.EventKind]bool),
		transitions: make(map[string]bool),
	}

	if filter == nil {
		s.matchAll = true
		return s
	}

	for _, et := range filter.EntityTypes {
		s.entityTypes[events.EntityType(et)] = true
	}
	for _, id := range filter.EntityIDs {
		s.entityIDs[id] = true
	}
	for _, ek := range filter.EventKinds {
		s.eventKinds[events.EventKind(ek)] = true
	}
	for _, t := range filter.Transitions {
		s.transitions[t] = true
	}

	if len(s.entityTypes) == 0 && len(s.entityIDs) == 0 &&
		len(s.eventKinds) == 0 && len(s.transitions) == 0 {
		s.matchAll = true
	}

	return s
}

// Matches returns true if the event passes this subscription's filter.
// Filter dimensions are ANDed; values within a dimension are ORed.
func (s *Subscription) Matches(evt *events.Event) bool {
	if s.matchAll {
		return true
	}

	if len(s.entityTypes) > 0 && !s.entityTypes[evt.EntityType] {
		return false
	}
	if len(s.entityIDs) > 0 && !s.entityIDs[evt.EntityID] {
		return false
	}
	if len(s.eventKinds) > 0 && !s.eventKinds[evt.Kind] {
		return false
	}
	if len(s.transitions) > 0 && !s.transitions[string(evt.Transition)] {
		return false
	}

	return true
}
