package biz

import (
	"context"
	"fmt"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/data"
	"QuotaLane/pkg/crypto"
	"QuotaLane/pkg/oauth"
	pkgoauth "QuotaLane/pkg/oauth" // 统一 OAuth Manager
	"QuotaLane/pkg/openai"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// AccountUsecase implements account business logic.
type AccountUsecase struct {
	repo          data.AccountRepo
	crypto        *crypto.AESCrypto
	oauth         oauth.OAuthService
	openaiService openai.OpenAIService
	oauthManager  *pkgoauth.OAuthManager // 统一 OAuth Manager
	rdb           *redis.Client
	logger        *log.Helper
}

// NewAccountUsecase creates a new account usecase.
func NewAccountUsecase(repo data.AccountRepo, crypto *crypto.AESCrypto, oauth oauth.OAuthService, openaiService openai.OpenAIService, oauthManager *pkgoauth.OAuthManager, rdb *redis.Client, logger log.Logger) *AccountUsecase {
	return &AccountUsecase{
		repo:          repo,
		crypto:        crypto,
		oauth:         oauth,
		openaiService: openaiService,
		oauthManager:  oauthManager,
		rdb:           rdb,
		logger:        log.NewHelper(logger),
	}
}

// CreateAccount creates a new account with encrypted credentials.
// MVP: Only supports CLAUDE_CONSOLE and OPENAI_RESPONSES providers.
func (uc *AccountUsecase) CreateAccount(ctx context.Context, req *v1.CreateAccountRequest) (*v1.Account, error) {
	// Validate provider (MVP restriction)
	if !uc.isSupportedProvider(req.Provider) {
		return nil, fmt.Errorf("unsupported provider: %v. MVP only supports CLAUDE_CONSOLE and OPENAI_RESPONSES",
			req.Provider)
	}

	// Validate and prepare metadata
	var metadataPtr *string
	if req.Metadata != "" {
		if err := data.ValidateMetadataJSON(req.Metadata); err != nil {
			return nil, fmt.Errorf("invalid metadata: %w", err)
		}
		metadataPtr = &req.Metadata
	}

	// Create account model
	account := &data.Account{
		Name:            req.Name,
		Provider:        data.ProviderFromProto(req.Provider),
		RpmLimit:        req.RpmLimit,
		TpmLimit:        req.TpmLimit,
		HealthScore:     100, // Initial health score
		IsCircuitBroken: false,
		Status:          data.StatusActive,
		Metadata:        metadataPtr,
	}

	// Encrypt API Key if provided (for OPENAI_RESPONSES)
	if req.ApiKey != "" {
		encrypted, err := uc.crypto.Encrypt(req.ApiKey)
		if err != nil {
			uc.logger.Errorf("failed to encrypt API key: %v", err)
			return nil, fmt.Errorf("failed to encrypt credentials")
		}
		account.APIKeyEncrypted = encrypted
	}

	// Encrypt OAuth Data if provided (for CLAUDE_CONSOLE)
	if req.OAuthData != "" {
		// Validate OAuth data is valid JSON
		if err := data.ValidateMetadataJSON(req.OAuthData); err != nil {
			return nil, fmt.Errorf("invalid OAuth data format: %w", err)
		}

		encrypted, err := uc.crypto.Encrypt(req.OAuthData)
		if err != nil {
			uc.logger.Errorf("failed to encrypt OAuth data: %v", err)
			return nil, fmt.Errorf("failed to encrypt credentials")
		}
		account.OAuthDataEncrypted = encrypted
	}

	// Save to database
	if err := uc.repo.CreateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	uc.logger.Infow("account created successfully",
		"id", account.ID,
		"name", account.Name,
		"provider", account.Provider)

	// Convert to proto and mask sensitive data
	proto := account.ToProto()
	uc.maskSensitiveFields(proto)

	return proto, nil
}

// GetAccount retrieves an account by ID with masked sensitive data.
func (uc *AccountUsecase) GetAccount(ctx context.Context, id int64) (*v1.Account, error) {
	account, err := uc.repo.GetAccount(ctx, id)
	if err != nil {
		return nil, err
	}

	// Convert to proto
	proto := account.ToProto()

	// Mask sensitive data
	uc.maskSensitiveFields(proto)

	return proto, nil
}

// ListAccounts retrieves accounts with pagination and filters.
func (uc *AccountUsecase) ListAccounts(ctx context.Context, req *v1.ListAccountsRequest) (*v1.ListAccountsResponse, error) {
	// Convert proto filter to data filter
	filter := &data.AccountFilter{
		Page:     req.Page,
		PageSize: req.PageSize,
		Provider: data.ProviderFromProto(req.Provider),
		Status:   data.StatusFromProto(req.Status),
	}

	accounts, total, err := uc.repo.ListAccounts(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Convert to proto accounts with masked sensitive data
	protoAccounts := make([]*v1.Account, 0, len(accounts))
	for _, account := range accounts {
		proto := account.ToProto()
		uc.maskSensitiveFields(proto)
		protoAccounts = append(protoAccounts, proto)
	}

	return &v1.ListAccountsResponse{
		Accounts: protoAccounts,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// UpdateAccount updates account information (non-sensitive fields).
func (uc *AccountUsecase) UpdateAccount(ctx context.Context, req *v1.UpdateAccountRequest) (*v1.Account, error) {
	// Fetch existing account
	account, err := uc.repo.GetAccount(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Name != nil {
		account.Name = *req.Name
	}
	if req.RpmLimit != nil {
		account.RpmLimit = *req.RpmLimit
	}
	if req.TpmLimit != nil {
		account.TpmLimit = *req.TpmLimit
	}
	if req.Status != nil {
		account.Status = data.StatusFromProto(*req.Status)
	}
	if req.Metadata != nil {
		// Validate metadata JSON
		if err := data.ValidateMetadataJSON(*req.Metadata); err != nil {
			return nil, fmt.Errorf("invalid metadata: %w", err)
		}
		account.Metadata = req.Metadata
	}

	// Update API Key if provided
	if req.ApiKey != nil && *req.ApiKey != "" {
		encrypted, err := uc.crypto.Encrypt(*req.ApiKey)
		if err != nil {
			uc.logger.Errorf("failed to encrypt API key: %v", err)
			return nil, fmt.Errorf("failed to encrypt credentials")
		}
		account.APIKeyEncrypted = encrypted
	}

	// Update OAuth Data if provided
	if req.OAuthData != nil && *req.OAuthData != "" {
		// Validate OAuth data is valid JSON
		if err := data.ValidateMetadataJSON(*req.OAuthData); err != nil {
			return nil, fmt.Errorf("invalid OAuth data format: %w", err)
		}

		encrypted, err := uc.crypto.Encrypt(*req.OAuthData)
		if err != nil {
			uc.logger.Errorf("failed to encrypt OAuth data: %v", err)
			return nil, fmt.Errorf("failed to encrypt credentials")
		}
		account.OAuthDataEncrypted = encrypted
	}

	// Save changes
	if err := uc.repo.UpdateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to update account: %w", err)
	}

	uc.logger.Infow("account updated successfully", "id", account.ID)

	// Convert to proto and mask sensitive data
	proto := account.ToProto()
	uc.maskSensitiveFields(proto)

	return proto, nil
}

// DeleteAccount performs soft delete on an account.
func (uc *AccountUsecase) DeleteAccount(ctx context.Context, id int64) error {
	if err := uc.repo.DeleteAccount(ctx, id); err != nil {
		return err
	}

	uc.logger.Infow("account deleted successfully", "id", id)
	return nil
}

// isSupportedProvider checks if provider is supported in MVP.
// MVP: Only CLAUDE_CONSOLE (2) and OPENAI_RESPONSES (7) are supported.
func (uc *AccountUsecase) isSupportedProvider(provider v1.AccountProvider) bool {
	return provider == v1.AccountProvider_CLAUDE_CONSOLE ||
		provider == v1.AccountProvider_OPENAI_RESPONSES
}

// maskSensitiveFields masks sensitive data in Account proto for display.
func (uc *AccountUsecase) maskSensitiveFields(account *v1.Account) {
	// Mask API Key: show first 4 + last 4 characters
	if account.ApiKeyEncrypted != "" && len(account.ApiKeyEncrypted) > 8 {
		account.ApiKeyEncrypted = data.MaskAPIKey(account.ApiKeyEncrypted)
	}

	// Mask OAuth Data: replace with placeholder
	if account.OAuthDataEncrypted != "" {
		account.OAuthDataEncrypted = "[ENCRYPTED]"
	}
}
