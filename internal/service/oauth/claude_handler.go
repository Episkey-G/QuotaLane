package oauth

import (
	"context"
	"fmt"
	"strings"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ClaudeHandler handles OAuth flow for Claude Official accounts.
type ClaudeHandler struct {
	uc     *biz.AccountUsecase
	logger *log.Helper
}

// NewClaudeHandler creates a new Claude OAuth handler.
func NewClaudeHandler(uc *biz.AccountUsecase, logger log.Logger) *ClaudeHandler {
	return &ClaudeHandler{
		uc:     uc,
		logger: log.NewHelper(logger),
	}
}

// GenerateAuthURL generates Claude OAuth authorization URL.
func (h *ClaudeHandler) GenerateAuthURL(ctx context.Context, req *v1.GenerateOAuthURLRequest) (*v1.GenerateOAuthURLResponse, error) {
	h.logger.Infow("ClaudeHandler: GenerateAuthURL called", "provider", req.Provider)

	// Extract proxy configuration
	var proxyURL string
	if req.Proxy != nil {
		proxyURL = req.Proxy.Url
	}

	// Extract redirect URI
	redirectURI := ""
	if req.RedirectUri != nil {
		redirectURI = *req.RedirectUri
	}

	// Call business logic layer
	authURL, sessionID, state, err := h.uc.GenerateOAuthURL(ctx, req.Provider, proxyURL, redirectURI, req.Scopes, req.Metadata)
	if err != nil {
		h.logger.Errorw("failed to generate OAuth URL", "error", err, "provider", req.Provider)
		return nil, fmt.Errorf("failed to generate OAuth URL: %w", err)
	}

	return &v1.GenerateOAuthURLResponse{
		AuthUrl:   authURL,
		SessionId: sessionID,
		State:     state,
	}, nil
}

// ExchangeCode exchanges Claude OAuth code for tokens and creates account.
func (h *ClaudeHandler) ExchangeCode(ctx context.Context, req *v1.ExchangeOAuthCodeRequest) (*v1.ExchangeOAuthCodeResponse, error) {
	h.logger.Infow("ClaudeHandler: ExchangeCode called", "session_id", req.SessionId, "name", req.Name)

	// Extract code from callback URL (supports fragment format: code#state)
	code := extractCodeFromCallback(req.Code)
	if code == "" {
		h.logger.Errorw("invalid code parameter", "raw_code", req.Code)
		return nil, fmt.Errorf("invalid code parameter: code is empty")
	}

	// Extract optional parameters
	var description string
	if req.Description != nil {
		description = *req.Description
	}

	var rpmLimit int32
	if req.RpmLimit != nil {
		rpmLimit = *req.RpmLimit
	}

	var tpmLimit int32
	if req.TpmLimit != nil {
		tpmLimit = *req.TpmLimit
	}

	// Call business logic layer
	accountID, accountName, accountStatus, tokenExpiresAt, err := h.uc.ExchangeOAuthCode(
		ctx,
		req.SessionId,
		code,
		req.Name,
		description,
		rpmLimit,
		tpmLimit,
		req.Metadata,
	)
	if err != nil {
		h.logger.Errorw("failed to exchange OAuth code", "error", err, "session_id", req.SessionId)
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Map status
	var protoStatus v1.AccountStatus
	switch accountStatus {
	case "active":
		protoStatus = v1.AccountStatus_ACCOUNT_ACTIVE
	case "created":
		protoStatus = v1.AccountStatus_ACCOUNT_CREATED
	case "error":
		protoStatus = v1.AccountStatus_ACCOUNT_ERROR
	default:
		protoStatus = v1.AccountStatus_ACCOUNT_STATUS_UNSPECIFIED
	}

	h.logger.Infow("OAuth code exchanged successfully",
		"account_id", accountID,
		"account_name", accountName,
		"status", accountStatus)

	var tokenExpiresAtProto *timestamppb.Timestamp
	if tokenExpiresAt != nil {
		tokenExpiresAtProto = timestamppb.New(*tokenExpiresAt)
	}

	return &v1.ExchangeOAuthCodeResponse{
		AccountId:      accountID,
		AccountName:    accountName,
		Status:         protoStatus,
		Message:        "OAuth account created successfully",
		TokenExpiresAt: tokenExpiresAtProto,
	}, nil
}

// ProviderType returns CLAUDE_OFFICIAL.
func (h *ClaudeHandler) ProviderType() v1.AccountProvider {
	return v1.AccountProvider_CLAUDE_OFFICIAL
}

// extractCodeFromCallback extracts authorization code from callback URL or raw code string.
// Supports three formats:
// 1. Claude OAuth (fragment): "code#state"
// 2. Codex OAuth (query): "http://localhost:1455/auth/callback?code=xxx&state=yyy"
// 3. Pure code: "xxx"
func extractCodeFromCallback(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Check for "#" (Claude OAuth fragment format)
	if strings.Contains(input, "#") {
		parts := strings.Split(input, "#")
		if len(parts) >= 1 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Check for "?" (standard OAuth query format)
	if strings.Contains(input, "?") {
		// Simple extraction without full URL parsing
		// Extract code=xxx from query string
		if idx := strings.Index(input, "code="); idx != -1 {
			codeStart := idx + 5 // len("code=")
			codeEnd := strings.Index(input[codeStart:], "&")
			if codeEnd == -1 {
				return strings.TrimSpace(input[codeStart:])
			}
			return strings.TrimSpace(input[codeStart : codeStart+codeEnd])
		}
	}

	// Pure code
	return input
}
