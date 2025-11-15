package biz

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRateLimitRepo is a mock implementation of RateLimitRepo for testing.
type MockRateLimitRepo struct {
	mock.Mock
}

func (m *MockRateLimitRepo) IncrementRPM(ctx context.Context, accountID int64) (int32, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockRateLimitRepo) GetRPMCount(ctx context.Context, accountID int64) (int32, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockRateLimitRepo) IncrementTPM(ctx context.Context, accountID int64, tokens int32) (int32, error) {
	args := m.Called(ctx, accountID, tokens)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockRateLimitRepo) GetTPMCount(ctx context.Context, accountID int64) (int32, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockRateLimitRepo) AddConcurrencyRequest(ctx context.Context, accountID int64, requestID string, timestamp int64) error {
	args := m.Called(ctx, accountID, requestID, timestamp)
	return args.Error(0)
}

func (m *MockRateLimitRepo) RemoveConcurrencyRequest(ctx context.Context, accountID int64, requestID string) error {
	args := m.Called(ctx, accountID, requestID)
	return args.Error(0)
}

func (m *MockRateLimitRepo) GetConcurrencyCount(ctx context.Context, accountID int64) (int32, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockRateLimitRepo) CleanupExpiredConcurrency(ctx context.Context, accountID int64, expiredBefore int64) error {
	args := m.Called(ctx, accountID, expiredBefore)
	return args.Error(0)
}

// Helper function to create a test RateLimiterUseCase
func newTestRateLimiter(repo *MockRateLimitRepo) *RateLimiterUseCase {
	logger := log.NewStdLogger(os.Stdout)
	return NewRateLimiterUseCase(repo, logger)
}

// Test CheckRPM - Normal case
func TestCheckRPM_Success(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	rpmLimit := int32(100)

	// Mock: current count is 50, within limit
	mockRepo.On("IncrementRPM", ctx, accountID).Return(int32(50), nil)

	err := uc.CheckRPM(ctx, accountID, rpmLimit)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test CheckRPM - Limit exceeded
func TestCheckRPM_LimitExceeded(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	rpmLimit := int32(100)

	// Mock: current count is 101, exceeds limit
	mockRepo.On("IncrementRPM", ctx, accountID).Return(int32(101), nil)

	err := uc.CheckRPM(ctx, accountID, rpmLimit)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RATE_LIMIT_EXCEEDED_RPM")
	mockRepo.AssertExpectations(t)
}

// Test CheckRPM - Redis error (graceful degradation)
func TestCheckRPM_RedisError(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	rpmLimit := int32(100)

	// Mock: Redis error
	mockRepo.On("IncrementRPM", ctx, accountID).Return(int32(0), errors.New("redis connection failed"))

	err := uc.CheckRPM(ctx, accountID, rpmLimit)
	// Should NOT return error (graceful degradation)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test CheckRPM - No limit configured
func TestCheckRPM_NoLimit(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	rpmLimit := int32(0) // No limit

	// Should not call Redis
	err := uc.CheckRPM(ctx, accountID, rpmLimit)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t) // No calls expected
}

// Test CheckTPM - Success
func TestCheckTPM_Success(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	tpmLimit := int32(100000)
	estimatedTokens := int32(1000)

	// Mock: current count is 50000, adding 1000 is within limit
	mockRepo.On("GetTPMCount", ctx, accountID).Return(int32(50000), nil)
	mockRepo.On("IncrementTPM", ctx, accountID, estimatedTokens).Return(int32(51000), nil)

	err := uc.CheckTPM(ctx, accountID, tpmLimit, estimatedTokens)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test CheckTPM - Limit would be exceeded
func TestCheckTPM_LimitExceeded(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	tpmLimit := int32(100000)
	estimatedTokens := int32(20000)

	// Mock: current count is 90000, adding 20000 would exceed limit
	mockRepo.On("GetTPMCount", ctx, accountID).Return(int32(90000), nil)

	err := uc.CheckTPM(ctx, accountID, tpmLimit, estimatedTokens)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RATE_LIMIT_EXCEEDED_TPM")
	mockRepo.AssertExpectations(t)
}

// Test CheckTPM - Redis error (graceful degradation)
func TestCheckTPM_RedisError(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	tpmLimit := int32(100000)
	estimatedTokens := int32(1000)

	// Mock: Redis GetTPMCount error
	mockRepo.On("GetTPMCount", ctx, accountID).Return(int32(0), errors.New("redis connection failed"))

	err := uc.CheckTPM(ctx, accountID, tpmLimit, estimatedTokens)
	// Should NOT return error (graceful degradation)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test UpdateTPM - Correction applied
func TestUpdateTPM_Correction(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	actualTokens := int32(1200)
	estimatedTokens := int32(1000)
	correction := actualTokens - estimatedTokens // 200

	// Mock: apply correction
	mockRepo.On("IncrementTPM", ctx, accountID, correction).Return(int32(1200), nil)

	err := uc.UpdateTPM(ctx, accountID, actualTokens, estimatedTokens)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test UpdateTPM - No correction needed
func TestUpdateTPM_NoCorrection(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	actualTokens := int32(1000)
	estimatedTokens := int32(1000)

	// Mock: no correction needed, Redis should not be called
	err := uc.UpdateTPM(ctx, accountID, actualTokens, estimatedTokens)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t) // No calls expected
}

// Test EstimateTokens - Normal case
func TestEstimateTokens(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	tests := []struct {
		name            string
		prompt          string
		maxOutputTokens int32
		expected        int32
	}{
		{
			name:            "Short prompt",
			prompt:          "Hello world",
			maxOutputTokens: 100,
			expected:        102, // len("Hello world") / 4 + 100 = 2 + 100
		},
		{
			name:            "Long prompt",
			prompt:          "This is a much longer prompt with many characters to test the estimation algorithm",
			maxOutputTokens: 500,
			expected:        520, // len=84, 84/4 + 500 = 21 + 500
		},
		{
			name:            "Empty prompt",
			prompt:          "",
			maxOutputTokens: 100,
			expected:        100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uc.EstimateTokens(tt.prompt, tt.maxOutputTokens)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test AcquireConcurrencySlot - Success
func TestAcquireConcurrencySlot_Success(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	requestID := "req-123"

	// Mock: add request, count is 5 (within limit of 10)
	mockRepo.On("AddConcurrencyRequest", ctx, accountID, requestID, mock.AnythingOfType("int64")).Return(nil)
	mockRepo.On("GetConcurrencyCount", ctx, accountID).Return(int32(5), nil)

	err := uc.AcquireConcurrencySlot(ctx, accountID, requestID)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test AcquireConcurrencySlot - Limit exceeded
func TestAcquireConcurrencySlot_LimitExceeded(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	requestID := "req-123"

	// Mock: add request, count is 11 (exceeds limit of 10)
	mockRepo.On("AddConcurrencyRequest", ctx, accountID, requestID, mock.AnythingOfType("int64")).Return(nil)
	mockRepo.On("GetConcurrencyCount", ctx, accountID).Return(int32(11), nil)
	mockRepo.On("RemoveConcurrencyRequest", ctx, accountID, requestID).Return(nil)

	err := uc.AcquireConcurrencySlot(ctx, accountID, requestID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RATE_LIMIT_EXCEEDED_Concurrency")
	mockRepo.AssertExpectations(t)
}

// Test AcquireConcurrencySlot - Redis error (graceful degradation)
func TestAcquireConcurrencySlot_RedisError(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	requestID := "req-123"

	// Mock: AddConcurrencyRequest error
	mockRepo.On("AddConcurrencyRequest", ctx, accountID, requestID, mock.AnythingOfType("int64")).
		Return(errors.New("redis connection failed"))

	err := uc.AcquireConcurrencySlot(ctx, accountID, requestID)
	// Should NOT return error (graceful degradation)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test ReleaseConcurrencySlot - Success
func TestReleaseConcurrencySlot_Success(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	requestID := "req-123"

	mockRepo.On("RemoveConcurrencyRequest", ctx, accountID, requestID).Return(nil)

	err := uc.ReleaseConcurrencySlot(ctx, accountID, requestID)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test ReleaseConcurrencySlot - Redis error (best-effort)
func TestReleaseConcurrencySlot_RedisError(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	requestID := "req-123"

	mockRepo.On("RemoveConcurrencyRequest", ctx, accountID, requestID).
		Return(errors.New("redis connection failed"))

	err := uc.ReleaseConcurrencySlot(ctx, accountID, requestID)
	// Should NOT return error (best-effort cleanup)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test CleanupExpiredConcurrency - Success
func TestCleanupExpiredConcurrency_Success(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)

	// Mock: cleanup expired requests
	mockRepo.On("CleanupExpiredConcurrency", ctx, accountID, mock.AnythingOfType("int64")).Return(nil)

	err := uc.CleanupExpiredConcurrency(ctx, accountID)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Test CleanupExpiredConcurrencyForAllAccounts - Success
func TestCleanupExpiredConcurrencyForAllAccounts_Success(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountIDs := []int64{1, 2, 3}

	// Mock: cleanup for each account
	for _, id := range accountIDs {
		mockRepo.On("CleanupExpiredConcurrency", ctx, id, mock.AnythingOfType("int64")).Return(nil)
	}

	cleanedCount, err := uc.CleanupExpiredConcurrencyForAllAccounts(ctx, accountIDs)
	assert.NoError(t, err)
	assert.Equal(t, 3, cleanedCount)
	mockRepo.AssertExpectations(t)
}

// Test CleanupExpiredConcurrencyForAllAccounts - Some failures
func TestCleanupExpiredConcurrencyForAllAccounts_PartialFailure(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountIDs := []int64{1, 2, 3}

	// Mock: cleanup fails for account 2
	mockRepo.On("CleanupExpiredConcurrency", ctx, int64(1), mock.AnythingOfType("int64")).Return(nil)
	mockRepo.On("CleanupExpiredConcurrency", ctx, int64(2), mock.AnythingOfType("int64")).
		Return(errors.New("cleanup failed"))
	mockRepo.On("CleanupExpiredConcurrency", ctx, int64(3), mock.AnythingOfType("int64")).Return(nil)

	cleanedCount, err := uc.CleanupExpiredConcurrencyForAllAccounts(ctx, accountIDs)
	assert.NoError(t, err)
	assert.Equal(t, 2, cleanedCount) // Only 2 accounts cleaned successfully
	mockRepo.AssertExpectations(t)
}

// Test fixed window edge case - rapid requests at window boundary
func TestCheckRPM_WindowBoundary(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	ctx := context.Background()
	accountID := int64(123)
	rpmLimit := int32(100)

	// Simulate rapid requests at window boundary
	mockRepo.On("IncrementRPM", ctx, accountID).Return(int32(99), nil).Once()
	mockRepo.On("IncrementRPM", ctx, accountID).Return(int32(100), nil).Once()
	mockRepo.On("IncrementRPM", ctx, accountID).Return(int32(101), nil).Once()

	// First request: count 99 - OK
	err := uc.CheckRPM(ctx, accountID, rpmLimit)
	assert.NoError(t, err)

	// Second request: count 100 - OK (exactly at limit)
	err = uc.CheckRPM(ctx, accountID, rpmLimit)
	assert.NoError(t, err)

	// Third request: count 101 - EXCEEDED
	err = uc.CheckRPM(ctx, accountID, rpmLimit)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RATE_LIMIT_EXCEEDED_RPM")

	mockRepo.AssertExpectations(t)
}

// Test token estimation accuracy (compared to expected ranges)
func TestEstimateTokens_Accuracy(t *testing.T) {
	mockRepo := new(MockRateLimitRepo)
	uc := newTestRateLimiter(mockRepo)

	// Test with realistic prompts
	testCases := []struct {
		name            string
		prompt          string
		maxOutputTokens int32
		expectedMin     int32
		expectedMax     int32
	}{
		{
			name:            "Short technical prompt",
			prompt:          "Explain how rate limiting works in Go",
			maxOutputTokens: 1000,
			expectedMin:     1008, // ~8 tokens from prompt
			expectedMax:     1012,
		},
		{
			name:            "Medium code snippet",
			prompt:          "func main() {\n  fmt.Println(\"Hello, World!\")\n}\n",
			maxOutputTokens: 500,
			expectedMin:     510,
			expectedMax:     520,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			estimated := uc.EstimateTokens(tc.prompt, tc.maxOutputTokens)
			assert.GreaterOrEqual(t, estimated, tc.expectedMin)
			assert.LessOrEqual(t, estimated, tc.expectedMax)
		})
	}
}
