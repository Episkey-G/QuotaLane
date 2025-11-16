package data

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// RateLimitRepo implements biz.RateLimitRepo interface.
// Following Kratos v2 DDD architecture, interface is defined in biz layer.
type RateLimitRepo struct {
	rdb    *redis.Client
	logger *log.Helper
}

// NewRateLimitRepo creates a new rate limit repository.
func NewRateLimitRepo(rdb *redis.Client, logger log.Logger) *RateLimitRepo {
	return &RateLimitRepo{
		rdb:    rdb,
		logger: log.NewHelper(logger),
	}
}

// IncrementRPM increments the RPM (Requests Per Minute) counter for an account.
// Uses Redis INCR with automatic expiration (60 seconds) on first increment.
// Returns the new count and any error.
func (r *RateLimitRepo) IncrementRPM(ctx context.Context, accountID int64) (int32, error) {
	if r.rdb == nil {
		return 0, fmt.Errorf("redis client is nil")
	}

	key := getRateLimitKey(accountID, "rpm")

	// Increment counter
	count, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment RPM: %w", err)
	}

	// Set expiration on first increment (atomic operation)
	if count == 1 {
		if err := r.rdb.Expire(ctx, key, 60).Err(); err != nil {
			r.logger.Warnf("Failed to set RPM expiration for account %d: %v", accountID, err)
			// Don't return error, counter is still incremented
		}
	}

	// Prevent overflow when converting int64 to int32
	if count > 2147483647 {
		count = 2147483647
	}

	return int32(count), nil // #nosec G115 -- overflow is handled above
}

// GetRPMCount retrieves the current RPM count for an account.
// Returns 0 if key doesn't exist.
func (r *RateLimitRepo) GetRPMCount(ctx context.Context, accountID int64) (int32, error) {
	if r.rdb == nil {
		return 0, fmt.Errorf("redis client is nil")
	}

	key := getRateLimitKey(accountID, "rpm")

	count, err := r.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key doesn't exist, return 0
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get RPM count: %w", err)
	}

	// Parse count
	countInt, err := strconv.ParseInt(count, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse RPM count: %w", err)
	}

	return int32(countInt), nil
}

// IncrementTPM increments the TPM (Tokens Per Minute) counter for an account.
// Uses Redis INCRBY with automatic expiration (60 seconds) on first increment.
// Returns the new count and any error.
func (r *RateLimitRepo) IncrementTPM(ctx context.Context, accountID int64, tokens int32) (int32, error) {
	if r.rdb == nil {
		return 0, fmt.Errorf("redis client is nil")
	}

	key := getRateLimitKey(accountID, "tpm")

	// Get current count first to detect first increment
	_, err := r.rdb.Get(ctx, key).Result()
	isFirstIncrement := (err == redis.Nil)

	// Increment counter by tokens
	count, err := r.rdb.IncrBy(ctx, key, int64(tokens)).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment TPM: %w", err)
	}

	// Set expiration on first increment
	if isFirstIncrement {
		if err := r.rdb.Expire(ctx, key, 60).Err(); err != nil {
			r.logger.Warnf("Failed to set TPM expiration for account %d: %v", accountID, err)
		}
	}

	// Prevent overflow when converting int64 to int32
	if count > 2147483647 {
		count = 2147483647
	}

	return int32(count), nil // #nosec G115 -- overflow is handled above
}

// GetTPMCount retrieves the current TPM count for an account.
// Returns 0 if key doesn't exist.
func (r *RateLimitRepo) GetTPMCount(ctx context.Context, accountID int64) (int32, error) {
	if r.rdb == nil {
		return 0, fmt.Errorf("redis client is nil")
	}

	key := getRateLimitKey(accountID, "tpm")

	count, err := r.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key doesn't exist, return 0
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get TPM count: %w", err)
	}

	// Parse count
	countInt, err := strconv.ParseInt(count, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse TPM count: %w", err)
	}

	return int32(countInt), nil
}

// AddConcurrencyRequest adds a request to the concurrency tracking sorted set.
// Uses Redis ZADD with the timestamp as score.
func (r *RateLimitRepo) AddConcurrencyRequest(ctx context.Context, accountID int64, requestID string, timestamp int64) error {
	if r.rdb == nil {
		return fmt.Errorf("redis client is nil")
	}

	key := getConcurrencyKey(accountID)

	// Add request to sorted set (score = timestamp, member = requestID)
	if err := r.rdb.ZAdd(ctx, key, redis.Z{
		Score:  float64(timestamp),
		Member: requestID,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add concurrency request: %w", err)
	}

	return nil
}

// RemoveConcurrencyRequest removes a request from the concurrency tracking sorted set.
// Uses Redis ZREM.
func (r *RateLimitRepo) RemoveConcurrencyRequest(ctx context.Context, accountID int64, requestID string) error {
	if r.rdb == nil {
		return fmt.Errorf("redis client is nil")
	}

	key := getConcurrencyKey(accountID)

	// Remove request from sorted set
	if err := r.rdb.ZRem(ctx, key, requestID).Err(); err != nil {
		return fmt.Errorf("failed to remove concurrency request: %w", err)
	}

	return nil
}

// GetConcurrencyCount retrieves the current concurrency count for an account.
// Uses Redis ZCARD to count members in the sorted set.
func (r *RateLimitRepo) GetConcurrencyCount(ctx context.Context, accountID int64) (int32, error) {
	if r.rdb == nil {
		return 0, fmt.Errorf("redis client is nil")
	}

	key := getConcurrencyKey(accountID)

	// Count members in sorted set
	count, err := r.rdb.ZCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get concurrency count: %w", err)
	}

	// Prevent overflow when converting int64 to int32
	if count > 2147483647 {
		count = 2147483647
	}

	return int32(count), nil // #nosec G115 -- overflow is handled above
}

// CleanupExpiredConcurrency removes expired requests from the concurrency tracking sorted set.
// Uses Redis ZREMRANGEBYSCORE to remove requests older than expiredBefore timestamp.
func (r *RateLimitRepo) CleanupExpiredConcurrency(ctx context.Context, accountID int64, expiredBefore int64) error {
	if r.rdb == nil {
		return fmt.Errorf("redis client is nil")
	}

	key := getConcurrencyKey(accountID)

	// Remove requests with score (timestamp) less than expiredBefore
	removedCount, err := r.rdb.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(expiredBefore, 10)).Result()
	if err != nil {
		return fmt.Errorf("failed to cleanup expired concurrency: %w", err)
	}

	if removedCount > 0 {
		r.logger.Debugw("Cleaned up expired concurrency requests",
			"account_id", accountID,
			"removed_count", removedCount)
	}

	return nil
}

// getRateLimitKey generates a Redis key for rate limiting.
// Format: rate:{account_id}:{type}
// Example: rate:123:rpm or rate:123:tpm
func getRateLimitKey(accountID int64, limitType string) string {
	return fmt.Sprintf("rate:%d:%s", accountID, limitType)
}

// getConcurrencyKey generates a Redis key for concurrency tracking.
// Format: concurrency:{account_id}
// Example: concurrency:123
func getConcurrencyKey(accountID int64) string {
	return fmt.Sprintf("concurrency:%d", accountID)
}
