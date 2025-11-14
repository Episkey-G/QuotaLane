package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/data"
	"QuotaLane/pkg/crypto"
	"QuotaLane/pkg/oauth"

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

func (m *MockAccountRepo) UpdateOAuthData(ctx context.Context, accountID int64, encryptedData string, expiresAt time.Time) error {
	args := m.Called(ctx, accountID, encryptedData, expiresAt)
	return args.Error(0)
}

func (m *MockAccountRepo) UpdateHealthScore(ctx context.Context, accountID int64, score int32) error {
	args := m.Called(ctx, accountID, score)
	return args.Error(0)
}

func (m *MockAccountRepo) UpdateAccountStatus(ctx context.Context, accountID int64, status data.AccountStatus) error {
	args := m.Called(ctx, accountID, status)
	return args.Error(0)
}

// setupTestUsecase creates a test AccountUsecase with mock dependencies.
func setupTestUsecase(t *testing.T) (*AccountUsecase, *MockAccountRepo, *crypto.AESCrypto) {
	mockRepo := new(MockAccountRepo)
	logger := log.DefaultLogger

	// Create AES crypto with test key (32 bytes)
	testKey := []byte("12345678901234567890123456789012")
	cryptoSvc, err := crypto.NewAESCrypto(testKey)
	assert.NoError(t, err)

	// Create mock OAuth service (nil for unit tests)
	var oauthSvc oauth.OAuthService = nil

	// Create mock Redis client (nil for unit tests)
	var rdb *redis.Client = nil

	uc := NewAccountUsecase(mockRepo, cryptoSvc, oauthSvc, rdb, logger)
	return uc, mockRepo, cryptoSvc
}

// TestCreateAccount_Success tests successful account creation.
func TestCreateAccount_Success(t *testing.T) {
	uc, mockRepo, _ := setupTestUsecase(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		req      *v1.CreateAccountRequest
		provider v1.AccountProvider
	}{
		{
			name: "CLAUDE_CONSOLE with OAuth",
			req: &v1.CreateAccountRequest{
				Name:      "Test Claude Console",
				Provider:  v1.AccountProvider_CLAUDE_CONSOLE,
				OauthData: `{"access_token":"test_token","refresh_token":"test_refresh"}`,
				RpmLimit:  50,
				TpmLimit:  100000,
				Metadata:  `{"region":"us-east-1"}`,
			},
			provider: v1.AccountProvider_CLAUDE_CONSOLE,
		},
		{
			name: "OPENAI_RESPONSES with API Key",
			req: &v1.CreateAccountRequest{
				Name:     "Test OpenAI Responses",
				Provider: v1.AccountProvider_OPENAI_RESPONSES,
				ApiKey:   "sk-test-1234567890abcdef",
				RpmLimit: 60,
				TpmLimit: 200000,
			},
			provider: v1.AccountProvider_OPENAI_RESPONSES,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo.On("CreateAccount", ctx, mock.AnythingOfType("*data.Account")).
				Return(nil).Once()

			result, err := uc.CreateAccount(ctx, tt.req)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.req.Name, result.Name)
			assert.Equal(t, tt.provider, result.Provider)
			assert.Equal(t, int32(100), result.HealthScore)
			assert.False(t, result.IsCircuitBroken)
			assert.Equal(t, v1.AccountStatus_ACCOUNT_ACTIVE, result.Status)

			// Verify sensitive data is masked
			if tt.req.ApiKey != "" {
				assert.NotEqual(t, tt.req.ApiKey, result.ApiKeyEncrypted)
				assert.Contains(t, result.ApiKeyEncrypted, "****")
			}
			if tt.req.OauthData != "" {
				assert.Equal(t, "[ENCRYPTED]", result.OauthDataEncrypted)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

// TestCreateAccount_UnsupportedProvider tests MVP provider validation.
func TestCreateAccount_UnsupportedProvider(t *testing.T) {
	uc, _, _ := setupTestUsecase(t)
	ctx := context.Background()

	unsupportedProviders := []v1.AccountProvider{
		v1.AccountProvider_CLAUDE_OFFICIAL,
		v1.AccountProvider_BEDROCK,
		v1.AccountProvider_CCR,
		v1.AccountProvider_DROID,
		v1.AccountProvider_GEMINI,
		v1.AccountProvider_AZURE_OPENAI,
	}

	for _, provider := range unsupportedProviders {
		t.Run(provider.String(), func(t *testing.T) {
			req := &v1.CreateAccountRequest{
				Name:     "Test Account",
				Provider: provider,
			}

			result, err := uc.CreateAccount(ctx, req)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "unsupported provider")
		})
	}
}

// TestCreateAccount_InvalidMetadata tests metadata validation.
func TestCreateAccount_InvalidMetadata(t *testing.T) {
	uc, _, _ := setupTestUsecase(t)
	ctx := context.Background()

	req := &v1.CreateAccountRequest{
		Name:     "Test Account",
		Provider: v1.AccountProvider_CLAUDE_CONSOLE,
		Metadata: "{invalid json}",
	}

	result, err := uc.CreateAccount(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid metadata")
}

// TestCreateAccount_InvalidOAuthData tests OAuth data validation.
func TestCreateAccount_InvalidOAuthData(t *testing.T) {
	uc, _, _ := setupTestUsecase(t)
	ctx := context.Background()

	req := &v1.CreateAccountRequest{
		Name:      "Test Account",
		Provider:  v1.AccountProvider_CLAUDE_CONSOLE,
		OauthData: "not a json",
	}

	result, err := uc.CreateAccount(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid OAuth data format")
}

// TestCreateAccount_RepoError tests repository error handling.
func TestCreateAccount_RepoError(t *testing.T) {
	uc, mockRepo, _ := setupTestUsecase(t)
	ctx := context.Background()

	req := &v1.CreateAccountRequest{
		Name:     "Test Account",
		Provider: v1.AccountProvider_CLAUDE_CONSOLE,
	}

	mockRepo.On("CreateAccount", ctx, mock.AnythingOfType("*data.Account")).
		Return(errors.New("database error"))

	result, err := uc.CreateAccount(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create account")
	mockRepo.AssertExpectations(t)
}

// TestGetAccount_Success tests successful account retrieval.
func TestGetAccount_Success(t *testing.T) {
	uc, mockRepo, cryptoSvc := setupTestUsecase(t)
	ctx := context.Background()

	// Encrypt test data
	encryptedKey, _ := cryptoSvc.Encrypt("sk-test-key")
	encryptedOAuth, _ := cryptoSvc.Encrypt(`{"token":"test"}`)

	account := &data.Account{
		ID:                 1,
		Name:               "Test Account",
		Provider:           data.ProviderClaudeConsole,
		APIKeyEncrypted:    encryptedKey,
		OAuthDataEncrypted: encryptedOAuth,
		RpmLimit:           50,
		TpmLimit:           100000,
		HealthScore:        100,
		Status:             data.StatusActive,
	}

	mockRepo.On("GetAccount", ctx, int64(1)).Return(account, nil)

	result, err := uc.GetAccount(ctx, 1)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.Id)
	assert.Equal(t, "Test Account", result.Name)

	// Verify sensitive data is masked
	assert.NotEqual(t, encryptedKey, result.ApiKeyEncrypted)
	assert.Equal(t, "[ENCRYPTED]", result.OauthDataEncrypted)

	mockRepo.AssertExpectations(t)
}

// TestGetAccount_NotFound tests account not found error.
func TestGetAccount_NotFound(t *testing.T) {
	uc, mockRepo, _ := setupTestUsecase(t)
	ctx := context.Background()

	mockRepo.On("GetAccount", ctx, int64(999)).
		Return(nil, errors.New("account not found"))

	result, err := uc.GetAccount(ctx, 999)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// TestListAccounts_Success tests successful account listing.
func TestListAccounts_Success(t *testing.T) {
	uc, mockRepo, _ := setupTestUsecase(t)
	ctx := context.Background()

	accounts := []*data.Account{
		{
			ID:          1,
			Name:        "Account 1",
			Provider:    data.ProviderClaudeConsole,
			HealthScore: 100,
			Status:      data.StatusActive,
		},
		{
			ID:          2,
			Name:        "Account 2",
			Provider:    data.ProviderOpenAIResponses,
			HealthScore: 90,
			Status:      data.StatusActive,
		},
	}

	req := &v1.ListAccountsRequest{
		Page:     1,
		PageSize: 10,
		Provider: v1.AccountProvider_CLAUDE_CONSOLE,
		Status:   v1.AccountStatus_ACCOUNT_ACTIVE,
	}

	mockRepo.On("ListAccounts", ctx, mock.AnythingOfType("*data.AccountFilter")).
		Return(accounts, int32(2), nil)

	result, err := uc.ListAccounts(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int32(2), result.Total)
	assert.Len(t, result.Accounts, 2)
	mockRepo.AssertExpectations(t)
}

// TestUpdateAccount_Success tests successful account update.
func TestUpdateAccount_Success(t *testing.T) {
	uc, mockRepo, _ := setupTestUsecase(t)
	ctx := context.Background()

	existingAccount := &data.Account{
		ID:          1,
		Name:        "Old Name",
		Provider:    data.ProviderClaudeConsole,
		RpmLimit:    50,
		TpmLimit:    100000,
		HealthScore: 100,
		Status:      data.StatusActive,
	}

	newName := "New Name"
	newRpmLimit := int32(100)
	newMetadata := `{"region":"us-west-2"}`

	req := &v1.UpdateAccountRequest{
		Id:       1,
		Name:     &newName,
		RpmLimit: &newRpmLimit,
		Metadata: &newMetadata,
	}

	mockRepo.On("GetAccount", ctx, int64(1)).Return(existingAccount, nil)
	mockRepo.On("UpdateAccount", ctx, mock.AnythingOfType("*data.Account")).Return(nil)

	result, err := uc.UpdateAccount(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, newName, result.Name)
	assert.Equal(t, newRpmLimit, result.RpmLimit)
	mockRepo.AssertExpectations(t)
}

// TestUpdateAccount_InvalidMetadata tests metadata validation on update.
func TestUpdateAccount_InvalidMetadata(t *testing.T) {
	uc, mockRepo, _ := setupTestUsecase(t)
	ctx := context.Background()

	existingAccount := &data.Account{
		ID:       1,
		Provider: data.ProviderClaudeConsole,
	}

	invalidMetadata := "not json"
	req := &v1.UpdateAccountRequest{
		Id:       1,
		Metadata: &invalidMetadata,
	}

	mockRepo.On("GetAccount", ctx, int64(1)).Return(existingAccount, nil)

	result, err := uc.UpdateAccount(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid metadata")
	mockRepo.AssertExpectations(t)
}

// TestUpdateAccount_NotFound tests update on non-existent account.
func TestUpdateAccount_NotFound(t *testing.T) {
	uc, mockRepo, _ := setupTestUsecase(t)
	ctx := context.Background()

	newName := "New Name"
	req := &v1.UpdateAccountRequest{
		Id:   999,
		Name: &newName,
	}

	mockRepo.On("GetAccount", ctx, int64(999)).
		Return(nil, errors.New("account not found"))

	result, err := uc.UpdateAccount(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// TestDeleteAccount_Success tests successful account deletion.
func TestDeleteAccount_Success(t *testing.T) {
	uc, mockRepo, _ := setupTestUsecase(t)
	ctx := context.Background()

	mockRepo.On("DeleteAccount", ctx, int64(1)).Return(nil)

	err := uc.DeleteAccount(ctx, 1)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestDeleteAccount_NotFound tests deletion of non-existent account.
func TestDeleteAccount_NotFound(t *testing.T) {
	uc, mockRepo, _ := setupTestUsecase(t)
	ctx := context.Background()

	mockRepo.On("DeleteAccount", ctx, int64(999)).
		Return(errors.New("account not found"))

	err := uc.DeleteAccount(ctx, 999)

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

// TestMaskSensitiveFields tests sensitive data masking.
func TestMaskSensitiveFields(t *testing.T) {
	uc, _, _ := setupTestUsecase(t)

	tests := []struct {
		name               string
		apiKeyEncrypted    string
		oauthDataEncrypted string
		expectedAPIKey     string
		expectedOAuth      string
	}{
		{
			name:               "long API key",
			apiKeyEncrypted:    "sk-proj-1234567890abcdef",
			oauthDataEncrypted: "encrypted_oauth_data",
			expectedAPIKey:     "sk-p****cdef",
			expectedOAuth:      "[ENCRYPTED]",
		},
		{
			name:               "short API key (will not be masked, <= 8 chars)",
			apiKeyEncrypted:    "12345678",
			oauthDataEncrypted: "",
			expectedAPIKey:     "12345678", // Not masked because <= 8 chars
			expectedOAuth:      "",
		},
		{
			name:               "empty fields",
			apiKeyEncrypted:    "",
			oauthDataEncrypted: "",
			expectedAPIKey:     "",
			expectedOAuth:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &v1.Account{
				ApiKeyEncrypted:    tt.apiKeyEncrypted,
				OauthDataEncrypted: tt.oauthDataEncrypted,
			}

			uc.maskSensitiveFields(account)

			assert.Equal(t, tt.expectedAPIKey, account.ApiKeyEncrypted)
			assert.Equal(t, tt.expectedOAuth, account.OauthDataEncrypted)
		})
	}
}
