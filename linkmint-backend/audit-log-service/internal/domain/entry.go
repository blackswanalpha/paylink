// Package domain holds the audit-log core: the Entry model, the Store/Publisher ports, and the
// Service that appends to + queries + verifies the tamper-evident hash chain. It is non-custodial
// (A.1) — it records actions, it never moves funds — and append-only (no edits/deletes).
package domain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/paylink/audit-log-service/internal/hashing"
)

// ActorKind classifies who performed an action.
type ActorKind string

const (
	ActorUser    ActorKind = "user"
	ActorService ActorKind = "service"
	ActorSystem  ActorKind = "system"
)

// Valid reports whether k is one of the allowed kinds.
func (k ActorKind) Valid() bool {
	return k == ActorUser || k == ActorService || k == ActorSystem
}

// Actor identifies the principal behind an action. ID is optional (service/system actors may have
// none); when present it is a UUID matching the actor_id column.
type Actor struct {
	ID   *uuid.UUID
	Kind ActorKind
}

// Entry is one row of the audit chain. Before/After/Context are arbitrary JSON (JSONB columns).
// Canonical is the exact deterministic serialization that was hashed (persisted verbatim as
// canonical_bytes); verify recomputes entry_hash = SHA256(prev_hash || Canonical) from it, never
// re-canonicalizing the jsonb columns — so it is immune to Postgres jsonb number normalization.
type Entry struct {
	EntryID    int64
	OccurredAt time.Time
	Actor      Actor
	Action     string
	Resource   string
	Before     json.RawMessage
	After      json.RawMessage
	Context    json.RawMessage
	PrevHash   []byte
	EntryHash  []byte
	Canonical  []byte
}

// BuildEntry is the shared append core for both stores: it canonicalizes the input, links it to the
// prior tail hash, and computes the chain hash. It populates Canonical (persisted as canonical_bytes),
// PrevHash, and EntryHash; EntryID is assigned by the store on insert.
func BuildEntry(in AppendInput, prev []byte) (Entry, error) {
	e := Entry{
		OccurredAt: in.OccurredAt,
		Actor:      in.Actor,
		Action:     in.Action,
		Resource:   in.Resource,
		Before:     in.Before,
		After:      in.After,
		Context:    in.Context,
		PrevHash:   prev,
	}
	canon, err := hashing.Canonical(e.HashInput())
	if err != nil {
		return Entry{}, err
	}
	e.Canonical = canon
	e.EntryHash = hashing.Hash(prev, canon)
	return e, nil
}

// GenesisHash is the prev_hash of the first entry (32 zero bytes).
func GenesisHash() []byte { return hashing.Genesis() }

// HashInput projects the entry onto the content that is hashed (entry_id excluded).
func (e Entry) HashInput() hashing.Input {
	var id string
	if e.Actor.ID != nil {
		id = e.Actor.ID.String()
	}
	return hashing.Input{
		OccurredAt: e.OccurredAt,
		ActorID:    id,
		ActorKind:  string(e.Actor.Kind),
		Action:     e.Action,
		Resource:   e.Resource,
		Before:     e.Before,
		After:      e.After,
		Context:    e.Context,
	}
}

// CheckEntry verifies one entry during a chain walk:
//
//   - selfOK: the stored entry_hash equals SHA256(stored prev_hash || stored canonical_bytes) — i.e.
//     the row's hashed content and hash are internally consistent (detects an edited canonical_bytes,
//     entry_hash, or prev_hash).
//   - linkOK: the stored prev_hash equals expectedPrev (the predecessor's entry_hash, or genesis) —
//     i.e. the entry is still linked where it was inserted (detects a deleted/reordered predecessor).
//
// Callers report broken_at on the first entry where either check fails, evaluating selfOK first.
func CheckEntry(e Entry, expectedPrev []byte) (selfOK, linkOK bool) {
	recomputed := hashing.Hash(e.PrevHash, e.Canonical)
	selfOK = bytes.Equal(recomputed, e.EntryHash)
	linkOK = bytes.Equal(e.PrevHash, expectedPrev)
	return selfOK, linkOK
}

// Proof is the Phase-1 linear-chain inclusion proof returned by GET /v1/audit-log/{entry_id}.
// valid means the returned entry still hashes to its stored entry_hash. Merkle / on-chain-anchor
// proofs are Phase 2 — chain_type lets the response shape evolve without breaking clients.
type Proof struct {
	EntryID        int64  `json:"entry_id"`
	ChainType      string `json:"chain_type"`
	PrevHash       string `json:"prev_hash"`
	EntryHash      string `json:"entry_hash"`
	RecomputedHash string `json:"recomputed_hash"`
	Valid          bool   `json:"valid"`
}

// BuildProof recomputes the entry's hash from its stored canonical bytes and assembles its proof.
func BuildProof(e Entry) Proof {
	recomputed := hashing.Hash(e.PrevHash, e.Canonical)
	valid := bytes.Equal(recomputed, e.EntryHash)
	return Proof{
		EntryID:        e.EntryID,
		ChainType:      "linear",
		PrevHash:       hex.EncodeToString(e.PrevHash),
		EntryHash:      hex.EncodeToString(e.EntryHash),
		RecomputedHash: hex.EncodeToString(recomputed),
		Valid:          valid,
	}
}

// VerifyResult is the outcome of a chain verification over a range. BrokenAt is the entry_id of the
// first entry that failed, nil when OK.
type VerifyResult struct {
	OK       bool
	BrokenAt *int64
}
