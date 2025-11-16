package service

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/biz"
	"QuotaLane/internal/data"
	"QuotaLane/pkg/crypto"
	"QuotaLane/pkg/oauth"
	"QuotaLane/pkg/openai"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAccountRepo is a mock implementation of data.AccountRepo for testing.
type MockAccountRepo struct {
	mock.Mock
}

func (m *MockAccountRepo) CreateAccount(ctx context.Context, account *data.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepo) GetAccount(ctx context.Context, id int64) (*data.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*data.Account), args.Error(1)
}

func (m *MockAccountRepo) ListAccounts(ctx context.Context, filter *data.AccountFilter) ([]*data.Account, int32, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int32), args.Error(2)
	}
	return args.Get(0).([]*data.Account), args.Get(1).(int32), args.Error(2)
}

func (m *MockAccountRepo) UpdateAccount(ctx context.Context, account *data.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepo) DeleteAccount(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAccountRepo) ListExpiringAccounts(ctx context.Context, expiryThreshold time.Time) ([]*data.Account, error) {
	args := m.Called(ctx, expiryThreshold)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*data.Account), args.Error(1)
}

func (m *MockAccountRepo) UpdateOAuthData(ctx context.Context, accountID int64, oauthData string, expiresAt time.Time) error {
	args := m.Called(ctx, accountID, oauthData, expiresAt)
	return args.Error(0)
}

func (m *MockAccountRepo) UpdateHealthScore(ctx context.Context, accountID int64, score int) error {
	args := m.Called(ctx, accountID, score)
	return args.Error(0)
}

func (m *MockAccountRepo) UpdateAccountStatus(ctx context.Context, accountID int64, status data.AccountStatus) error {
	args := m.Called(ctx, accountID, status)
	return args.Error(0)
}

func (m *MockAccountRepo) ListAccountsByProvider(ctx context.Context, provider data.AccountProvider, status data.AccountStatus) ([]*data.Account, error) {
	args := m.Called(ctx, provider, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*data.Account), args.Error(1)
}

func (m *MockAccountRepo) ListCodexCLIAccountsNeedingRefresh(ctx context.Context) ([]*data.Account, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*data.Account), args.Error(1)
}

// MockOAuthService is a mock implementation of oauth.OAuthService for testing.
type MockOAuthService struct {
	mock.Mock
}

func (m *MockOAuthService) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*oauth.TokenResponse, error) {
	args := m.Called(ctx, refreshToken, proxyURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth.TokenResponse), args.Error(1)
}

// setupTestService creates a test AccountService with mock repository.
func setupTestService(t *testing.T) (*AccountService, *MockAccountRepo) {
	mockRepo := new(MockAccountRepo)
	mockOAuth := new(MockOAuthService)
	logger := log.DefaultLogger

	// Create AES crypto with test key
	testKey := []byte("12345678901234567890123456789012")
	cryptoSvc, err := crypto.NewAESCrypto(testKey)
	assert.NoError(t, err)

	// Create test Redis client (use nil for unit tests, or miniredis for integration tests)
	// For unit tests, we don't actually need a real Redis connection
	var rdb *redis.Client = nil

	// Create mock OpenAI service (nil for unit tests)
	var mockOpenAI openai.OpenAIService = nil

	// Create mock OAuth manager (nil for unit tests)
	var mockOAuthManager *oauth.OAuthManager = nil

	// Create mock CircuitBreakerUsecase (nil for unit tests - not used in these service layer tests)
	var mockCircuitBreaker *biz.CircuitBreakerUsecase = nil

	// Create real usecase with mock dependencies
	uc := biz.NewAccountUsecase(mockRepo, cryptoSvc, mockOAuth, mockOpenAI, mockOAuthManager, mockCircuitBreaker, rdb, logger)

	// Create service with real usecase
	svc := NewAccountService(uc, logger)
	return svc, mockRepo
}

// TestCreateAccount tests CreateAccount RPC method.
func TestCreateAccount(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	req := &v1.CreateAccountRequest{
		Name:     "Test Account",
		Provider: v1.AccountProvider_CLAUDE_CONSOLE,
		RpmLimit: 50,
	}

	mockRepo.On("CreateAccount", ctx, mock.AnythingOfType("*data.Account")).Return(nil)

	resp, err := svc.CreateAccount(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Account)
	assert.Equal(t, "Test Account", resp.Account.Name)
	mockRepo.AssertExpectations(t)
}

// TestCreateAccount_Error tests CreateAccount error handling.
func TestCreateAccount_Error(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	req := &v1.CreateAccountRequest{
		Name:     "Test Account",
		Provider: v1.AccountProvider_CLAUDE_CONSOLE,
	}

	mockRepo.On("CreateAccount", ctx, mock.AnythingOfType("*data.Account")).
		Return(errors.New("database error"))

	resp, err := svc.CreateAccount(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	mockRepo.AssertExpectations(t)
}

// TestListAccounts tests ListAccounts RPC method.
func TestListAccounts(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	req := &v1.ListAccountsRequest{
		Page:     1,
		PageSize: 10,
	}

	accounts := []*data.Account{
		{
			ID:          1,
			Name:        "Account 1",
			Provider:    data.ProviderClaudeConsole,
			HealthScore: 100,
			Status:      data.StatusActive,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	mockRepo.On("ListAccounts", ctx, mock.AnythingOfType("*data.AccountFilter")).
		Return(accounts, int32(1), nil)

	resp, err := svc.ListAccounts(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(1), resp.Total)
	assert.Len(t, resp.Accounts, 1)
	mockRepo.AssertExpectations(t)
}

// TestListAccounts_Error tests ListAccounts error handling.
func TestListAccounts_Error(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	req := &v1.ListAccountsRequest{
		Page:     1,
		PageSize: 10,
	}

	mockRepo.On("ListAccounts", ctx, mock.AnythingOfType("*data.AccountFilter")).
		Return(nil, int32(0), errors.New("database error"))

	resp, err := svc.ListAccounts(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	mockRepo.AssertExpectations(t)
}

// TestGetAccount tests GetAccount RPC method.
func TestGetAccount(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	req := &v1.GetAccountRequest{
		Id: 1,
	}

	account := &data.Account{
		ID:          1,
		Name:        "Test Account",
		Provider:    data.ProviderClaudeConsole,
		HealthScore: 100,
		Status:      data.StatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockRepo.On("GetAccount", ctx, int64(1)).Return(account, nil)

	resp, err := svc.GetAccount(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Account)
	assert.Equal(t, int64(1), resp.Account.Id)
	mockRepo.AssertExpectations(t)
}

// TestGetAccount_Error tests GetAccount error handling.
func TestGetAccount_Error(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	req := &v1.GetAccountRequest{
		Id: 999,
	}

	mockRepo.On("GetAccount", ctx, int64(999)).
		Return(nil, errors.New("account not found"))

	resp, err := svc.GetAccount(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	mockRepo.AssertExpectations(t)
}

// TestUpdateAccount tests UpdateAccount RPC method.
func TestUpdateAccount(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	newName := "Updated Account"
	req := &v1.UpdateAccountRequest{
		Id:   1,
		Name: &newName,
	}

	existingAccount := &data.Account{
		ID:          1,
		Name:        "Old Name",
		Provider:    data.ProviderClaudeConsole,
		HealthScore: 100,
		Status:      data.StatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockRepo.On("GetAccount", ctx, int64(1)).Return(existingAccount, nil)
	mockRepo.On("UpdateAccount", ctx, mock.AnythingOfType("*data.Account")).Return(nil)

	resp, err := svc.UpdateAccount(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Account)
	assert.Equal(t, "Updated Account", resp.Account.Name)
	mockRepo.AssertExpectations(t)
}

// TestUpdateAccount_Error tests UpdateAccount error handling.
func TestUpdateAccount_Error(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	newName := "Updated Account"
	req := &v1.UpdateAccountRequest{
		Id:   999,
		Name: &newName,
	}

	mockRepo.On("GetAccount", ctx, int64(999)).
		Return(nil, errors.New("account not found"))

	resp, err := svc.UpdateAccount(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	mockRepo.AssertExpectations(t)
}

// TestDeleteAccount tests DeleteAccount RPC method.
func TestDeleteAccount(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	req := &v1.DeleteAccountRequest{
		Id: 1,
	}

	mockRepo.On("DeleteAccount", ctx, int64(1)).Return(nil)

	resp, err := svc.DeleteAccount(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "Account deleted successfully", resp.Message)
	mockRepo.AssertExpectations(t)
}

// TestDeleteAccount_Error tests DeleteAccount error handling.
func TestDeleteAccount_Error(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	req := &v1.DeleteAccountRequest{
		Id: 999,
	}

	mockRepo.On("DeleteAccount", ctx, int64(999)).
		Return(errors.New("account not found"))

	resp, err := svc.DeleteAccount(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	mockRepo.AssertExpectations(t)
}

// TestRefreshToken tests RefreshToken RPC.
// This test expects failure because we're using nil Redis client in setupTestService.
// Full refresh logic is tested in integration tests (internal/biz/account_refresh_integration_test.go).
func TestRefreshToken(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	req := &v1.RefreshTokenRequest{
		Id: 1,
	}

	// Mock GetAccount call (will fail early due to nil Redis)
	mockRepo.On("GetAccount", ctx, int64(1)).Return(&data.Account{
		ID:                 1,
		Name:               "Test Account",
		Provider:           data.ProviderClaudeConsole,
		OAuthDataEncrypted: "encrypted_oauth_data",
		HealthScore:        100,
		Status:             data.StatusActive,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}, nil)

	resp, err := svc.RefreshToken(ctx, req)

	// Should fail because Redis client is nil (expected in unit tests)
	// Real refresh logic is tested in integration tests
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Success)
	mockRepo.AssertExpectations(t)
}

// TestTestAccount tests TestAccount RPC method with OpenAI Responses account.
func TestTestAccount(t *testing.T) {
	svc, mockRepo := setupTestService(t)
	ctx := context.Background()

	// Setup test account (OpenAI Responses type)
	testAccount := &data.Account{
		ID:              1,
		Name:            "Test OpenAI Account",
		Provider:        data.ProviderOpenAIResponses,
		Status:          data.StatusActive,
		HealthScore:     100,
		APIKeyEncrypted: "encrypted_key",
		BaseAPI:         "https://api.openai.com",
		IsCircuitBroken: false,
	}

	// Mock GetAccount call
	mockRepo.On("GetAccount", ctx, int64(1)).Return(testAccount, nil)

	req := &v1.TestAccountRequest{
		Id: 1,
	}

	// This will fail because we don't have real OpenAI service or network
	// But it should return a proper response structure
	resp, err := svc.TestAccount(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// The test will fail validation (no real API), so success should be false
	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Message)
	mockRepo.AssertExpectations(t)
}
