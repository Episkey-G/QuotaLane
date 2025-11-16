package biz

import (
	"context"
)

// RateLimitRepo defines the interface for rate limiting operations.
// Following Kratos v2 DDD architecture, interfaces are defined in biz layer.
// Implementation is in data layer (data.RateLimitRepo).
type RateLimitRepo interface {
	// RPM (Requests Per Minute) operations
	IncrementRPM(ctx context.Context, accountID int64) (int32, error)
	GetRPMCount(ctx context.Context, accountID int64) (int32, error)

	// TPM (Tokens Per Minute) operations
	IncrementTPM(ctx context.Context, accountID int64, tokens int32) (int32, error)
	GetTPMCount(ctx context.Context, accountID int64) (int32, error)

	// Concurrency control operations
	AddConcurrencyRequest(ctx context.Context, accountID int64, requestID string, timestamp int64) error
	RemoveConcurrencyRequest(ctx context.Context, accountID int64, requestID string) error
	GetConcurrencyCount(ctx context.Context, accountID int64) (int32, error)
	CleanupExpiredConcurrency(ctx context.Context, accountID int64, expiredBefore int64) error
}
