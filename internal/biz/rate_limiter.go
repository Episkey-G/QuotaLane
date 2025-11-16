package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// RateLimiterUseCase implements rate limiting business logic for accounts.
// It provides RPM (Requests Per Minute), TPM (Tokens Per Minute) rate limiting,
// and concurrency control using Redis-based counters and sorted sets.
type RateLimiterUseCase struct {
	repo   RateLimitRepo
	logger *log.Helper
}

// NewRateLimiterUseCase creates a new rate limiter use case.
func NewRateLimiterUseCase(repo RateLimitRepo, logger log.Logger) *RateLimiterUseCase {
	return &RateLimiterUseCase{
		repo:   repo,
		logger: log.NewHelper(logger),
	}
}

// RateLimitExceededError represents a rate limit exceeded error with retry information.
type RateLimitExceededError struct {
	LimitType    string // "RPM", "TPM", or "Concurrency"
	CurrentCount int32  // Current count
	Limit        int32  // Configured limit
	RetryAfter   int64  // Seconds until retry is allowed
}

// Error implements the error interface.
func (e *RateLimitExceededError) Error() string {
	return fmt.Sprintf("rate limit exceeded: %s current=%d limit=%d retry_after=%ds",
		e.LimitType, e.CurrentCount, e.Limit, e.RetryAfter)
}

// newRateLimitExceededError creates a gRPC ResourceExhausted error from RateLimitExceededError.
func newRateLimitExceededError(limitType string, current, limit int32, retryAfter int64) error {
	return errors.New(
		429, // HTTP 429 Too Many Requests
		fmt.Sprintf("RATE_LIMIT_EXCEEDED_%s", limitType),
		fmt.Sprintf("rate limit exceeded: %s current=%d limit=%d retry_after=%ds",
			limitType, current, limit, retryAfter),
	)
}

// CheckRPM checks if the account has exceeded its RPM (Requests Per Minute) limit.
// It uses Redis INCR with fixed window rate limiting algorithm.
// Returns error if limit is exceeded, nil otherwise.
// Redis degradation: on Redis failure, logs warning and allows request (graceful degradation).
func (uc *RateLimiterUseCase) CheckRPM(ctx context.Context, accountID int64, rpmLimit int32) error {
	if rpmLimit <= 0 {
		// No limit configured, allow request
		return nil
	}

	// Increment RPM counter
	count, err := uc.repo.IncrementRPM(ctx, accountID)
	if err != nil {
		// Redis failure: log warning and allow request (graceful degradation)
		uc.logger.Warnf("Redis RPM check failed for account %d: %v (request allowed)", accountID, err)
		return nil
	}

	// Check if limit exceeded
	if count > rpmLimit {
		uc.logger.Warnw("RPM limit exceeded",
			"account_id", accountID,
			"current", count,
			"limit", rpmLimit)
		return newRateLimitExceededError("RPM", count, rpmLimit, 60)
	}

	return nil
}

// CheckTPM checks if the account has enough TPM (Tokens Per Minute) quota for the estimated tokens.
// It uses Redis INCRBY with token estimation before request.
// Returns error if limit is exceeded, nil otherwise.
// Redis degradation: on Redis failure, logs warning and allows request.
func (uc *RateLimiterUseCase) CheckTPM(ctx context.Context, accountID int64, tpmLimit int32, estimatedTokens int32) error {
	if tpmLimit <= 0 {
		// No limit configured, allow request
		return nil
	}

	if estimatedTokens <= 0 {
		// Invalid estimation, skip check
		uc.logger.Warnf("Invalid token estimation for account %d: %d", accountID, estimatedTokens)
		return nil
	}

	// Get current TPM count
	currentCount, err := uc.repo.GetTPMCount(ctx, accountID)
	if err != nil {
		// Redis failure: log warning and allow request
		uc.logger.Warnf("Redis TPM get failed for account %d: %v (request allowed)", accountID, err)
		return nil
	}

	// Check if adding estimated tokens would exceed limit
	if currentCount+estimatedTokens > tpmLimit {
		uc.logger.Warnw("TPM limit would be exceeded",
			"account_id", accountID,
			"current", currentCount,
			"estimated", estimatedTokens,
			"limit", tpmLimit)
		return newRateLimitExceededError("TPM", currentCount, tpmLimit, 60)
	}

	// Pre-increment TPM counter with estimated tokens
	newCount, err := uc.repo.IncrementTPM(ctx, accountID, estimatedTokens)
	if err != nil {
		// Redis failure: log warning and allow request
		uc.logger.Warnf("Redis TPM increment failed for account %d: %v (request allowed)", accountID, err)
		return nil
	}

	uc.logger.Debugw("TPM check passed",
		"account_id", accountID,
		"current", newCount,
		"estimated", estimatedTokens,
		"limit", tpmLimit)

	return nil
}

// UpdateTPM updates the TPM counter with the actual token usage after request completion.
// It calculates the difference between actual and estimated tokens and adjusts the counter.
// This correction ensures accurate rate limiting based on real API responses.
func (uc *RateLimiterUseCase) UpdateTPM(ctx context.Context, accountID int64, actualTokens int32, estimatedTokens int32) error {
	if actualTokens <= 0 {
		uc.logger.Warnf("Invalid actual tokens for account %d: %d", accountID, actualTokens)
		return nil
	}

	// Calculate correction: actual - estimated
	correction := actualTokens - estimatedTokens

	if correction == 0 {
		// Estimation was accurate, no correction needed
		return nil
	}

	// Apply correction to TPM counter
	_, err := uc.repo.IncrementTPM(ctx, accountID, correction)
	if err != nil {
		// Redis failure: log warning but don't return error (correction is best-effort)
		uc.logger.Warnf("Redis TPM correction failed for account %d: %v (actual=%d estimated=%d)",
			accountID, err, actualTokens, estimatedTokens)
		return nil
	}

	uc.logger.Debugw("TPM corrected",
		"account_id", accountID,
		"actual", actualTokens,
		"estimated", estimatedTokens,
		"correction", correction)

	return nil
}

// EstimateTokens estimates the number of tokens for a request.
// Algorithm: tokens ≈ len(prompt) / 4 + max_output_tokens
// This is a rough estimation for MVP; more accurate methods (e.g., tiktoken) can be added later.
func (uc *RateLimiterUseCase) EstimateTokens(prompt string, maxOutputTokens int32) int32 {
	// Rough estimation: 1 token ≈ 4 characters for English text
	// Prevent overflow: cap prompt length calculation
	promptLen := len(prompt) / 4
	if promptLen > 2147483647 {
		promptLen = 2147483647
	}
	promptTokens := int32(promptLen) // #nosec G115 -- overflow is handled above

	// Add max output tokens
	estimatedTotal := promptTokens + maxOutputTokens

	// Ensure minimum 1 token
	if estimatedTotal <= 0 {
		estimatedTotal = 1
	}

	return estimatedTotal
}

// AcquireConcurrencySlot attempts to acquire a concurrency slot for the request.
// It uses Redis Sorted Set (ZADD + ZCARD) to track concurrent requests.
// Maximum concurrency is hardcoded to 10 for MVP.
// Returns error if concurrency limit is exceeded.
func (uc *RateLimiterUseCase) AcquireConcurrencySlot(ctx context.Context, accountID int64, requestID string) error {
	const maxConcurrency = 10

	// Add request to concurrency set with current timestamp
	timestamp := time.Now().Unix()
	if err := uc.repo.AddConcurrencyRequest(ctx, accountID, requestID, timestamp); err != nil {
		// Redis failure: log warning and allow request
		uc.logger.Warnf("Redis concurrency add failed for account %d: %v (request allowed)", accountID, err)
		return nil
	}

	// Check current concurrency count
	count, err := uc.repo.GetConcurrencyCount(ctx, accountID)
	if err != nil {
		// Redis failure: log warning, remove added request, and allow
		uc.logger.Warnf("Redis concurrency count failed for account %d: %v (request allowed)", accountID, err)
		// Best-effort cleanup
		_ = uc.repo.RemoveConcurrencyRequest(ctx, accountID, requestID)
		return nil
	}

	// Check if concurrency limit exceeded
	if count > maxConcurrency {
		// Remove the request we just added
		_ = uc.repo.RemoveConcurrencyRequest(ctx, accountID, requestID)

		uc.logger.Warnw("Concurrency limit exceeded",
			"account_id", accountID,
			"current", count,
			"limit", maxConcurrency)
		return newRateLimitExceededError("Concurrency", count, maxConcurrency, 5)
	}

	uc.logger.Debugw("Concurrency slot acquired",
		"account_id", accountID,
		"request_id", requestID,
		"current", count,
		"limit", maxConcurrency)

	return nil
}

// ReleaseConcurrencySlot releases a concurrency slot after request completion.
// This should be called with defer to ensure cleanup even on errors.
func (uc *RateLimiterUseCase) ReleaseConcurrencySlot(ctx context.Context, accountID int64, requestID string) error {
	if err := uc.repo.RemoveConcurrencyRequest(ctx, accountID, requestID); err != nil {
		// Log error but don't return it (cleanup is best-effort)
		uc.logger.Warnf("Failed to release concurrency slot for account %d request %s: %v",
			accountID, requestID, err)
		return nil
	}

	uc.logger.Debugw("Concurrency slot released",
		"account_id", accountID,
		"request_id", requestID)

	return nil
}

// CleanupExpiredConcurrency cleans up expired concurrency requests for an account.
// Requests older than 10 minutes are considered expired.
// This should be called periodically by a cron job.
func (uc *RateLimiterUseCase) CleanupExpiredConcurrency(ctx context.Context, accountID int64) error {
	const expiryMinutes = 10

	// Calculate cutoff timestamp (10 minutes ago)
	expiredBefore := time.Now().Add(-expiryMinutes * time.Minute).Unix()

	if err := uc.repo.CleanupExpiredConcurrency(ctx, accountID, expiredBefore); err != nil {
		uc.logger.Warnf("Failed to cleanup expired concurrency for account %d: %v", accountID, err)
		return err
	}

	return nil
}

// CleanupExpiredConcurrencyForAllAccounts cleans up expired concurrency for all accounts.
// This is called by the cron job to periodically clean up stale concurrency slots.
func (uc *RateLimiterUseCase) CleanupExpiredConcurrencyForAllAccounts(ctx context.Context, accountIDs []int64) (int, error) {
	cleanedCount := 0

	for _, accountID := range accountIDs {
		if err := uc.CleanupExpiredConcurrency(ctx, accountID); err != nil {
			// Log error but continue with other accounts
			uc.logger.Warnf("Failed to cleanup account %d: %v", accountID, err)
			continue
		}
		cleanedCount++
	}

	uc.logger.Infow("Concurrency cleanup completed",
		"total_accounts", len(accountIDs),
		"cleaned", cleanedCount)

	return cleanedCount, nil
}
