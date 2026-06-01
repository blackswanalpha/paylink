package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/paylink/audit-log-service/internal/httpx"
)

// ErrNotFound is returned by the store when no entry matches a lookup.
var ErrNotFound = errors.New("audit entry not found")

// Field-size caps (bound row size; reject oversized intake with INVALID_PAYLOAD).
const (
	maxActionLen   = 200
	maxResourceLen = 512
	maxJSONBytes   = 256 * 1024 // 256 KiB per JSONB field
)

// AppendInput is the validated content for a new entry. OccurredAt may be zero (the service stamps
// now); the Actor/Action/Resource/Context are required.
type AppendInput struct {
	OccurredAt time.Time
	Actor      Actor
	Action     string
	Resource   string
	Before     json.RawMessage
	After      json.RawMessage
	Context    json.RawMessage
}

// Validate enforces the intake contract, returning an INVALID_PAYLOAD AppError on any violation.
// It runs on every append path (HTTP handler and the future NATS consumer), so validation is not
// duplicated per transport.
func (in AppendInput) Validate() error {
	if !in.Actor.Kind.Valid() {
		return httpx.NewError(httpx.CodeInvalidPayload, "actor.kind must be one of user|service|system", nil)
	}
	if strings.TrimSpace(in.Action) == "" {
		return httpx.NewError(httpx.CodeInvalidPayload, "action is required", nil)
	}
	if len(in.Action) > maxActionLen {
		return httpx.NewError(httpx.CodeInvalidPayload, "action exceeds the maximum length", map[string]any{"max": maxActionLen})
	}
	if !utf8.ValidString(in.Action) || !utf8.ValidString(in.Resource) {
		return httpx.NewError(httpx.CodeInvalidPayload, "action and resource must be valid UTF-8", nil)
	}
	if strings.TrimSpace(in.Resource) == "" {
		return httpx.NewError(httpx.CodeInvalidPayload, "resource is required", nil)
	}
	if len(in.Resource) > maxResourceLen {
		return httpx.NewError(httpx.CodeInvalidPayload, "resource exceeds the maximum length", map[string]any{"max": maxResourceLen})
	}
	if !isJSONObject(in.Context) {
		return httpx.NewError(httpx.CodeInvalidPayload, "context is required and must be a JSON object", nil)
	}
	for field, raw := range map[string]json.RawMessage{"before": in.Before, "after": in.After, "context": in.Context} {
		if len(raw) > maxJSONBytes {
			return httpx.NewError(httpx.CodeInvalidPayload, field+" exceeds the maximum size", map[string]any{"max": maxJSONBytes})
		}
		if len(raw) > 0 && !json.Valid(raw) {
			return httpx.NewError(httpx.CodeInvalidPayload, field+" is not valid JSON", nil)
		}
	}
	return nil
}

// isJSONObject reports whether raw is a non-empty JSON object.
func isJSONObject(raw json.RawMessage) bool {
	t := strings.TrimSpace(string(raw))
	if len(t) == 0 || t[0] != '{' {
		return false
	}
	return json.Valid(raw)
}

// QueryFilter selects entries for the list endpoint. Cursor is an entry_id high-water mark
// (exclusive, for newest-first paging); 0 means start at the newest.
type QueryFilter struct {
	Actor    *uuid.UUID
	Resource string
	From     *time.Time
	To       *time.Time
	Cursor   int64
	Limit    int
}

// Page is a page of entries plus the cursor for the next page (nil when exhausted).
type Page struct {
	Items      []Entry
	NextCursor *int64
}

// Store persists the append-only hash chain. Implementations: store/memory (tests, dev) and
// store/postgres (production). Append MUST serialize against concurrent appends so prev_hash always
// links to the true tail.
type Store interface {
	// Append links the entry to the current tail, computes its entry_hash, inserts it, and returns
	// the stored entry with EntryID + hashes populated.
	Append(ctx context.Context, in AppendInput) (Entry, error)
	// GetByID returns the entry, or ErrNotFound.
	GetByID(ctx context.Context, id int64) (Entry, error)
	// Query returns a newest-first page of entries matching the filter.
	Query(ctx context.Context, f QueryFilter) (Page, error)
	// VerifyRange walks the (optionally time-bounded) chain in entry_id order, seeding linkage from
	// the entry immediately preceding the range (or genesis), and returns the first break.
	VerifyRange(ctx context.Context, from, to *time.Time) (VerifyResult, error)
	// Tail returns the current head entry_hash (genesis when empty) and the entry count.
	Tail(ctx context.Context) (hash []byte, count int64, err error)
	Ping(ctx context.Context) error
}

// Publisher emits domain events by logical name (transport seam — Kafka/SQS/NATS, ADR-004).
type Publisher interface {
	Publish(ctx context.Context, name, key string, payload any) error
}
