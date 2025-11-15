package data

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRedis creates a miniredis instance for testing
func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return rdb, mr
}

// Test IncrementRPM - First increment
func TestIncrementRPM_FirstIncrement(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)

	count, err := repo.IncrementRPM(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), count)

	// Verify TTL is set
	key := getRateLimitKey(accountID, "rpm")
	ttl := rdb.TTL(ctx, key).Val()
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, 60*time.Second)
}

// Test IncrementRPM - Subsequent increments
func TestIncrementRPM_SubsequentIncrements(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)

	// First increment
	count1, err := repo.IncrementRPM(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), count1)

	// Second increment
	count2, err := repo.IncrementRPM(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), count2)

	// Third increment
	count3, err := repo.IncrementRPM(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), count3)
}

// Test GetRPMCount - Existing key
func TestGetRPMCount_Exists(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)

	// Set initial value
	_, err := repo.IncrementRPM(ctx, accountID)
	require.NoError(t, err)

	// Get count
	count, err := repo.GetRPMCount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), count)
}

// Test GetRPMCount - Non-existent key
func TestGetRPMCount_NotExists(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(999)

	// Get count for non-existent key
	count, err := repo.GetRPMCount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), count)
}

// Test IncrementTPM - First increment
func TestIncrementTPM_FirstIncrement(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)
	tokens := int32(1000)

	count, err := repo.IncrementTPM(ctx, accountID, tokens)
	assert.NoError(t, err)
	assert.Equal(t, int32(1000), count)

	// Verify TTL is set
	key := getRateLimitKey(accountID, "tpm")
	ttl := rdb.TTL(ctx, key).Val()
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, 60*time.Second)
}

// Test IncrementTPM - Multiple increments
func TestIncrementTPM_MultipleIncrements(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)

	// First increment: +1000
	count1, err := repo.IncrementTPM(ctx, accountID, 1000)
	assert.NoError(t, err)
	assert.Equal(t, int32(1000), count1)

	// Second increment: +500
	count2, err := repo.IncrementTPM(ctx, accountID, 500)
	assert.NoError(t, err)
	assert.Equal(t, int32(1500), count2)

	// Third increment: +200
	count3, err := repo.IncrementTPM(ctx, accountID, 200)
	assert.NoError(t, err)
	assert.Equal(t, int32(1700), count3)
}

// Test IncrementTPM - Negative correction
func TestIncrementTPM_NegativeCorrection(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)

	// First: add estimated 1000 tokens
	_, err := repo.IncrementTPM(ctx, accountID, 1000)
	require.NoError(t, err)

	// Correction: actual was 800, so subtract 200
	count, err := repo.IncrementTPM(ctx, accountID, -200)
	assert.NoError(t, err)
	assert.Equal(t, int32(800), count)
}

// Test GetTPMCount
func TestGetTPMCount(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)

	// Initially zero
	count, err := repo.GetTPMCount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), count)

	// After increment
	_, err = repo.IncrementTPM(ctx, accountID, 5000)
	require.NoError(t, err)

	count, err = repo.GetTPMCount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(5000), count)
}

// Test AddConcurrencyRequest
func TestAddConcurrencyRequest(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)
	requestID := "req-123"
	timestamp := time.Now().Unix()

	err := repo.AddConcurrencyRequest(ctx, accountID, requestID, timestamp)
	assert.NoError(t, err)

	// Verify request was added to sorted set
	key := getConcurrencyKey(accountID)
	members := rdb.ZRange(ctx, key, 0, -1).Val()
	assert.Contains(t, members, requestID)
}

// Test RemoveConcurrencyRequest
func TestRemoveConcurrencyRequest(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)
	requestID := "req-123"
	timestamp := time.Now().Unix()

	// Add request first
	err := repo.AddConcurrencyRequest(ctx, accountID, requestID, timestamp)
	require.NoError(t, err)

	// Remove request
	err = repo.RemoveConcurrencyRequest(ctx, accountID, requestID)
	assert.NoError(t, err)

	// Verify request was removed
	key := getConcurrencyKey(accountID)
	members := rdb.ZRange(ctx, key, 0, -1).Val()
	assert.NotContains(t, members, requestID)
}

// Test GetConcurrencyCount
func TestGetConcurrencyCount(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)
	timestamp := time.Now().Unix()

	// Initially zero
	count, err := repo.GetConcurrencyCount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), count)

	// Add 3 requests
	repo.AddConcurrencyRequest(ctx, accountID, "req-1", timestamp)
	repo.AddConcurrencyRequest(ctx, accountID, "req-2", timestamp)
	repo.AddConcurrencyRequest(ctx, accountID, "req-3", timestamp)

	count, err = repo.GetConcurrencyCount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), count)
}

// Test CleanupExpiredConcurrency
func TestCleanupExpiredConcurrency(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)

	now := time.Now().Unix()
	// Add requests: some old, some recent
	repo.AddConcurrencyRequest(ctx, accountID, "req-old-1", now-900)  // 15 min ago (expired)
	repo.AddConcurrencyRequest(ctx, accountID, "req-old-2", now-700)  // 11.7 min ago (expired)
	repo.AddConcurrencyRequest(ctx, accountID, "req-recent", now-300) // 5 min ago (active)

	// Cleanup requests older than 10 minutes
	expiredBefore := now - 600 // 10 minutes ago
	err := repo.CleanupExpiredConcurrency(ctx, accountID, expiredBefore)
	assert.NoError(t, err)

	// Verify only recent request remains
	key := getConcurrencyKey(accountID)
	members := rdb.ZRange(ctx, key, 0, -1).Val()
	assert.Len(t, members, 1)
	assert.Contains(t, members, "req-recent")
}

// Test Redis Key generation
func TestGetRateLimitKey(t *testing.T) {
	tests := []struct {
		accountID int64
		limitType string
		expected  string
	}{
		{123, "rpm", "rate:123:rpm"},
		{456, "tpm", "rate:456:tpm"},
		{789, "rpm", "rate:789:rpm"},
	}

	for _, tt := range tests {
		result := getRateLimitKey(tt.accountID, tt.limitType)
		assert.Equal(t, tt.expected, result)
	}
}

// Test Concurrency Key generation
func TestGetConcurrencyKey(t *testing.T) {
	tests := []struct {
		accountID int64
		expected  string
	}{
		{123, "concurrency:123"},
		{456, "concurrency:456"},
		{789, "concurrency:789"},
	}

	for _, tt := range tests {
		result := getConcurrencyKey(tt.accountID)
		assert.Equal(t, tt.expected, result)
	}
}

// Test concurrent RPM increments (race condition test)
func TestIncrementRPM_Concurrent(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)
	goroutines := 100

	// Launch 100 concurrent increments
	done := make(chan bool, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := repo.IncrementRPM(ctx, accountID)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Verify final count is exactly 100
	count, err := repo.GetRPMCount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(goroutines), count)
}

// Test TPM pipeline performance (simulating Redis pipeline usage)
func TestIncrementTPM_Performance(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	defer rdb.Close()

	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(rdb, logger)

	ctx := context.Background()
	accountID := int64(123)

	// Benchmark 100 sequential increments
	start := time.Now()
	for i := 0; i < 100; i++ {
		_, err := repo.IncrementTPM(ctx, accountID, 10)
		require.NoError(t, err)
	}
	duration := time.Since(start)

	// Verify correctness
	count, err := repo.GetTPMCount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, int32(1000), count)

	// Performance expectation: < 100ms for 100 ops with miniredis
	// This is a loose check since miniredis is in-memory
	assert.Less(t, duration, 100*time.Millisecond, "TPM increments should be fast")
}

// Test nil Redis client handling
func TestRateLimitRepo_NilRedis(t *testing.T) {
	logger := log.NewStdLogger(os.Stdout)
	repo := NewRateLimitRepo(nil, logger)

	ctx := context.Background()
	accountID := int64(123)

	// All operations should return errors with nil Redis client
	_, err := repo.IncrementRPM(ctx, accountID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")

	_, err = repo.GetRPMCount(ctx, accountID)
	assert.Error(t, err)

	_, err = repo.IncrementTPM(ctx, accountID, 100)
	assert.Error(t, err)

	_, err = repo.GetTPMCount(ctx, accountID)
	assert.Error(t, err)

	err = repo.AddConcurrencyRequest(ctx, accountID, "req-1", time.Now().Unix())
	assert.Error(t, err)

	err = repo.RemoveConcurrencyRequest(ctx, accountID, "req-1")
	assert.Error(t, err)

	_, err = repo.GetConcurrencyCount(ctx, accountID)
	assert.Error(t, err)

	err = repo.CleanupExpiredConcurrency(ctx, accountID, time.Now().Unix())
	assert.Error(t, err)
}
