package datastream

import "encoding/json"

// ClientMessage is a message sent from the client to the server.
type ClientMessage struct {
	Action string           `json:"action"`           // "subscribe", "unsubscribe", "ping"
	ID     string           `json:"id,omitempty"`     // client-assigned request ID for correlation
	Filter *SubscribeFilter `json:"filter,omitempty"` // filter for subscribe action
}

// SubscribeFilter defines what events a client wants to receive.
type SubscribeFilter struct {
	EntityTypes []string `json:"entityTypes,omitempty"` // ["paylink", "validator", "account", "block", "tx"]
	EntityIDs   []string `json:"entityIds,omitempty"`   // specific entity hex IDs
	EventKinds  []string `json:"eventKinds,omitempty"`  // specific event kinds
	Transitions []string `json:"transitions,omitempty"` // specific FSM transition kinds
}

// ServerMessage is a message sent from the server to the client.
type ServerMessage struct {
	Type  string          `json:"type"`            // "event", "subscribed", "unsubscribed", "error", "pong"
	ID    string          `json:"id,omitempty"`    // echoed client request ID
	Event json.RawMessage `json:"event,omitempty"` // the Event JSON for type="event"
	Error string          `json:"error,omitempty"` // for type="error"
	Info  string          `json:"info,omitempty"`  // for type="subscribed"/"unsubscribed"
}
