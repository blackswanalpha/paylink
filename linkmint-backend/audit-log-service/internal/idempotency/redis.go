package idempotency

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient adapts a *redis.Client to RedisLike.
type RedisClient struct {
	c *redis.Client
}

// NewRedisClient connects to Redis from a URL (redis://host:port/db).
func NewRedisClient(url string) (*RedisClient, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &RedisClient{c: redis.NewClient(opt)}, nil
}

// SetNX sets key only if absent.
func (r *RedisClient) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return r.c.SetNX(ctx, key, value, ttl).Result()
}

// Set unconditionally sets key with a TTL.
func (r *RedisClient) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.c.Set(ctx, key, value, ttl).Err()
}

// Get returns the value and whether it existed.
func (r *RedisClient) Get(ctx context.Context, key string) (string, bool, error) {
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
func (r *RedisClient) Del(ctx context.Context, key string) error {
	return r.c.Del(ctx, key).Err()
}

// Ping checks connectivity.
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.c.Ping(ctx).Err()
}

// Close releases the connection pool.
func (r *RedisClient) Close() error {
	return r.c.Close()
}
