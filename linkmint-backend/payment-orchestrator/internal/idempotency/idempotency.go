// Package idempotency implements the Redis-backed Idempotency-Key store for state-mutating
// endpoints (24h TTL). A request that re-presents the same key+body replays the cached response;
// the same key with a different body is a 409 conflict; an in-flight duplicate is also a conflict.
// Keys are namespaced per service+route so different routes never collide.
package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/paylink/payment-orchestrator/internal/config"
	"github.com/paylink/payment-orchestrator/internal/httpx"
)

// RedisLike is the subset of Redis operations used here (so tests can fake it).
type RedisLike interface {
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Del(ctx context.Context, key string) error
	Ping(ctx context.Context) error
}

// CachedResponse is a previously-completed response to replay.
type CachedResponse struct {
	Status int
	Body   json.RawMessage
}

// Store is the idempotency-key reservation store.
type Store struct {
	redis RedisLike
	ttl   time.Duration
}

// New builds a Store with the given TTL.
func New(redis RedisLike, ttl time.Duration) *Store {
	return &Store{redis: redis, ttl: ttl}
}

func (s *Store) key(route, key string) string {
	return "idem:" + config.ServiceName + ":" + route + ":" + key
}

// Fingerprint is a stable SHA-256 over the canonical request body bytes.
func Fingerprint(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

type record struct {
	State  string          `json:"state"` // "pending" | "completed"
	FP     string          `json:"fp"`
	Status int             `json:"status,omitempty"`
	Body   json.RawMessage `json:"body,omitempty"`
}

func backendErr(err error) error {
	return httpx.NewError(httpx.CodeInternalError, "idempotency backend error: "+err.Error(), nil)
}

// Begin reserves the key for this request, or surfaces a prior result / conflict:
//   - returns (cached, nil) when a completed result should be replayed;
//   - returns (nil, nil) when the caller now owns the key and must do the work then Complete;
//   - returns an IDEMPOTENT_CONFLICT AppError on a body mismatch or in-flight duplicate.
func (s *Store) Begin(ctx context.Context, route, key, fp string) (*CachedResponse, error) {
	rkey := s.key(route, key)
	pending, _ := json.Marshal(record{State: "pending", FP: fp})

	// Acquisition is ALWAYS via SETNX (never a plain SET), so two concurrent requests can never
	// both take ownership. The loop handles the narrow window where a reservation expires between
	// our SETNX and GET: we simply retry the SETNX.
	for attempt := 0; attempt < 2; attempt++ {
		reserved, err := s.redis.SetNX(ctx, rkey, string(pending), s.ttl)
		if err != nil {
			return nil, backendErr(err)
		}
		if reserved {
			return nil, nil
		}

		raw, found, err := s.redis.Get(ctx, rkey)
		if err != nil {
			return nil, backendErr(err)
		}
		if !found {
			continue // reservation vanished between SETNX and GET — retry acquisition
		}

		var rec record
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			return nil, backendErr(err)
		}
		if rec.FP != fp {
			return nil, httpx.NewError(httpx.CodeIdempotentConflict,
				"Idempotency-Key was already used with a different request body", nil)
		}
		if rec.State == "completed" {
			return &CachedResponse{Status: rec.Status, Body: rec.Body}, nil
		}
		return nil, httpx.NewError(httpx.CodeIdempotentConflict,
			"a request with this Idempotency-Key is still in progress", nil)
	}
	// Lost the acquisition race twice in a row — treat as in-progress to stay safe.
	return nil, httpx.NewError(httpx.CodeIdempotentConflict,
		"a request with this Idempotency-Key is still in progress", nil)
}

// Complete stores the final response so future Begin calls with the same key+body replay it.
func (s *Store) Complete(ctx context.Context, route, key, fp string, status int, body json.RawMessage) error {
	rec, _ := json.Marshal(record{State: "completed", FP: fp, Status: status, Body: body})
	if err := s.redis.Set(ctx, s.key(route, key), string(rec), s.ttl); err != nil {
		return backendErr(err)
	}
	return nil
}

// Release drops a pending reservation so a failed request can be retried with the same key.
// It uses a cancel-detached context so the cleanup still runs if the client has disconnected
// (otherwise the key would stay pending until TTL, blocking legitimate retries).
func (s *Store) Release(ctx context.Context, route, key string) {
	_ = s.redis.Del(context.WithoutCancel(ctx), s.key(route, key))
}

// Ping checks the idempotency backend for readiness.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.redis.Ping(ctx); err != nil {
		return backendErr(err)
	}
	return nil
}
