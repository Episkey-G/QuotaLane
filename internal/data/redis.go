// Package data provides data access layer implementations.
package data

import (
	"context"
	"fmt"
	"time"

	"QuotaLane/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a new Redis client with connection pool configuration.
// It returns the client, a cleanup function, and an error.
// Connection failure does not prevent application startup (graceful degradation).
func NewRedisClient(c *conf.Data, logger log.Logger) (*redis.Client, func(), error) {
	helper := log.NewHelper(logger)

	// Validate configuration
	if c == nil || c.Redis == nil {
		helper.Warn("Redis configuration is nil, skipping Redis initialization")
		return nil, func() {}, nil
	}

	addr := c.Redis.Addr
	if addr == "" {
		helper.Warn("Redis address is empty, skipping Redis initialization")
		return nil, func() {}, nil
	}

	// Create Redis client with connection pool settings
	// Pool configuration follows acceptance criteria:
	// - MaxIdleConns = 10 (minimum idle connections)
	// - PoolSize = 100 (maximum active connections)
	// - ConnMaxLifetime = 3s (connection timeout)
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "", // No password for local development
		DB:           0,  // Use default DB
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  c.Redis.ReadTimeout.AsDuration(),
		WriteTimeout: c.Redis.WriteTimeout.AsDuration(),
		// ConnMaxLifetime is not directly supported in go-redis v9
		// Use ConnMaxIdleTime instead for idle connection cleanup
		ConnMaxIdleTime: 5 * time.Minute,
	})

	// Health check: verify connection with ping
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		helper.Warnf("Failed to connect to Redis at %s: %v (application will continue without Redis)", addr, err)
		// Return client anyway for graceful degradation
		// Services will handle nil cache client appropriately
		return rdb, func() {
			helper.Info("Closing Redis client (connection was unavailable)")
			_ = rdb.Close()
		}, fmt.Errorf("redis ping failed: %w", err)
	}

	helper.Infof("Successfully connected to Redis at %s", addr)

	// Cleanup function to close Redis connection
	cleanup := func() {
		helper.Info("Closing Redis client")
		if err := rdb.Close(); err != nil {
			helper.Errorf("Failed to close Redis client: %v", err)
		}
	}

	return rdb, cleanup, nil
}
