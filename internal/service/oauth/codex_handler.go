package oauth

import (
	"context"
	"fmt"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CodexHandler handles OAuth flow for Codex CLI accounts.
type CodexHandler struct {
	uc     *biz.AccountUsecase
	logger *log.Helper
}

// NewCodexHandler creates a new Codex OAuth handler.
func NewCodexHandler(uc *biz.AccountUsecase, logger log.Logger) *CodexHandler {
	return &CodexHandler{
		uc:     uc,
		logger: log.NewHelper(logger),
	}
}

// GenerateAuthURL generates Codex OAuth authorization URL.
func (h *CodexHandler) GenerateAuthURL(ctx context.Context, req *v1.GenerateOAuthURLRequest) (*v1.GenerateOAuthURLResponse, error) {
	h.logger.Infow("CodexHandler: GenerateAuthURL called", "provider", req.Provider)

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

// ExchangeCode exchanges Codex OAuth code for tokens and creates account.
func (h *CodexHandler) ExchangeCode(ctx context.Context, req *v1.ExchangeOAuthCodeRequest) (*v1.ExchangeOAuthCodeResponse, error) {
	h.logger.Infow("CodexHandler: ExchangeCode called", "session_id", req.SessionId, "name", req.Name)

	// Extract code from callback URL (supports query format: ?code=xxx)
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

	// Convert *time.Time to *timestamppb.Timestamp
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

// ProviderType returns CODEX_CLI.
func (h *CodexHandler) ProviderType() v1.AccountProvider {
	return v1.AccountProvider_CODEX_CLI
}
