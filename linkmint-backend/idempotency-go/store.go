package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
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
	redis   RedisLike
	service string
	ttl     time.Duration
}

// New builds a Store for the named service with the given TTL. The service name is part of the Redis
// key namespace ("idem:<service>:<route>:<key>"), so each service's keys are isolated even on a shared
// Redis. Pass your service's canonical name (e.g. config.ServiceName).
func New(redis RedisLike, service string, ttl time.Duration) *Store {
	return &Store{redis: redis, service: service, ttl: ttl}
}

func (s *Store) key(route, key string) string {
	return "idem:" + s.service + ":" + route + ":" + key
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
	return fmt.Errorf("%w: %v", ErrBackend, err)
}

// Begin reserves the key for this request, or surfaces a prior result / conflict:
//   - returns (cached, nil) when a completed result should be replayed;
//   - returns (nil, nil) when the caller now owns the key and must do the work then Complete;
//   - returns a *ConflictError (errors.Is ErrConflict) on a body mismatch or in-flight duplicate.
func (s *Store) Begin(ctx context.Context, route, key, fp string) (*CachedResponse, error) {
	rkey := s.key(route, key)
	pending, _ := json.Marshal(record{State: "pending", FP: fp})

	// Acquisition is ALWAYS via SETNX (never a plain SET), so two concurrent requests can never both
	// take ownership. The loop handles the narrow window where a reservation expires between our SETNX
	// and GET: we simply retry the SETNX.
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
			return nil, &ConflictError{
				Reason: "body_mismatch",
				Msg:    "Idempotency-Key was already used with a different request body",
			}
		}
		if rec.State == "completed" {
			return &CachedResponse{Status: rec.Status, Body: rec.Body}, nil
		}
		return nil, &ConflictError{Reason: "in_flight", Msg: "a request with this Idempotency-Key is still in progress"}
	}
	// Lost the acquisition race twice in a row — treat as in-progress to stay safe.
	return nil, &ConflictError{Reason: "in_flight", Msg: "a request with this Idempotency-Key is still in progress"}
}

// Complete stores the final response so future Begin calls with the same key+body replay it.
func (s *Store) Complete(ctx context.Context, route, key, fp string, status int, body json.RawMessage) error {
	rec, _ := json.Marshal(record{State: "completed", FP: fp, Status: status, Body: body})
	if err := s.redis.Set(ctx, s.key(route, key), string(rec), s.ttl); err != nil {
		return backendErr(err)
	}
	return nil
}

// Release drops a pending reservation so a failed request can be retried with the same key. It uses a
// cancel-detached context so the cleanup still runs if the client has disconnected (otherwise the key
// would stay pending until TTL, blocking legitimate retries).
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
