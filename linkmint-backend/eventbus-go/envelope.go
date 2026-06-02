package eventbus

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Envelope is the wire format for every domain event on the bus. Its JSON encoding is canonical and
// byte-identical to the Python client (linkmint_eventbus), so either language can produce an event
// the other consumes. See workload/catalog.md.
//
// The field order (id, name, key, correlation_id, occurred_at, source, payload) is part of the wire
// contract and MUST NOT change. Payload is stored already-canonicalized (keys recursively sorted,
// compact, no HTML escaping); Marshal emits it inline.
type Envelope struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Key           string          `json:"key"`
	CorrelationID string          `json:"correlation_id"`
	OccurredAt    string          `json:"occurred_at"`
	Source        string          `json:"source"`
	Payload       json.RawMessage `json:"payload"`
}

// occurredAtLayout is RFC3339, UTC, seconds precision, with a literal Z (never a +00:00 offset or
// fractional seconds) — the format the Python lib emits too.
const occurredAtLayout = "2006-01-02T15:04:05Z"

// NewEnvelope builds an envelope, stamping a fresh UUID id and the current UTC time. The payload is
// canonicalized so struct field order vs map key order never changes the bytes.
func NewEnvelope(name, key, correlationID, source string, payload any) (Envelope, error) {
	canon, err := canonicalPayload(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		ID:            uuid.NewString(),
		Name:          name,
		Key:           key,
		CorrelationID: correlationID,
		OccurredAt:    time.Now().UTC().Format(occurredAtLayout),
		Source:        source,
		Payload:       canon,
	}, nil
}

// Marshal encodes the envelope to canonical JSON: compact, no HTML escaping, no trailing newline.
// Byte-identical to the Python lib's to_canonical_bytes.
func (e Envelope) Marshal() ([]byte, error) {
	return encodeCanonical(e)
}

// UnmarshalEnvelope decodes envelope bytes. Unknown fields are ignored (forward-compat).
func UnmarshalEnvelope(b []byte) (Envelope, error) {
	var e Envelope
	err := json.Unmarshal(b, &e)
	return e, err
}

// canonicalPayload marshals an arbitrary payload, then re-encodes it with map keys recursively
// sorted (Go's encoding/json sorts map keys) and numbers preserved exactly via json.Number (no
// float rounding). A struct payload is normalized to sorted-key map order this way too.
func canonicalPayload(payload any) (json.RawMessage, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	out, err := encodeCanonical(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

// encodeCanonical writes v as compact JSON with HTML escaping disabled and the trailing newline
// (added by json.Encoder) trimmed. For maps Go sorts keys; for structs it uses field order; an
// inlined json.RawMessage is passed through (compacted, not re-escaped).
func encodeCanonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
