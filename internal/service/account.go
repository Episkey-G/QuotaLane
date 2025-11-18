package service

import (
	"context"
	"fmt"
	"time"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/biz"
	"QuotaLane/internal/service/oauth"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AccountService implements the AccountService gRPC interface.
type AccountService struct {
	v1.UnimplementedAccountServiceServer

	uc            *biz.AccountUsecase
	oauthRegistry *oauth.Registry
	logger        *log.Helper
}

// NewAccountService creates a new AccountService instance.
func NewAccountService(uc *biz.AccountUsecase, logger log.Logger) *AccountService {
	// Initialize OAuth handler registry
	registry := oauth.NewRegistry(logger)

	// Register OAuth handlers for each provider
	registry.Register(oauth.NewClaudeHandler(uc, logger))
	registry.Register(oauth.NewCodexHandler(uc, logger))

	return &AccountService{
		uc:            uc,
		oauthRegistry: registry,
		logger:        log.NewHelper(logger),
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
		ExpiresAt: account.OAuthExpiresAt, // 返回真实的 OAuth Token 过期时间
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

	// 安全转换 int64 to int32，防止溢出（#nosec G115）
	var responseTimeMsInt32 int32
	if responseTimeMs > 2147483647 { // int32 max value
		responseTimeMsInt32 = 2147483647 // Cap at max int32 value
	} else {
		responseTimeMsInt32 = int32(responseTimeMs) // #nosec G115
	}

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
		ResponseTimeMs: responseTimeMsInt32,
	}, nil
}

// ========== 统一 OAuth 授权流程 RPC 实现 ==========

// GenerateOAuthURL 生成 OAuth 授权 URL（统一接口）
func (s *AccountService) GenerateOAuthURL(ctx context.Context, req *v1.GenerateOAuthURLRequest) (*v1.GenerateOAuthURLResponse, error) {
	s.logger.Infow("GenerateOAuthURL called", "provider", req.Provider)

	// Delegate to provider-specific handler
	resp, err := s.oauthRegistry.GenerateAuthURL(ctx, req)
	if err != nil {
		s.logger.Errorw("failed to generate OAuth URL", "error", err, "provider", req.Provider)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to generate OAuth URL: %v", err))
	}

	s.logger.Infow("OAuth URL generated successfully", "provider", req.Provider, "session_id", resp.SessionId)
	return resp, nil
}

// ExchangeOAuthCode 交换 OAuth 授权码（统一接口）
func (s *AccountService) ExchangeOAuthCode(ctx context.Context, req *v1.ExchangeOAuthCodeRequest) (*v1.ExchangeOAuthCodeResponse, error) {
	s.logger.Infow("ExchangeOAuthCode called", "session_id", req.SessionId, "name", req.Name)

	// Delegate to provider-specific handler
	resp, err := s.oauthRegistry.ExchangeCode(ctx, req)
	if err != nil {
		s.logger.Errorw("failed to exchange OAuth code", "error", err, "session_id", req.SessionId)

		// Map error types to appropriate gRPC codes
		if contains(err.Error(), "session not found") || contains(err.Error(), "expired") {
			return nil, statusError(codes.InvalidArgument, "session not found or expired")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to exchange code: %v", err))
	}

	s.logger.Infow("OAuth code exchanged successfully", "account_id", resp.AccountId, "account_name", resp.AccountName)
	return resp, nil
}

// PollOAuthStatus 轮询 OAuth 授权状态（Device Flow 预留接口）
func (s *AccountService) PollOAuthStatus(ctx context.Context, req *v1.PollOAuthStatusRequest) (*v1.PollOAuthStatusResponse, error) {
	s.logger.Infow("PollOAuthStatus called", "session_id", req.SessionId)

	// TODO: 实现 Device Flow 状态轮询逻辑
	// 当前返回未实现错误
	return nil, status.Error(codes.Unimplemented, "Device Flow is not yet implemented")
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func statusError(code codes.Code, msg string) error {
	return status.Error(code, msg)
}

// ResetHealthScore resets account health score to 100 (admin operation).
// Implements Story 2.5 AC#6
func (s *AccountService) ResetHealthScore(ctx context.Context, req *v1.ResetHealthScoreRequest) (*v1.ResetHealthScoreResponse, error) {
	s.logger.Infow("ResetHealthScore called", "account_id", req.Id)

	// Call AccountUsecase to reset health score
	account, err := s.uc.ResetHealthScoreByAdmin(ctx, req.Id)
	if err != nil {
		s.logger.Errorw("failed to reset health score", "account_id", req.Id, "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to reset health score: %v", err))
	}

	return &v1.ResetHealthScoreResponse{
		Account: account,
	}, nil
}

// ========== Story 2.6: 账户组管理 RPC 实现 ==========

// CreateAccountGroup creates a new account group (admin operation).
func (s *AccountService) CreateAccountGroup(ctx context.Context, req *v1.CreateAccountGroupRequest) (*v1.CreateAccountGroupResponse, error) {
	s.logger.Infow("CreateAccountGroup called", "name", req.Name, "priority", req.Priority, "accounts", len(req.AccountIds))

	// TODO: Add admin permission check

	group, err := s.uc.GetAccountGroupUseCase().CreateAccountGroup(ctx, req.Name, req.Description, req.Priority, req.AccountIds)
	if err != nil {
		s.logger.Errorw("failed to create account group", "name", req.Name, "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create account group: %v", err))
	}

	return &v1.CreateAccountGroupResponse{
		Group: convertAccountGroupToProto(group),
	}, nil
}

// ListAccountGroups retrieves a paginated list of account groups.
func (s *AccountService) ListAccountGroups(ctx context.Context, req *v1.ListAccountGroupsRequest) (*v1.ListAccountGroupsResponse, error) {
	s.logger.Debugw("ListAccountGroups called", "page", req.Page, "page_size", req.PageSize)

	groups, total, err := s.uc.GetAccountGroupUseCase().ListAccountGroups(ctx, req.Page, req.PageSize)
	if err != nil {
		s.logger.Errorw("failed to list account groups", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to list account groups: %v", err))
	}

	protoGroups := make([]*v1.AccountGroup, len(groups))
	for i, g := range groups {
		protoGroups[i] = convertAccountGroupToProto(g)
	}

	return &v1.ListAccountGroupsResponse{
		Groups:   protoGroups,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetAccountGroup retrieves an account group by ID with full details.
func (s *AccountService) GetAccountGroup(ctx context.Context, req *v1.GetAccountGroupRequest) (*v1.GetAccountGroupResponse, error) {
	s.logger.Debugw("GetAccountGroup called", "id", req.Id)

	group, err := s.uc.GetAccountGroupUseCase().GetAccountGroup(ctx, req.Id)
	if err != nil {
		s.logger.Errorw("failed to get account group", "id", req.Id, "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get account group: %v", err))
	}

	// Get accounts in the group
	accounts, err := s.uc.GetAccountGroupUseCase().GetAccountsByGroup(ctx, req.Id)
	if err != nil {
		s.logger.Errorw("failed to get group accounts", "group_id", req.Id, "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get group accounts: %v", err))
	}

	// Convert accounts to Proto (simplified version)
	protoAccounts := make([]*v1.Account, len(accounts))
	for i, acc := range accounts {
		protoAccounts[i] = &v1.Account{
			Id:          acc.ID,
			Name:        acc.Name,
			HealthScore: int32(acc.HealthScore),
			// Add other fields as needed
		}
	}

	return &v1.GetAccountGroupResponse{
		Group:    convertAccountGroupToProto(group),
		Accounts: protoAccounts,
	}, nil
}

// UpdateAccountGroup updates an existing account group (admin operation).
func (s *AccountService) UpdateAccountGroup(ctx context.Context, req *v1.UpdateAccountGroupRequest) (*v1.UpdateAccountGroupResponse, error) {
	s.logger.Infow("UpdateAccountGroup called", "id", req.Id)

	// TODO: Add admin permission check

	name := req.GetName()
	description := req.GetDescription()
	priority := req.GetPriority()

	// IMPORTANT: In Proto3, omitted repeated fields become empty slices, not nil.
	// Current API design: passing empty AccountIds clears all members (documented in Proto).
	// To preserve existing members, client MUST explicitly GET current members first and pass them back.
	accountIDs := req.AccountIds
	if accountIDs == nil {
		accountIDs = []int64{} // Ensure non-nil for consistency
	}

	err := s.uc.GetAccountGroupUseCase().UpdateAccountGroup(ctx, req.Id, name, description, priority, accountIDs)
	if err != nil {
		s.logger.Errorw("failed to update account group", "id", req.Id, "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to update account group: %v", err))
	}

	// Get updated group
	group, err := s.uc.GetAccountGroupUseCase().GetAccountGroup(ctx, req.Id)
	if err != nil {
		s.logger.Errorw("failed to get updated group", "id", req.Id, "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get updated group: %v", err))
	}

	return &v1.UpdateAccountGroupResponse{
		Group: convertAccountGroupToProto(group),
	}, nil
}

// DeleteAccountGroup soft deletes an account group (admin operation).
func (s *AccountService) DeleteAccountGroup(ctx context.Context, req *v1.DeleteAccountGroupRequest) (*v1.DeleteAccountGroupResponse, error) {
	s.logger.Infow("DeleteAccountGroup called", "id", req.Id)

	// TODO: Add admin permission check

	err := s.uc.GetAccountGroupUseCase().DeleteAccountGroup(ctx, req.Id)
	if err != nil {
		s.logger.Errorw("failed to delete account group", "id", req.Id, "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to delete account group: %v", err))
	}

	return &v1.DeleteAccountGroupResponse{
		Success: true,
		Message: "账户组删除成功",
	}, nil
}

// convertAccountGroupToProto converts biz.AccountGroup to Proto message.
func convertAccountGroupToProto(group *biz.AccountGroup) *v1.AccountGroup {
	return &v1.AccountGroup{
		Id:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		Priority:    group.Priority,
		AccountIds:  group.AccountIDs,
		CreatedAt:   timestamppb.New(group.CreatedAt),
		UpdatedAt:   timestamppb.New(group.UpdatedAt),
	}
}
