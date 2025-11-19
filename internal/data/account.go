package data

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	v1 "QuotaLane/api/v1"
	pkgerrors "QuotaLane/pkg/errors"
	"QuotaLane/pkg/metadata"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// AccountProvider represents the database ENUM type for provider.
type AccountProvider string

// Account provider constants representing different AI service providers.
const (
	ProviderClaudeOfficial  AccountProvider = "claude-official"
	ProviderClaudeConsole   AccountProvider = "claude-console"
	ProviderBedrock         AccountProvider = "bedrock"
	ProviderCCR             AccountProvider = "ccr"
	ProviderDroid           AccountProvider = "droid"
	ProviderGemini          AccountProvider = "gemini"
	ProviderOpenAIResponses AccountProvider = "openai-responses"
	ProviderCodexCLI        AccountProvider = "codex-cli"
	ProviderAzureOpenAI     AccountProvider = "azure-openai"
)

// AccountStatus represents the database ENUM type for status.
type AccountStatus string

// Account status constants representing the current state of an account.
const (
	StatusCreated  AccountStatus = "created" // 账户已创建但未验证
	StatusActive   AccountStatus = "active"
	StatusInactive AccountStatus = "inactive"
	StatusError    AccountStatus = "error"
)

// Account is the GORM model for api_accounts table.
type Account struct {
	ID                 int64           `gorm:"primaryKey;column:id"`
	Name               string          `gorm:"column:name;size:100;not null"`
	Description        string          `gorm:"column:description;type:text"`
	Provider           AccountProvider `gorm:"column:provider;type:enum('claude-official','claude-console','bedrock','ccr','droid','gemini','openai-responses','codex-cli','azure-openai');not null"`
	APIKeyEncrypted    string          `gorm:"column:api_key_encrypted;type:text"`
	BaseAPI            string          `gorm:"column:base_api;size:255"` // OpenAI Responses 等服务的 API 基础地址
	OAuthDataEncrypted string          `gorm:"column:oauth_data_encrypted;type:text"`
	OAuthExpiresAt     *time.Time      `gorm:"column:oauth_expires_at"` // OAuth Token 过期时间（可为 NULL）
	// Codex CLI OAuth 相关字段
	AccessTokenEncrypted  string        `gorm:"column:access_token_encrypted;type:varchar(1024)"`
	RefreshTokenEncrypted string        `gorm:"column:refresh_token_encrypted;type:varchar(1024)"`
	TokenExpiresAt        *time.Time    `gorm:"column:token_expires_at"`
	IDTokenEncrypted      string        `gorm:"column:id_token_encrypted;type:varchar(2048)"`
	Organizations         string        `gorm:"column:organizations;type:text"` // JSON array
	RpmLimit              int32         `gorm:"column:rpm_limit;default:0;not null"`
	TpmLimit              int32         `gorm:"column:tpm_limit;default:0;not null"`
	HealthScore           int           `gorm:"column:health_score;default:100;not null"`
	IsCircuitBroken       bool          `gorm:"column:is_circuit_broken;default:false;not null"`
	Status                AccountStatus `gorm:"column:status;type:enum('created','active','inactive','error');default:'active';not null"`
	Metadata              *string       `gorm:"column:metadata;type:json"`                    // JSON string (pointer for NULL support)
	Version               int32         `gorm:"column:version;default:1;not null"`            // 乐观锁版本号
	CircuitBrokenAt       *time.Time    `gorm:"column:circuit_broken_at"`                     // 熔断触发时间
	LastError             *string       `gorm:"column:last_error;type:text"`                  // 最后一次错误信息（JSON，pointer for NULL support）
	LastErrorAt           *time.Time    `gorm:"column:last_error_at"`                         // 最后一次错误发生时间
	ConsecutiveErrors     int32         `gorm:"column:consecutive_errors;default:0;not null"` // 连续失败次数
	CreatedAt             time.Time     `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt             time.Time     `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName specifies the table name for GORM.
func (Account) TableName() string {
	return "api_accounts"
}

// Scan implements sql.Scanner interface for AccountProvider.
func (p *AccountProvider) Scan(value interface{}) error {
	if value == nil {
		*p = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*p = AccountProvider(v)
	case string:
		*p = AccountProvider(v)
	default:
		return fmt.Errorf("cannot scan type %T into AccountProvider", value)
	}
	return nil
}

// Value implements driver.Valuer interface for AccountProvider.
func (p AccountProvider) Value() (driver.Value, error) {
	return string(p), nil
}

// Scan implements sql.Scanner interface for AccountStatus.
func (s *AccountStatus) Scan(value interface{}) error {
	if value == nil {
		*s = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*s = AccountStatus(v)
	case string:
		*s = AccountStatus(v)
	default:
		return fmt.Errorf("cannot scan type %T into AccountStatus", value)
	}
	return nil
}

// Value implements driver.Valuer interface for AccountStatus.
func (s AccountStatus) Value() (driver.Value, error) {
	return string(s), nil
}

// ProviderToProto converts database AccountProvider to Proto enum.
func ProviderToProto(p AccountProvider) v1.AccountProvider {
	switch p {
	case ProviderClaudeOfficial:
		return v1.AccountProvider_CLAUDE_OFFICIAL
	case ProviderClaudeConsole:
		return v1.AccountProvider_CLAUDE_CONSOLE
	case ProviderBedrock:
		return v1.AccountProvider_BEDROCK
	case ProviderCCR:
		return v1.AccountProvider_CCR
	case ProviderDroid:
		return v1.AccountProvider_DROID
	case ProviderGemini:
		return v1.AccountProvider_GEMINI
	case ProviderOpenAIResponses:
		return v1.AccountProvider_OPENAI_RESPONSES
	case ProviderAzureOpenAI:
		return v1.AccountProvider_AZURE_OPENAI
	default:
		return v1.AccountProvider_ACCOUNT_PROVIDER_UNSPECIFIED
	}
}

// ProviderFromProto converts Proto enum to database AccountProvider.
func ProviderFromProto(p v1.AccountProvider) AccountProvider {
	switch p {
	case v1.AccountProvider_CLAUDE_OFFICIAL:
		return ProviderClaudeOfficial
	case v1.AccountProvider_CLAUDE_CONSOLE:
		return ProviderClaudeConsole
	case v1.AccountProvider_BEDROCK:
		return ProviderBedrock
	case v1.AccountProvider_CCR:
		return ProviderCCR
	case v1.AccountProvider_DROID:
		return ProviderDroid
	case v1.AccountProvider_GEMINI:
		return ProviderGemini
	case v1.AccountProvider_OPENAI_RESPONSES:
		return ProviderOpenAIResponses
	case v1.AccountProvider_AZURE_OPENAI:
		return ProviderAzureOpenAI
	default:
		return ""
	}
}

// StatusToProto converts database AccountStatus to Proto enum.
func StatusToProto(s AccountStatus) v1.AccountStatus {
	switch s {
	case StatusActive:
		return v1.AccountStatus_ACCOUNT_ACTIVE
	case StatusInactive:
		return v1.AccountStatus_ACCOUNT_INACTIVE
	case StatusError:
		return v1.AccountStatus_ACCOUNT_ERROR
	default:
		return v1.AccountStatus_ACCOUNT_STATUS_UNSPECIFIED
	}
}

// StatusFromProto converts Proto enum to database AccountStatus.
func StatusFromProto(s v1.AccountStatus) AccountStatus {
	switch s {
	case v1.AccountStatus_ACCOUNT_ACTIVE:
		return StatusActive
	case v1.AccountStatus_ACCOUNT_INACTIVE:
		return StatusInactive
	case v1.AccountStatus_ACCOUNT_ERROR:
		return StatusError
	default:
		return StatusActive // Default to active
	}
}

// ToProto converts GORM Account model to Proto Account message.
func (a *Account) ToProto() *v1.Account {
	// Handle Metadata pointer - convert to string
	var metadataStr string
	if a.Metadata != nil {
		metadataStr = *a.Metadata
	}

	proto := &v1.Account{
		Id:                 a.ID,
		Name:               a.Name,
		Provider:           ProviderToProto(a.Provider),
		ApiKeyEncrypted:    a.APIKeyEncrypted,
		OAuthDataEncrypted: a.OAuthDataEncrypted,
		RpmLimit:           a.RpmLimit,
		TpmLimit:           a.TpmLimit,
		HealthScore:        int32(a.HealthScore), // #nosec G115 -- HealthScore is bounded 0-100
		IsCircuitBroken:    a.IsCircuitBroken,
		Status:             StatusToProto(a.Status),
		Metadata:           metadataStr,
		CreatedAt:          timestamppb.New(a.CreatedAt),
		UpdatedAt:          timestamppb.New(a.UpdatedAt),
	}

	// OAuthExpiresAt 可为空，只有在非 nil 时才转换
	if a.OAuthExpiresAt != nil {
		proto.OAuthExpiresAt = timestamppb.New(*a.OAuthExpiresAt)
	}

	return proto
}

// MaskSensitiveData masks sensitive fields in Account for display.
// API Key: show first 4 + last 4 characters (e.g., "sk-proj****1234")
// OAuth Data: replace with "[ENCRYPTED]"
func (a *Account) MaskSensitiveData() {
	// Mask API Key
	if a.APIKeyEncrypted != "" && len(a.APIKeyEncrypted) > 8 {
		prefix := a.APIKeyEncrypted[:4]
		suffix := a.APIKeyEncrypted[len(a.APIKeyEncrypted)-4:]
		a.APIKeyEncrypted = prefix + "****" + suffix
	}

	// Mask OAuth Data
	if a.OAuthDataEncrypted != "" {
		a.OAuthDataEncrypted = "[ENCRYPTED]"
	}
}

// AccountFilter defines query filter for listing accounts.
type AccountFilter struct {
	Page     int32           // Page number (starts from 1)
	PageSize int32           // Page size (1-100)
	Provider AccountProvider // Filter by provider (optional)
	Status   AccountStatus   // Filter by status (optional)
}

// AccountRepo implements biz.AccountRepo interface.
// Following Kratos v2 DDD architecture, interface is defined in biz layer.
type AccountRepo struct {
	data   *Data
	db     *gorm.DB
	cache  CacheClient
	logger *log.Helper
}

// NewAccountRepo creates a new account repository.
func NewAccountRepo(data *Data, db *gorm.DB, logger log.Logger) *AccountRepo {
	return &AccountRepo{
		data:   data,
		db:     db,
		cache:  data.GetCache(),
		logger: log.NewHelper(logger),
	}
}

// CreateAccount creates a new account in the database.
// Returns classified database errors for better error handling in upper layers.
func (r *AccountRepo) CreateAccount(ctx context.Context, account *Account) error {
	if err := r.db.WithContext(ctx).Create(account).Error; err != nil {
		// Classify the database error for better error handling
		dbErr := pkgerrors.ClassifyDBError(err)

		// Log with appropriate level based on error type
		switch dbErr.Type {
		case pkgerrors.ErrorTypeDuplicateKey:
			r.logger.Warnw("duplicate account name",
				"name", account.Name,
				"provider", account.Provider,
				"error", dbErr.Error())
		case pkgerrors.ErrorTypeInvalidJSON:
			r.logger.Errorw("invalid JSON in account metadata",
				"name", account.Name,
				"metadata", account.Metadata,
				"error", dbErr.Error())
		case pkgerrors.ErrorTypeConnectionError:
			r.logger.Errorw("database connection error",
				"error", dbErr.Error())
		default:
			r.logger.Errorw("failed to create account",
				"name", account.Name,
				"error", dbErr.Error())
		}

		// Return the classified error
		return dbErr
	}

	r.logger.Infow("account created", "id", account.ID, "name", account.Name, "provider", account.Provider)
	return nil
}

// GetAccount retrieves an account by ID with caching.
// Cache key: "account:{id}", TTL: 5 minutes
func (r *AccountRepo) GetAccount(ctx context.Context, id int64) (*Account, error) {
	cacheKey := fmt.Sprintf("account:%d", id)

	// Try to get from cache first
	var cachedAccount Account
	if err := r.cache.Get(ctx, cacheKey, &cachedAccount); err == nil {
		r.logger.Debugw("account cache hit", "id", id)
		return &cachedAccount, nil
	}

	// Cache miss, query from database
	var account Account
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("account not found: id=%d", id)
		}
		r.logger.Errorf("failed to get account: %v", err)
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Store in cache (5 minutes TTL)
	if err := r.cache.Set(ctx, cacheKey, &account, 5*time.Minute); err != nil {
		r.logger.Warnw("failed to cache account", "id", id, "error", err)
		// Cache failure doesn't affect the operation
	}

	r.logger.Debugw("account fetched from database", "id", id)
	return &account, nil
}

// ListAccounts retrieves accounts with pagination and filters.
func (r *AccountRepo) ListAccounts(ctx context.Context, filter *AccountFilter) ([]*Account, int32, error) {
	if filter == nil {
		filter = &AccountFilter{Page: 1, PageSize: 20}
	}

	// Set defaults
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	// Build query with soft delete filter (status != inactive)
	query := r.db.WithContext(ctx).Model(&Account{})

	// Apply filters
	if filter.Provider != "" {
		query = query.Where("provider = ?", filter.Provider)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	} else {
		// Default: exclude inactive accounts (soft delete)
		query = query.Where("status != ?", StatusInactive)
	}

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		r.logger.Errorf("failed to count accounts: %v", err)
		return nil, 0, fmt.Errorf("failed to count accounts: %w", err)
	}

	// Fetch paginated accounts
	var accounts []*Account
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Offset(int(offset)).Limit(int(filter.PageSize)).
		Order("created_at DESC").
		Find(&accounts).Error; err != nil {
		r.logger.Errorf("failed to list accounts: %v", err)
		return nil, 0, fmt.Errorf("failed to list accounts: %w", err)
	}

	r.logger.Debugw("accounts listed", "count", len(accounts), "total", total, "page", filter.Page)

	// Safe conversion of int64 to int32 with overflow check
	if total > 2147483647 { // max int32
		return accounts, 2147483647, nil
	}
	return accounts, int32(total), nil // #nosec G115 -- safe conversion with overflow check
}

// UpdateAccount updates an account and clears its cache.
func (r *AccountRepo) UpdateAccount(ctx context.Context, account *Account) error {
	account.UpdatedAt = time.Now()

	if err := r.db.WithContext(ctx).Save(account).Error; err != nil {
		r.logger.Errorf("failed to update account: %v", err)
		return fmt.Errorf("failed to update account: %w", err)
	}

	// Clear cache
	cacheKey := fmt.Sprintf("account:%d", account.ID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warnw("failed to delete account cache", "id", account.ID, "error", err)
	}

	r.logger.Infow("account updated", "id", account.ID, "name", account.Name)
	return nil
}

// DeleteAccount performs soft delete (sets status to INACTIVE) and clears cache.
func (r *AccountRepo) DeleteAccount(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     StatusInactive,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		r.logger.Errorf("failed to delete account: %v", result.Error)
		return fmt.Errorf("failed to delete account: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("account not found: id=%d", id)
	}

	// Clear cache
	cacheKey := fmt.Sprintf("account:%d", id)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warnw("failed to delete account cache", "id", id, "error", err)
	}

	r.logger.Infow("account deleted (soft)", "id", id)
	return nil
}

// MaskAPIKey masks API key for display (show first 4 + last 4 characters).
func MaskAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	if len(apiKey) <= 8 {
		return strings.Repeat("*", len(apiKey))
	}
	prefix := apiKey[:4]
	suffix := apiKey[len(apiKey)-4:]
	return prefix + "****" + suffix
}

// ValidateMetadataJSON validates if metadata is valid JSON.
// Empty string is NOT allowed - use NULL (nil pointer) instead for database storage.
func ValidateMetadataJSON(metadata string) error {
	if metadata == "" {
		return fmt.Errorf("metadata cannot be empty string, use null (nil pointer) or valid JSON")
	}
	var js json.RawMessage
	if err := json.Unmarshal([]byte(metadata), &js); err != nil {
		return fmt.Errorf("invalid JSON metadata: %w", err)
	}
	return nil
}

// ListExpiringAccounts 查询即将过期的 Claude 账户
// expiryThreshold: 过期时间阈值（如 time.Now().Add(10 * time.Minute)）
// 返回 oauth_expires_at <= expiryThreshold 的 active 状态 Claude 账户
func (r *AccountRepo) ListExpiringAccounts(ctx context.Context, expiryThreshold time.Time) ([]*Account, error) {
	var accounts []*Account

	// SQL: WHERE provider IN ('claude-official', 'claude-console')
	//      AND status = 'active'
	//      AND oauth_expires_at IS NOT NULL
	//      AND oauth_expires_at <= ?
	//      ORDER BY oauth_expires_at ASC
	err := r.db.WithContext(ctx).
		Where("provider IN (?, ?)", ProviderClaudeOfficial, ProviderClaudeConsole).
		Where("status = ?", StatusActive).
		Where("oauth_expires_at IS NOT NULL").
		Where("oauth_expires_at <= ?", expiryThreshold).
		Order("oauth_expires_at ASC").
		Find(&accounts).Error

	if err != nil {
		r.logger.Errorf("failed to list expiring accounts: %v", err)
		return nil, fmt.Errorf("failed to list expiring accounts: %w", err)
	}

	r.logger.Infow("expiring accounts listed", "count", len(accounts), "threshold", expiryThreshold)
	return accounts, nil
}

// UpdateOAuthData 更新账户的 OAuth 数据和过期时间
// accountID: 账户 ID
// oauthData: 加密后的 OAuth 数据（Base64 编码）
// expiresAt: OAuth Token 过期时间
func (r *AccountRepo) UpdateOAuthData(ctx context.Context, accountID int64, oauthData string, expiresAt time.Time) error {
	updates := map[string]interface{}{
		"oauth_data_encrypted": oauthData,
		"oauth_expires_at":     expiresAt,
		"updated_at":           time.Now(),
	}

	result := r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", accountID).
		Updates(updates)

	if result.Error != nil {
		r.logger.Errorf("failed to update OAuth data: %v", result.Error)
		return fmt.Errorf("failed to update OAuth data: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("account not found: id=%d", accountID)
	}

	// Clear cache
	cacheKey := fmt.Sprintf("account:%d", accountID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warnw("failed to delete account cache after OAuth update", "id", accountID, "error", err)
	}

	r.logger.Infow("OAuth data updated", "account_id", accountID, "expires_at", expiresAt)
	return nil
}

// UpdateHealthScore 更新账户的健康分数
// accountID: 账户 ID
// score: 新的健康分数（0-100）
// 使用 GREATEST(0, LEAST(100, ?)) 确保分数在 [0, 100] 范围内
func (r *AccountRepo) UpdateHealthScore(ctx context.Context, accountID int64, score int) error {
	// SQL: UPDATE api_accounts
	//      SET health_score = GREATEST(0, LEAST(100, ?)),
	//          updated_at = NOW()
	//      WHERE id = ?
	result := r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", accountID).
		Updates(map[string]interface{}{
			"health_score": gorm.Expr("GREATEST(0, LEAST(100, ?))", score),
			"updated_at":   time.Now(),
		})

	if result.Error != nil {
		r.logger.Errorf("failed to update health score: %v", result.Error)
		return fmt.Errorf("failed to update health score: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("account not found: id=%d", accountID)
	}

	// Clear cache
	cacheKey := fmt.Sprintf("account:%d", accountID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warnw("failed to delete account cache after health score update", "id", accountID, "error", err)
	}

	r.logger.Infow("health score updated", "account_id", accountID, "score", score)
	return nil
}

// UpdateAccountStatus 更新账户状态
// accountID: 账户 ID
// status: 新状态（active/inactive/error）
func (r *AccountRepo) UpdateAccountStatus(ctx context.Context, accountID int64, status AccountStatus) error {
	result := r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", accountID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		r.logger.Errorf("failed to update account status: %v", result.Error)
		return fmt.Errorf("failed to update account status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("account not found: id=%d", accountID)
	}

	// Clear cache
	cacheKey := fmt.Sprintf("account:%d", accountID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warnw("failed to delete account cache after status update", "id", accountID, "error", err)
	}

	r.logger.Infow("account status updated", "account_id", accountID, "status", status)
	return nil
}

// ListAccountsByProvider 查询指定 Provider 类型和状态的所有账户
// provider: Provider 类型（如 ProviderOpenAIResponses）
// status: 账户状态（如 StatusActive）
// 返回符合条件的账户列表（按 ID 升序排列）
func (r *AccountRepo) ListAccountsByProvider(ctx context.Context, provider AccountProvider, status AccountStatus) ([]*Account, error) {
	var accounts []*Account

	// SQL: SELECT * FROM api_accounts
	//      WHERE provider = ?
	//      AND status = ?
	//      ORDER BY id ASC
	err := r.db.WithContext(ctx).
		Where("provider = ?", provider).
		Where("status = ?", status).
		Order("id ASC").
		Find(&accounts).Error

	if err != nil {
		r.logger.Errorf("failed to list accounts by provider: %v", err)
		return nil, fmt.Errorf("failed to list accounts by provider: %w", err)
	}

	r.logger.Infow("accounts listed by provider", "provider", provider, "status", status, "count", len(accounts))
	return accounts, nil
}

// ListCodexCLIAccountsNeedingRefresh 查询需要刷新 token 的 Codex CLI 账户
// 查询条件：provider='codex-cli' AND status='active' AND token_expires_at < now() + 5分钟
func (r *AccountRepo) ListCodexCLIAccountsNeedingRefresh(ctx context.Context) ([]*Account, error) {
	var accounts []*Account

	// Token 即将在 5 分钟内过期
	threshold := time.Now().Add(5 * time.Minute)

	err := r.db.WithContext(ctx).
		Where("provider = ? AND status = ? AND token_expires_at < ?",
			ProviderCodexCLI, StatusActive, threshold).
		Order("token_expires_at ASC").
		Find(&accounts).Error

	if err != nil {
		r.logger.Errorf("failed to list Codex CLI accounts needing refresh: %v", err)
		return nil, fmt.Errorf("failed to list Codex CLI accounts needing refresh: %w", err)
	}

	r.logger.Infow("Codex CLI accounts needing refresh", "count", len(accounts), "threshold", threshold)
	return accounts, nil
}

// ParseMetadata parses metadata JSON string into AccountMetadata struct.
// Returns nil if metadata is nil or empty (no error).
// Story: 2-7 Account Metadata and Extended Configuration
func ParseMetadata(metadataPtr *string) (*metadata.AccountMetadata, error) {
	if metadataPtr == nil || *metadataPtr == "" {
		return &metadata.AccountMetadata{}, nil
	}

	return metadata.Parse(*metadataPtr)
}

// ListAccountsByTags queries accounts that match ALL specified tags (AND logic).
// Uses JSON_CONTAINS to filter accounts by tags in metadata JSON.
// Returns accounts ordered by health_score DESC, id ASC.
// Story: 2-7 Account Metadata and Extended Configuration
func (r *AccountRepo) ListAccountsByTags(ctx context.Context, tags []string, limit, offset int) ([]*Account, error) {
	if len(tags) == 0 {
		// No tags specified, return empty list (not all accounts)
		// Caller should use ListAccounts instead for unfiltered queries
		return []*Account{}, nil
	}

	var accounts []*Account

	// Build query: start with base WHERE clause
	query := r.db.WithContext(ctx).Where("status = ?", StatusActive)

	// Add JSON_CONTAINS condition for each tag (AND logic)
	// SQL: WHERE JSON_CONTAINS(metadata->'$.tags', '["tag1"]')
	//      AND JSON_CONTAINS(metadata->'$.tags', '["tag2"]')
	for _, tag := range tags {
		// JSON array format: ["tag"]
		tagJSON := fmt.Sprintf(`["%s"]`, tag)
		query = query.Where("JSON_CONTAINS(metadata->'$.tags', ?)", tagJSON)
	}

	// Apply pagination and ordering
	err := query.
		Order("health_score DESC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&accounts).Error

	if err != nil {
		r.logger.Errorf("failed to list accounts by tags: %v", err)
		return nil, fmt.Errorf("failed to list accounts by tags: %w", err)
	}

	r.logger.Infow("accounts listed by tags",
		"tags", tags,
		"count", len(accounts),
		"limit", limit,
		"offset", offset)

	return accounts, nil
}
