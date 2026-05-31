// Package redisx is a thin Redis client wrapper shared by the idempotency store (Idempotency-Key
// replay) and the correlation store (CheckoutRequestID → PayLink mapping + callback dedupe). One
// connection pool, two consumers; both depend on small structural interfaces this type satisfies.
package redisx

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client adapts a *redis.Client to the SetNX/Set/Get/Del/Ping surface the stores use.
type Client struct {
	c *redis.Client
}

// New connects to Redis from a URL (redis://host:port/db).
func New(url string) (*Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &Client{c: redis.NewClient(opt)}, nil
}

// SetNX sets key only if absent.
func (r *Client) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return r.c.SetNX(ctx, key, value, ttl).Result()
}

// Set unconditionally sets key with a TTL.
func (r *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.c.Set(ctx, key, value, ttl).Err()
}

// Get returns the value and whether it existed.
func (r *Client) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := r.c.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// Del removes a key.
func (r *Client) Del(ctx context.Context, key string) error {
	return r.c.Del(ctx, key).Err()
}

// Ping checks connectivity.
func (r *Client) Ping(ctx context.Context) error {
	return r.c.Ping(ctx).Err()
}

// Close releases the connection pool.
func (r *Client) Close() error {
	return r.c.Close()
}
