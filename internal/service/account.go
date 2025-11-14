package service

import (
	"context"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// AccountService implements the AccountService gRPC interface.
type AccountService struct {
	v1.UnimplementedAccountServiceServer

	uc     *biz.AccountUsecase
	logger *log.Helper
}

// NewAccountService creates a new AccountService instance.
func NewAccountService(uc *biz.AccountUsecase, logger log.Logger) *AccountService {
	return &AccountService{
		uc:     uc,
		logger: log.NewHelper(logger),
	}
}

// CreateAccount creates a new account.
func (s *AccountService) CreateAccount(ctx context.Context, req *v1.CreateAccountRequest) (*v1.CreateAccountResponse, error) {
	s.logger.Infow("CreateAccount called", "name", req.Name, "provider", req.Provider)

	account, err := s.uc.CreateAccount(ctx, req)
	if err != nil {
		s.logger.Errorw("failed to create account", "error", err)
		return nil, err
	}

	return &v1.CreateAccountResponse{
		Account: account,
	}, nil
}

// ListAccounts retrieves accounts with pagination and filters.
func (s *AccountService) ListAccounts(ctx context.Context, req *v1.ListAccountsRequest) (*v1.ListAccountsResponse, error) {
	s.logger.Debugw("ListAccounts called", "page", req.Page, "page_size", req.PageSize)

	resp, err := s.uc.ListAccounts(ctx, req)
	if err != nil {
		s.logger.Errorw("failed to list accounts", "error", err)
		return nil, err
	}

	return resp, nil
}

// GetAccount retrieves an account by ID.
func (s *AccountService) GetAccount(ctx context.Context, req *v1.GetAccountRequest) (*v1.GetAccountResponse, error) {
	s.logger.Debugw("GetAccount called", "id", req.Id)

	account, err := s.uc.GetAccount(ctx, req.Id)
	if err != nil {
		s.logger.Errorw("failed to get account", "id", req.Id, "error", err)
		return nil, err
	}

	return &v1.GetAccountResponse{
		Account: account,
	}, nil
}

// UpdateAccount updates an account.
func (s *AccountService) UpdateAccount(ctx context.Context, req *v1.UpdateAccountRequest) (*v1.UpdateAccountResponse, error) {
	s.logger.Infow("UpdateAccount called", "id", req.Id)

	account, err := s.uc.UpdateAccount(ctx, req)
	if err != nil {
		s.logger.Errorw("failed to update account", "id", req.Id, "error", err)
		return nil, err
	}

	return &v1.UpdateAccountResponse{
		Account: account,
	}, nil
}

// DeleteAccount soft deletes an account.
func (s *AccountService) DeleteAccount(ctx context.Context, req *v1.DeleteAccountRequest) (*v1.DeleteAccountResponse, error) {
	s.logger.Infow("DeleteAccount called", "id", req.Id)

	if err := s.uc.DeleteAccount(ctx, req.Id); err != nil {
		s.logger.Errorw("failed to delete account", "id", req.Id, "error", err)
		return nil, err
	}

	return &v1.DeleteAccountResponse{
		Success: true,
		Message: "Account deleted successfully",
	}, nil
}

// RefreshToken refreshes OAuth token for an account (Not implemented in this story).
func (s *AccountService) RefreshToken(ctx context.Context, req *v1.RefreshTokenRequest) (*v1.RefreshTokenResponse, error) {
	s.logger.Warnw("RefreshToken not implemented in Story 2.1", "id", req.Id)
	return &v1.RefreshTokenResponse{
		Success: false,
		Message: "RefreshToken feature will be implemented in Story 2.2",
	}, nil
}

// TestAccount tests account connectivity and health (Not implemented in this story).
func (s *AccountService) TestAccount(ctx context.Context, req *v1.TestAccountRequest) (*v1.TestAccountResponse, error) {
	s.logger.Warnw("TestAccount not implemented in Story 2.1", "id", req.Id)
	return &v1.TestAccountResponse{
		Success:      false,
		Message:      "TestAccount feature will be implemented in Story 2.3",
		HealthScore:  0,
		ResponseTimeMs: 0,
	}, nil
}
