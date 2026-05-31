package correlation

import (
	"context"
	"encoding/json"
	"time"
)

// conn is the subset of Redis ops the store needs (satisfied by *redisx.Client; fakeable in tests).
type conn interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (value string, found bool, err error)
}

// Redis is the Redis-backed Store. Correlations expire after the configured TTL (≈ PayLink expiry)
// so stale entries self-clean.
type Redis struct {
	c   conn
	ttl time.Duration
}

// NewRedis builds a Redis-backed correlation store.
func NewRedis(c conn, ttl time.Duration) *Redis { return &Redis{c: c, ttl: ttl} }

func recordKey(id string) string { return "corr:" + id }

// Put stores the correlation for id with the configured TTL.
func (r *Redis) Put(ctx context.Context, id string, rec Record) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return r.c.Set(ctx, recordKey(id), string(b), r.ttl)
}

// Get returns the correlation for id, or ErrNotFound.
func (r *Redis) Get(ctx context.Context, id string) (Record, error) {
	raw, found, err := r.c.Get(ctx, recordKey(id))
	if err != nil {
		return Record{}, err
	}
	if !found {
		return Record{}, ErrNotFound
	}
	var rec Record
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}
