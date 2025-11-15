// Package data provides data access layer implementations.
// It handles database connections and data persistence.
package data

import (
	"QuotaLane/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewRedisClient,
	NewCacheClient,
	NewMySQLClient,
	NewAccountRepo,
	NewRateLimitRepo,
)

// Data contains all data layer dependencies.
type Data struct {
	// redisClient is the Redis client for caching
	redisClient *redis.Client
	// cache is the cache interface for repository use
	cache CacheClient
	// Note: MySQL DB is not stored here, it's injected directly to repositories
}

// NewData creates a new Data instance with all data layer dependencies.
// Redis connection failure does not prevent application startup (graceful degradation).
func NewData(_ *conf.Data, logger log.Logger, rdb *redis.Client, cache CacheClient) (*Data, func(), error) {
	helper := log.NewHelper(logger)

	// Check if Redis is available
	if rdb == nil {
		helper.Warn("Redis client is nil, caching will be unavailable")
	}

	d := &Data{
		redisClient: rdb,
		cache:       cache,
	}

	cleanup := func() {
		helper.Info("closing the data resources")
		// Redis cleanup is handled by NewRedisClient's cleanup function
		// which is called automatically by Wire
	}

	return d, cleanup, nil
}

// GetCache returns the cache client for repository use.
func (d *Data) GetCache() CacheClient {
	return d.cache
}

// GetRedisClient returns the Redis client for advanced operations.
func (d *Data) GetRedisClient() *redis.Client {
	return d.redisClient
}
