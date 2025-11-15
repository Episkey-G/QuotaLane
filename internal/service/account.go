package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/biz"
	pkgerrors "QuotaLane/pkg/errors"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// mapDBErrorToGRPCStatus converts a database error to a gRPC status error.
// This provides consistent error responses to clients with appropriate gRPC codes.
//
// Mapping:
//   - ErrorTypeDuplicateKey → codes.AlreadyExists (409 HTTP equivalent)
//   - ErrorTypeInvalidJSON → codes.InvalidArgument (400 HTTP equivalent)
//   - ErrorTypeInvalidValue → codes.InvalidArgument (400 HTTP equivalent)
//   - ErrorTypeConstraintViolation → codes.FailedPrecondition (412 HTTP equivalent)
//   - ErrorTypeNotFound → codes.NotFound (404 HTTP equivalent)
//   - ErrorTypeDeadlock → codes.Aborted (409 HTTP equivalent)
//   - ErrorTypeConnectionError → codes.Unavailable (503 HTTP equivalent)
//   - ErrorTypeUnknown → codes.Internal (500 HTTP equivalent)
func mapDBErrorToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}

	// Try to unwrap to get the database error
	var dbErr *pkgerrors.DatabaseError
	if !errors.As(err, &dbErr) {
		// Not a classified database error, return as internal error
		return status.Error(codes.Internal, err.Error())
	}

	switch dbErr.Type {
	case pkgerrors.ErrorTypeDuplicateKey:
		return status.Error(codes.AlreadyExists, "resource already exists (duplicate key)")

	case pkgerrors.ErrorTypeInvalidJSON:
		return status.Error(codes.InvalidArgument, "invalid JSON data in request")

	case pkgerrors.ErrorTypeInvalidValue:
		return status.Error(codes.InvalidArgument, "invalid value in request")

	case pkgerrors.ErrorTypeDataTooLong:
		return status.Error(codes.InvalidArgument, "data too long for field")

	case pkgerrors.ErrorTypeConstraintViolation:
		return status.Error(codes.FailedPrecondition, "constraint violation")

	case pkgerrors.ErrorTypeNotFound:
		return status.Error(codes.NotFound, "resource not found")

	case pkgerrors.ErrorTypeDeadlock:
		return status.Error(codes.Aborted, "operation aborted due to deadlock, please retry")

	case pkgerrors.ErrorTypeConnectionError:
		return status.Error(codes.Unavailable, "database connection error, please try again later")

	default:
		return status.Error(codes.Internal, "internal database error")
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

// RefreshToken refreshes OAuth token for an account.
// This RPC manually triggers token refresh for a specific Claude account.
// Only admin users can call this endpoint (permission check should be done in middleware).
func (s *AccountService) RefreshToken(ctx context.Context, req *v1.RefreshTokenRequest) (*v1.RefreshTokenResponse, error) {
	s.logger.Infow("RefreshToken called", "account_id", req.Id)

	// TODO: Add admin permission check here (JWT middleware should validate role = admin)
	// This will be implemented in Story 4.2 (JWT Auth Middleware)

	// Call business logic to refresh token
	if err := s.uc.RefreshClaudeToken(ctx, req.Id); err != nil {
		s.logger.Errorw("failed to refresh token", "account_id", req.Id, "error", err)
		return &v1.RefreshTokenResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Fetch updated account to get new expires_at
	account, err := s.uc.GetAccount(ctx, req.Id)
	if err != nil {
		s.logger.Warnw("failed to get updated account after refresh", "account_id", req.Id, "error", err)
		// Still return success since refresh succeeded
		return &v1.RefreshTokenResponse{
			Success: true,
			Message: "Token refreshed successfully",
		}, nil
	}

	return &v1.RefreshTokenResponse{
		Success:   true,
		Message:   "Token refreshed successfully",
		ExpiresAt: account.OauthExpiresAt, // 返回真实的 OAuth Token 过期时间
	}, nil
}

// TestAccount tests account connectivity and health.
// Supports multiple provider types: OpenAI Responses, Claude Console, etc.
func (s *AccountService) TestAccount(ctx context.Context, req *v1.TestAccountRequest) (*v1.TestAccountResponse, error) {
	startTime := time.Now()

	// 获取账户信息以确定 Provider 类型
	account, err := s.uc.GetAccount(ctx, req.Id)
	if err != nil {
		s.logger.Errorw("failed to get account for testing",
			"id", req.Id,
			"error", err)
		return &v1.TestAccountResponse{
			Success:        false,
			Message:        fmt.Sprintf("Failed to get account: %v", err),
			HealthScore:    0,
			ResponseTimeMs: 0,
		}, nil
	}

	var testErr error
	var message string

	// 根据 Provider 类型调用对应的验证方法
	switch account.Provider {
	case v1.AccountProvider_OPENAI_RESPONSES:
		// OpenAI Responses: 调用 ValidateOpenAIResponsesAccount
		testErr = s.uc.ValidateOpenAIResponsesAccount(ctx, req.Id)
		if testErr == nil {
			message = "OpenAI Responses account test passed"
		} else {
			message = fmt.Sprintf("OpenAI Responses account test failed: %v", testErr)
		}

	case v1.AccountProvider_CLAUDE_CONSOLE, v1.AccountProvider_CLAUDE_OFFICIAL:
		// Claude: 调用 RefreshClaudeToken（Story 2.2 已实现）
		testErr = s.uc.RefreshClaudeToken(ctx, req.Id)
		if testErr == nil {
			message = "Claude account test passed (token refreshed)"
		} else {
			message = fmt.Sprintf("Claude account test failed: %v", testErr)
		}

	case v1.AccountProvider_CODEX_CLI:
		// Codex CLI: 调用 ValidateCodexCLIAccount
		testErr = s.uc.ValidateCodexCLIAccount(ctx, req.Id)
		if testErr == nil {
			message = "Codex CLI account test passed"
		} else {
			message = fmt.Sprintf("Codex CLI account test failed: %v", testErr)
		}

	default:
		// 其他类型暂不支持
		message = fmt.Sprintf("该账户类型暂不支持健康检查: %s", account.Provider.String())
		return &v1.TestAccountResponse{
			Success:        false,
			Message:        message,
			HealthScore:    0,
			ResponseTimeMs: 0,
		}, nil
	}

	// 测试完成后，重新获取账户信息（健康分数可能已更新）
	updatedAccount, err := s.uc.GetAccount(ctx, req.Id)
	if err != nil {
		s.logger.Warnw("failed to get updated account after test",
			"id", req.Id,
			"error", err)
		// 使用旧的账户信息
		updatedAccount = account
	}

	// 计算响应时间
	responseTimeMs := time.Since(startTime).Milliseconds()

	// 脱敏 API Key 和 Base API（前 8 位 + ****）
	if updatedAccount.ApiKeyEncrypted != "" && len(updatedAccount.ApiKeyEncrypted) > 8 {
		updatedAccount.ApiKeyEncrypted = updatedAccount.ApiKeyEncrypted[:8] + "****"
	}

	s.logger.Infow("account test completed",
		"id", req.Id,
		"provider", account.Provider.String(),
		"success", testErr == nil,
		"response_time_ms", responseTimeMs)

	return &v1.TestAccountResponse{
		Success:        testErr == nil,
		Message:        message,
		HealthScore:    updatedAccount.HealthScore,
		ResponseTimeMs: int32(responseTimeMs),
	}, nil
}

// ========== Codex CLI OAuth 授权流程 ==========

// GenerateOpenAIAuthURL 生成 OpenAI OAuth 授权链接
func (s *AccountService) GenerateOpenAIAuthURL(ctx context.Context, req *v1.GenerateOpenAIAuthURLRequest) (*v1.GenerateOpenAIAuthURLResponse, error) {
	s.logger.Infow("GenerateOpenAIAuthURL called", "proxy_url", req.GetProxyUrl())

	// 调用 Biz 层生成授权 URL
	authURL, sessionID, state, err := s.uc.GenerateOpenAIAuthURL(ctx, req.GetProxyUrl())
	if err != nil {
		s.logger.Errorw("failed to generate OpenAI auth URL", "error", err)
		return nil, err
	}

	s.logger.Infow("OpenAI auth URL generated successfully",
		"session_id", sessionID,
		"proxy_url", req.GetProxyUrl())

	return &v1.GenerateOpenAIAuthURLResponse{
		AuthUrl:   authURL,
		SessionId: sessionID,
		State:     state,
	}, nil
}

// ExchangeOpenAICode 交换 OpenAI OAuth 授权码并创建账户
func (s *AccountService) ExchangeOpenAICode(ctx context.Context, req *v1.ExchangeOpenAICodeRequest) (*v1.ExchangeOpenAICodeResponse, error) {
	s.logger.Infow("ExchangeOpenAICode called",
		"session_id", req.SessionId,
		"name", req.Name)

	// 调用 Biz 层交换授权码并创建账户
	accountID, accountName, status, tokenExpiresAt, err := s.uc.ExchangeOpenAICode(
		ctx,
		req.SessionId,
		req.Code,
		req.Name,
		req.GetDescription(),
		req.GetRpmLimit(),
		req.GetTpmLimit(),
		req.GetMetadata(),
	)

	if err != nil {
		s.logger.Errorw("failed to exchange OpenAI code",
			"session_id", req.SessionId,
			"error", err)

		// 【Fail Fast】验证失败时不会创建账户（已在 Biz 层实现）
		// 将数据库错误映射为 gRPC Status Codes
		return nil, mapDBErrorToGRPCStatus(err)
	}

	// 转换状态为 Proto 枚举
	var protoStatus v1.AccountStatus
	switch status {
	case "active":
		protoStatus = v1.AccountStatus_ACCOUNT_ACTIVE
	case "created":
		protoStatus = v1.AccountStatus_ACCOUNT_CREATED
	case "error":
		protoStatus = v1.AccountStatus_ACCOUNT_ERROR
	default:
		protoStatus = v1.AccountStatus_ACCOUNT_STATUS_UNSPECIFIED
	}

	s.logger.Infow("OpenAI code exchanged successfully",
		"account_id", accountID,
		"account_name", accountName,
		"status", status)

	return &v1.ExchangeOpenAICodeResponse{
		AccountId:      accountID,
		AccountName:    accountName,
		Status:         protoStatus,
		Message:        "Codex CLI account created successfully",
		TokenExpiresAt: timestampProto(tokenExpiresAt),
	}, nil
}

// timestampProto 将 *time.Time 转换为 *timestamppb.Timestamp
func timestampProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
