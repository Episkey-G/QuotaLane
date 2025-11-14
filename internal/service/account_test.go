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

	"github.com/go-kratos/kratos/v2/log"
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

// setupTestService creates a test AccountService with mock repository.
func setupTestService(t *testing.T) (*AccountService, *MockAccountRepo) {
	mockRepo := new(MockAccountRepo)
	logger := log.DefaultLogger

	// Create AES crypto with test key
	testKey := []byte("12345678901234567890123456789012")
	cryptoSvc, err := crypto.NewAESCrypto(testKey)
	assert.NoError(t, err)

	// Create real usecase with mock repo
	uc := biz.NewAccountUsecase(mockRepo, cryptoSvc, logger)

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

// TestRefreshToken tests RefreshToken placeholder.
func TestRefreshToken(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	req := &v1.RefreshTokenRequest{
		Id: 1,
	}

	resp, err := svc.RefreshToken(ctx, req)

	// Should return success=false with message about future implementation
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "Story 2.2")
}

// TestTestAccount tests TestAccount placeholder.
func TestTestAccount(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	req := &v1.TestAccountRequest{
		Id: 1,
	}

	resp, err := svc.TestAccount(ctx, req)

	// Should return success=false with message about future implementation
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "Story 2.3")
	assert.Equal(t, int32(0), resp.HealthScore)
	assert.Equal(t, int32(0), resp.ResponseTimeMs)
}
