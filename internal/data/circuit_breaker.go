package data

import (
	"context"
	"fmt"
	"time"

	//  - removed to avoid circular dependency
	"QuotaLane/internal/model"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// circuitBreakerRepo implements CircuitBreakerRepo interface (defined in biz layer)
type CircuitBreakerRepo struct {
	db     *gorm.DB
	rdb    *redis.Client
	logger *log.Helper
}

// NewCircuitBreakerRepo creates a new circuit breaker repository
func NewCircuitBreakerRepo(db *gorm.DB, rdb *redis.Client, logger log.Logger) *CircuitBreakerRepo {
	return &CircuitBreakerRepo{
		db:     db,
		rdb:    rdb,
		logger: log.NewHelper(logger),
	}
}

// UpdateHealthScore updates account health score using optimistic locking with retry
func (r *CircuitBreakerRepo) UpdateHealthScore(ctx context.Context, accountID int64, newScore int) error {
	const maxRetries = 3

	for i := 0; i < maxRetries; i++ {
		// Read current version
		var account Account
		if err := r.db.WithContext(ctx).Where("id = ?", accountID).First(&account).Error; err != nil {
			return fmt.Errorf("failed to get account: %w", err)
		}

		currentVersion := account.Version

		// Update with version check (optimistic locking)
		result := r.db.WithContext(ctx).
			Model(&Account{}).
			Where("id = ? AND version = ?", accountID, currentVersion).
			Updates(map[string]interface{}{
				"health_score": newScore,
				"version":      currentVersion + 1,
				"updated_at":   time.Now(),
			})

		if result.Error != nil {
			return fmt.Errorf("failed to update health score: %w", result.Error)
		}

		// Success if row was updated
		if result.RowsAffected > 0 {
			r.logger.Debugw("health score updated with optimistic locking",
				"account_id", accountID,
				"new_score", newScore,
				"version", currentVersion+1)

			// Clear account cache
			if err := r.clearAccountCache(ctx, accountID); err != nil {
				r.logger.Warnw("failed to clear account cache", "account_id", accountID, "error", err)
			}

			return nil
		}

		// Version conflict, retry with exponential backoff
		backoff := time.Duration(i+1) * 10 * time.Millisecond
		r.logger.Debugw("version conflict, retrying",
			"account_id", accountID,
			"retry", i+1,
			"backoff", backoff)

		time.Sleep(backoff)
	}

	return fmt.Errorf("health score update failed after %d retries (version conflicts)", maxRetries)
}

// SetCircuitBroken marks account as circuit broken
func (r *CircuitBreakerRepo) SetCircuitBroken(ctx context.Context, accountID int64, brokenAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", accountID).
		Updates(map[string]interface{}{
			"is_circuit_broken": true,
			"circuit_broken_at": brokenAt,
			"updated_at":        time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to set circuit broken: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("account not found: %d", accountID)
	}

	// Set Redis TTL for circuit breaker (initial: 5 minutes)
	circuitKey := fmt.Sprintf("circuit:%d", accountID)
	if err := r.rdb.Set(ctx, circuitKey, "1", 5*time.Minute).Err(); err != nil {
		r.logger.Warnw("failed to set circuit breaker in Redis (degraded mode)",
			"account_id", accountID,
			"error", err)
		// Don't fail the operation if Redis is down
	}

	// Clear account cache
	if err := r.clearAccountCache(ctx, accountID); err != nil {
		r.logger.Warnw("failed to clear account cache", "account_id", accountID, "error", err)
	}

	return nil
}

// GetCircuitState retrieves current circuit breaker state from Redis and DB
func (r *CircuitBreakerRepo) GetCircuitState(ctx context.Context, accountID int64) (*model.CircuitState, error) {
	// Get from DB
	var account Account
	if err := r.db.WithContext(ctx).Where("id = ?", accountID).First(&account).Error; err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	state := &model.CircuitState{
		IsCircuitBroken: account.IsCircuitBroken,
		CircuitBrokenAt: account.CircuitBrokenAt,
	}

	// Check if half-open from Redis
	halfOpenKey := fmt.Sprintf("circuit:%d:half_open", accountID)
	exists, err := r.rdb.Exists(ctx, halfOpenKey).Result()
	if err != nil {
		r.logger.Warnw("failed to check half-open state (degraded mode: assume not half-open)",
			"account_id", accountID,
			"error", err)
		state.IsHalfOpen = false
	} else {
		state.IsHalfOpen = exists > 0
	}

	// Get success count from Redis
	successCount, err := r.GetSuccessCount(ctx, accountID)
	if err != nil {
		r.logger.Warnw("failed to get success count (degraded mode: default to 0)",
			"account_id", accountID,
			"error", err)
		successCount = 0
	}
	state.SuccessCount = successCount

	// Get backoff time
	backoffTime, err := r.GetBackoffTime(ctx, accountID)
	if err == nil && backoffTime != nil {
		state.BackoffRetryTime = *backoffTime
	}

	return state, nil
}

// SetHalfOpen sets half-open state marker in Redis using SETNX (atomic)
func (r *CircuitBreakerRepo) SetHalfOpen(ctx context.Context, accountID int64, ttl time.Duration) (bool, error) {
	halfOpenKey := fmt.Sprintf("circuit:%d:half_open", accountID)

	// Use SetNX for atomic set-if-not-exists
	success, err := r.rdb.SetNX(ctx, halfOpenKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to set half-open marker: %w", err)
	}

	if success {
		r.logger.Debugw("half-open marker set successfully",
			"account_id", accountID,
			"ttl", ttl)
	}

	return success, nil
}

// IncrementSuccessCount increments probe success counter and returns new count
func (r *CircuitBreakerRepo) IncrementSuccessCount(ctx context.Context, accountID int64) (int, error) {
	successKey := fmt.Sprintf("circuit:%d:success_count", accountID)

	// Increment and get new value
	newCount, err := r.rdb.Incr(ctx, successKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment success count: %w", err)
	}

	// Set TTL if this is the first increment
	if newCount == 1 {
		r.rdb.Expire(ctx, successKey, 10*time.Minute)
	}

	return int(newCount), nil
}

// GetSuccessCount gets current probe success count
func (r *CircuitBreakerRepo) GetSuccessCount(ctx context.Context, accountID int64) (int, error) {
	successKey := fmt.Sprintf("circuit:%d:success_count", accountID)

	count, err := r.rdb.Get(ctx, successKey).Int()
	if err == redis.Nil {
		return 0, nil // Key doesn't exist, return 0
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get success count: %w", err)
	}

	return count, nil
}

// ResetCircuitBreaker resets circuit breaker state (marks as healthy)
func (r *CircuitBreakerRepo) ResetCircuitBreaker(ctx context.Context, accountID int64) error {
	// Update database
	result := r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", accountID).
		Updates(map[string]interface{}{
			"is_circuit_broken": false,
			"circuit_broken_at": nil,
			"updated_at":        time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to reset circuit breaker: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("account not found: %d", accountID)
	}

	// Clear Redis keys
	circuitKey := fmt.Sprintf("circuit:%d", accountID)
	successKey := fmt.Sprintf("circuit:%d:success_count", accountID)
	halfOpenKey := fmt.Sprintf("circuit:%d:half_open", accountID)
	backoffKey := fmt.Sprintf("circuit:%d:backoff", accountID)

	keys := []string{circuitKey, successKey, halfOpenKey, backoffKey}
	if err := r.rdb.Del(ctx, keys...).Err(); err != nil {
		r.logger.Warnw("failed to delete circuit breaker keys from Redis (degraded mode)",
			"account_id", accountID,
			"error", err)
		// Don't fail the operation if Redis is down
	}

	// Clear account cache
	if err := r.clearAccountCache(ctx, accountID); err != nil {
		r.logger.Warnw("failed to clear account cache", "account_id", accountID, "error", err)
	}

	r.logger.Infow("circuit breaker reset successfully", "account_id", accountID)

	return nil
}

// SetBackoffTime sets next retry time for exponential backoff
func (r *CircuitBreakerRepo) SetBackoffTime(ctx context.Context, accountID int64, nextRetry time.Time) error {
	backoffKey := fmt.Sprintf("circuit:%d:backoff", accountID)

	// Store as Unix timestamp
	timestamp := nextRetry.Unix()
	if err := r.rdb.Set(ctx, backoffKey, timestamp, 1*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to set backoff time: %w", err)
	}

	r.logger.Debugw("backoff time set",
		"account_id", accountID,
		"next_retry", nextRetry)

	return nil
}

// GetBackoffTime gets next retry time
func (r *CircuitBreakerRepo) GetBackoffTime(ctx context.Context, accountID int64) (*time.Time, error) {
	backoffKey := fmt.Sprintf("circuit:%d:backoff", accountID)

	timestamp, err := r.rdb.Get(ctx, backoffKey).Int64()
	if err == redis.Nil {
		return nil, nil // Key doesn't exist
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get backoff time: %w", err)
	}

	t := time.Unix(timestamp, 0)
	return &t, nil
}

// GetAccount retrieves account info (implements both AccountRepo and CircuitBreakerRepo interface)
func (r *CircuitBreakerRepo) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	var account Account
	if err := r.db.WithContext(ctx).Where("id = ?", accountID).First(&account).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("account not found: %d", accountID)
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &account, nil
}

// clearAccountCache clears account cache from Redis
func (r *CircuitBreakerRepo) clearAccountCache(ctx context.Context, accountID int64) error {
	cacheKey := fmt.Sprintf("account:%d", accountID)

	if err := r.rdb.Del(ctx, cacheKey).Err(); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	r.logger.Debugw("account cache cleared", "account_id", accountID)
	return nil
}
