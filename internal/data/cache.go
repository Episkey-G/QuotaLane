// Package data provides data access layer implementations.
package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache key prefixes and TTL constants as defined in AC#2
const (
	// CacheKeyAccount is the prefix for account caches: account:{id}
	CacheKeyAccount = "account"
	// CacheKeyUser is the prefix for user caches: user:{id}
	CacheKeyUser = "user"
	// CacheKeySession is the prefix for JWT session caches: session:{token}
	CacheKeySession = "session"
	// CacheKeySticky is the prefix for sticky session caches: sticky:{hash}
	CacheKeySticky = "sticky"
	// CacheKeyRate is the prefix for rate limit caches: rate:{keyId}:{window}
	CacheKeyRate = "rate"
	// CacheKeyPlan is the prefix for subscription plan caches: plan:{id}
	CacheKeyPlan = "plan"
)

// Cache TTL durations as defined in AC#2
const (
	// TTLAccount is the TTL for account caches (5 minutes)
	TTLAccount = 5 * time.Minute
	// TTLUser is the TTL for user caches (5 minutes)
	TTLUser = 5 * time.Minute
	// TTLSession is the TTL for JWT session caches (24 hours)
	TTLSession = 24 * time.Hour
	// TTLSticky is the TTL for sticky session caches (1 hour)
	TTLSticky = 1 * time.Hour
	// TTLRate is the TTL for rate limit counters (1 minute)
	TTLRate = 1 * time.Minute
	// TTLPlan is the TTL for subscription plan caches (10 minutes)
	TTLPlan = 10 * time.Minute
)

// ErrCacheNotFound is returned when a cache key does not exist
var ErrCacheNotFound = errors.New("cache: key not found")

// CacheClient defines the interface for cache operations.
// Implementations must be thread-safe and handle serialization/deserialization.
type CacheClient interface {
	// Get retrieves a value from cache and deserializes it into dest.
	// Returns ErrCacheNotFound if key doesn't exist.
	Get(ctx context.Context, key string, dest interface{}) error

	// Set stores a value in cache with the specified TTL.
	// The value is serialized to JSON before storage.
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a key from cache.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in cache.
	Exists(ctx context.Context, key string) (bool, error)
}

// redisCache is the Redis-based implementation of CacheClient.
type redisCache struct {
	client *redis.Client
}

// NewCacheClient creates a new Redis-based cache client.
// If the Redis client is nil, cache operations will gracefully fail.
func NewCacheClient(rdb *redis.Client) CacheClient {
	return &redisCache{
		client: rdb,
	}
}

// Get retrieves a value from cache and deserializes it into dest.
// Returns ErrCacheNotFound if the key doesn't exist (redis.Nil).
func (c *redisCache) Get(ctx context.Context, key string, dest interface{}) error {
	if c.client == nil {
		return errors.New("cache: redis client is nil")
	}

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheNotFound
		}
		return fmt.Errorf("cache: failed to get key %s: %w", key, err)
	}

	// Deserialize JSON into dest
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return fmt.Errorf("cache: failed to unmarshal value for key %s: %w", key, err)
	}

	return nil
}

// Set stores a value in cache with the specified TTL.
// The value is serialized to JSON before storage.
func (c *redisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c.client == nil {
		return errors.New("cache: redis client is nil")
	}

	// Serialize value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache: failed to marshal value for key %s: %w", key, err)
	}

	// Store in Redis with TTL
	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache: failed to set key %s: %w", key, err)
	}

	return nil
}

// Delete removes a key from cache.
func (c *redisCache) Delete(ctx context.Context, key string) error {
	if c.client == nil {
		return errors.New("cache: redis client is nil")
	}

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("cache: failed to delete key %s: %w", key, err)
	}

	return nil
}

// Exists checks if a key exists in cache.
func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	if c.client == nil {
		return false, errors.New("cache: redis client is nil")
	}

	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("cache: failed to check existence of key %s: %w", key, err)
	}

	return count > 0, nil
}

// BuildCacheKey constructs a cache key with the appropriate prefix.
// Examples:
//   - BuildCacheKey(CacheKeyAccount, "123") -> "account:123"
//   - BuildCacheKey(CacheKeyRate, "abc", "1m") -> "rate:abc:1m"
func BuildCacheKey(prefix string, parts ...string) string {
	key := prefix
	for _, part := range parts {
		key += ":" + part
	}
	return key
}
