package idempotency

import "errors"

// Sentinel errors. Callers errors.Is() against these and render their own error envelope — the library
// stays transport-free (no httpx/chi import), so the HTTP-status mapping lives at the service boundary.
var (
	// ErrConflict marks an Idempotency-Key conflict: the same key re-presented with a different request
	// body, or while a first request with that key is still in flight. Map it to 409 IDEMPOTENT_CONFLICT.
	ErrConflict = errors.New("idempotency: conflict")

	// ErrBackend marks a failure talking to the Redis/DB backend. Map it to 500 in the caller's envelope.
	ErrBackend = errors.New("idempotency: backend error")
)

// ConflictError carries the conflict Reason ("body_mismatch" | "in_flight") and a human message; it
// unwraps to ErrConflict so errors.Is(err, ErrConflict) holds while errors.As exposes the Reason.
type ConflictError struct {
	Reason string
	Msg    string
}

func (e *ConflictError) Error() string { return e.Msg }
func (e *ConflictError) Unwrap() error { return ErrConflict }
