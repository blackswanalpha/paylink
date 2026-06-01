// Package hashing is the tamper-evidence core: it produces a deterministic canonical JSON
// encoding of an audit entry and the chain hash entry_hash = SHA256(prev_hash || canonical_json).
//
// Determinism is the whole game — verify must recompute byte-for-byte what append computed, even
// after the JSONB fields have round-tripped through Postgres. Two rules make that hold:
//
//   - Objects are emitted with sorted keys. encoding/json marshals a map[string]any with its keys
//     in sorted order, so we canonicalize via map[string]any (never struct field order), and we
//     decode the arbitrary JSONB fields into any (objects → map[string]any) so every nested object
//     is recursively sorted. Arrays keep their order (order is semantically significant).
//   - Numbers keep their exact source lexeme. We decode with json.Decoder.UseNumber(), so a number
//     becomes a json.Number (its digit string) rather than a float64; encoding/json then re-emits
//     it verbatim. This avoids the float64 round-trip (1e6→1000000, precision loss past 2^53) that
//     would otherwise make a re-canonicalized entry hash differently from the original.
//
// This package imports neither domain nor store — it is a leaf so those packages can depend on it
// without a cycle.
package hashing

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"time"
)

// HashLen is the byte length of a chain hash (SHA-256).
const HashLen = 32

// Genesis is the prev_hash of the first entry in the chain: HashLen zero bytes.
func Genesis() []byte { return make([]byte, HashLen) }

// Input is the content of an audit entry that participates in the hash. entry_id is deliberately
// excluded: it is a DB-assigned BIGSERIAL that cannot exist before the hash is computed, and the
// chain's prev_hash linkage — not the serial — is what pins an entry's position.
type Input struct {
	OccurredAt time.Time
	ActorID    string // "" => null in the canonical form
	ActorKind  string
	Action     string
	Resource   string
	Before     json.RawMessage // nil/empty/null => null
	After      json.RawMessage // nil/empty/null => null
	Context    json.RawMessage // required by the caller; canonicalized like the others
}

// Canonical returns the deterministic canonical JSON bytes of in. The output object has these
// keys (emitted sorted by encoding/json): action, actor, after, before, context, occurred_at,
// resource. occurred_at is normalized to RFC3339 nanos in UTC, truncated to microseconds so the
// write-time value matches what Postgres timestamptz round-trips back.
func Canonical(in Input) ([]byte, error) {
	before, err := decodeJSON(in.Before)
	if err != nil {
		return nil, err
	}
	after, err := decodeJSON(in.After)
	if err != nil {
		return nil, err
	}
	ctx, err := decodeJSON(in.Context)
	if err != nil {
		return nil, err
	}

	obj := map[string]any{
		"occurred_at": in.OccurredAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano),
		"actor":       canonicalActor(in.ActorID, in.ActorKind),
		"action":      in.Action,
		"resource":    in.Resource,
		"before":      before,
		"after":       after,
		"context":     ctx,
	}
	return json.Marshal(obj)
}

// Hash returns SHA256(prev || canonical), the chain hash of an entry whose canonical bytes are
// already computed. prev must be exactly HashLen raw bytes (the previous entry's entry_hash, or
// Genesis() for the first entry); it is prepended unmodified, so the concatenation is unambiguous
// (fixed-width prefix, no separator). Verify hashes the persisted canonical_bytes through here, so
// it never re-canonicalizes (and so is immune to Postgres jsonb number normalization).
func Hash(prev, canonical []byte) []byte {
	h := sha256.New()
	h.Write(prev)
	h.Write(canonical)
	return h.Sum(nil)
}

// EntryHash canonicalizes in and returns Hash(prev, canonical). Append computes Canonical and Hash
// separately so it can also persist the canonical bytes; this is the convenience composition used
// by tests.
func EntryHash(prev []byte, in Input) ([]byte, error) {
	canon, err := Canonical(in)
	if err != nil {
		return nil, err
	}
	return Hash(prev, canon), nil
}

func canonicalActor(id, kind string) map[string]any {
	var idv any // nil => null
	if id != "" {
		idv = id
	}
	return map[string]any{"id": idv, "kind": kind}
}

// decodeJSON decodes a JSONB field into a generic value with UseNumber so re-marshalling is
// byte-stable. Absent and explicit null both decode to nil (canonicalize to "null").
func decodeJSON(raw json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}
