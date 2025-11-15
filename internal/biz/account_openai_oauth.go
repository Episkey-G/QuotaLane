package biz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"QuotaLane/internal/data"
	"QuotaLane/pkg/openai"
)

const (
	// OAuthSessionKeyPrefix Redis OAuth 会话前缀
	OAuthSessionKeyPrefix = "oauth_session:"

	// OAuthSessionTTL OAuth 会话 TTL（10 分钟）
	OAuthSessionTTL = 10 * time.Minute

	// TokenRefreshThreshold Token 刷新提前量（5 分钟）
	TokenRefreshThreshold = 5 * time.Minute
)

// OAuthSession OAuth 会话数据（存储在 Redis）
type OAuthSession struct {
	CodeVerifier string    `json:"code_verifier"`
	State        string    `json:"state"`
	ProxyURL     string    `json:"proxy_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// GenerateOpenAIAuthURL 生成 OpenAI OAuth 授权链接
// proxyURL: 代理 URL（可选）
// 返回: 授权 URL、会话 ID、state 参数、错误
func (uc *AccountUsecase) GenerateOpenAIAuthURL(ctx context.Context, proxyURL string) (string, string, string, error) {
	// 1. 生成 PKCE 参数
	pkce, err := openai.GeneratePKCE()
	if err != nil {
		uc.logger.Errorw("failed to generate PKCE", "error", err)
		return "", "", "", fmt.Errorf("failed to generate PKCE: %w", err)
	}

	// 2. 生成会话 ID（32 字节随机字符串）
	sessionIDBytes := make([]byte, 32)
	if _, err := rand.Read(sessionIDBytes); err != nil {
		uc.logger.Errorw("failed to generate session ID", "error", err)
		return "", "", "", fmt.Errorf("failed to generate session ID: %w", err)
	}
	sessionID := base64.RawURLEncoding.EncodeToString(sessionIDBytes)

	// 3. 保存会话数据到 Redis
	session := OAuthSession{
		CodeVerifier: pkce.CodeVerifier,
		State:        pkce.State,
		ProxyURL:     proxyURL,
		CreatedAt:    time.Now(),
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		uc.logger.Errorw("failed to marshal session", "error", err)
		return "", "", "", fmt.Errorf("failed to marshal session: %w", err)
	}

	sessionKey := fmt.Sprintf("%s%s", OAuthSessionKeyPrefix, sessionID)
	if err := uc.rdb.Set(ctx, sessionKey, sessionJSON, OAuthSessionTTL).Err(); err != nil {
		uc.logger.Errorw("failed to save session to Redis",
			"session_id", sessionID,
			"error", err)
		return "", "", "", fmt.Errorf("failed to save session: %w", err)
	}

	// 4. 生成授权 URL
	authURL := uc.openaiService.GenerateAuthURL(pkce)

	uc.logger.Infow("generated OpenAI auth URL",
		"session_id", sessionID,
		"proxy_url", proxyURL)

	return authURL, sessionID, pkce.State, nil
}

// ExchangeOpenAICode 交换 OpenAI OAuth 授权码并创建账户
// sessionID: 会话 ID
// code: 授权码
// name: 账户名称
// description: 账户描述（可选）
// rpmLimit: RPM 限制（可选）
// tpmLimit: TPM 限制（可选）
// metadata: 扩展元数据（JSON格式）（可选）
// 返回: 账户 ID、账户名称、账户状态、token 过期时间、错误
func (uc *AccountUsecase) ExchangeOpenAICode(
	ctx context.Context,
	sessionID string,
	code string,
	name string,
	description string,
	rpmLimit int32,
	tpmLimit int32,
	metadata string,
) (int64, string, data.AccountStatus, *time.Time, error) {
	// 1. 从 Redis 读取会话数据
	sessionKey := fmt.Sprintf("%s%s", OAuthSessionKeyPrefix, sessionID)
	sessionJSON, err := uc.rdb.Get(ctx, sessionKey).Result()
	if err != nil {
		uc.logger.Errorw("failed to get session from Redis",
			"session_id", sessionID,
			"error", err)
		return 0, "", "", nil, fmt.Errorf("session not found or expired: %w", err)
	}

	var session OAuthSession
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		uc.logger.Errorw("failed to unmarshal session",
			"session_id", sessionID,
			"error", err)
		return 0, "", "", nil, fmt.Errorf("invalid session data: %w", err)
	}

	// 2. 使用 code_verifier 交换授权码获取 token
	tokens, err := uc.openaiService.ExchangeCode(ctx, code, session.CodeVerifier, session.ProxyURL)
	if err != nil {
		uc.logger.Errorw("failed to exchange code",
			"session_id", sessionID,
			"error", err)
		return 0, "", "", nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// 验证必要字段
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		return 0, "", "", nil, fmt.Errorf("incomplete token response: missing access_token or refresh_token")
	}

	// 3. 加密存储 tokens
	accessTokenEncrypted, err := uc.crypto.Encrypt(tokens.AccessToken)
	if err != nil {
		uc.logger.Errorw("failed to encrypt access token", "error", err)
		return 0, "", "", nil, fmt.Errorf("failed to encrypt access token: %w", err)
	}

	refreshTokenEncrypted, err := uc.crypto.Encrypt(tokens.RefreshToken)
	if err != nil {
		uc.logger.Errorw("failed to encrypt refresh token", "error", err)
		return 0, "", "", nil, fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	var idTokenEncrypted string
	if tokens.IDToken != "" {
		idTokenEncrypted, err = uc.crypto.Encrypt(tokens.IDToken)
		if err != nil {
			uc.logger.Warnw("failed to encrypt ID token, continuing without it", "error", err)
		}
	}

	// 4. 序列化 organizations（如果有）
	var organizationsJSON string
	if len(tokens.Organizations) > 0 {
		orgBytes, err := json.Marshal(tokens.Organizations)
		if err != nil {
			uc.logger.Warnw("failed to marshal organizations", "error", err)
		} else {
			organizationsJSON = string(orgBytes)
		}
	}

	// 5. 【Fail Fast】使用 ID Token 验证账户（先验证再创建）
	// 注意：不使用 access token 验证，因为 OAuth access token 无法调用 /v1/models 等 API 端点
	// 参考 claude-relay-service: src/routes/admin.js:7228-7248
	claims, validationErr := uc.openaiService.ValidateIDToken(tokens.IDToken)
	if validationErr != nil {
		uc.logger.Errorw("ID token validation failed before creating account",
			"session_id", sessionID,
			"error", validationErr)
		return 0, "", "", nil, fmt.Errorf("ID token validation failed: %w", validationErr)
	}

	// 记录 ID Token 中的用户信息（用于调试）
	uc.logger.Infow("ID token validation successful",
		"session_id", sessionID,
		"user_sub", claims.Sub,
		"user_email", claims.Email,
		"email_verified", claims.EmailVerified,
		"token_exp", time.Unix(claims.Exp, 0))

	// 6. 计算 token 过期时间
	tokenExpiresAt := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	// 7. 处理 metadata：验证 JSON 格式，空字符串转为 nil（MySQL JSON 列要求）
	var metadataPtr *string
	if metadata != "" {
		// 验证 JSON 格式（Fail Fast：验证失败立即返回错误）
		if err := data.ValidateMetadataJSON(metadata); err != nil {
			uc.logger.Errorw("invalid metadata JSON",
				"session_id", sessionID,
				"metadata", metadata,
				"error", err)
			return 0, "", "", nil, fmt.Errorf("invalid metadata: %w", err)
		}
		// 有效 JSON -> 存储
		metadataPtr = &metadata
	}
	// metadata 为空字符串 -> metadataPtr 保持 nil (数据库 NULL)

	// 8. 创建账户（验证通过后才创建）
	account := &data.Account{
		Name:                  name,
		Description:           description,
		Provider:              data.ProviderCodexCLI,
		BaseAPI:               "https://api.openai.com", // Codex CLI 使用官方 OpenAI API
		AccessTokenEncrypted:  accessTokenEncrypted,
		RefreshTokenEncrypted: refreshTokenEncrypted,
		TokenExpiresAt:        &tokenExpiresAt,
		IDTokenEncrypted:      idTokenEncrypted,
		Organizations:         organizationsJSON,
		RpmLimit:              rpmLimit,
		TpmLimit:              tpmLimit,
		HealthScore:           100, // 初始健康分数为 100
		IsCircuitBroken:       false,
		Status:                data.StatusCreated, // 初始状态为 created，待验证后改为 active
		Metadata:              metadataPtr,
	}

	err = uc.repo.CreateAccount(ctx, account)
	if err != nil {
		// Database errors are already classified by the Data layer
		// Just pass them up with context
		uc.logger.Errorw("failed to create account",
			"name", name,
			"session_id", sessionID,
			"error", err)
		return 0, "", "", nil, fmt.Errorf("failed to create account: %w", err)
	}

	accountID := account.ID

	// 9. 账户创建成功，直接设置为 ACTIVE 状态（验证已在创建前完成）
	account.ID = accountID
	account.Status = data.StatusActive
	if err := uc.repo.UpdateAccount(ctx, account); err != nil {
		uc.logger.Warnw("failed to update account status to active",
			"account_id", accountID,
			"error", err)
	}

	// 10. 删除 Redis 会话数据
	if err := uc.rdb.Del(ctx, sessionKey).Err(); err != nil {
		uc.logger.Warnw("failed to delete session from Redis",
			"session_id", sessionID,
			"error", err)
	}

	uc.logger.Infow("Codex CLI account created successfully",
		"account_id", accountID,
		"account_name", name,
		"token_expires_at", tokenExpiresAt)

	return accountID, name, data.StatusActive, &tokenExpiresAt, nil
}

// ValidateCodexCLIAccount 验证 Codex CLI 账户（使用 access token）
// accountID: 账户 ID
// 返回: 验证成功返回 nil，失败返回错误
func (uc *AccountUsecase) ValidateCodexCLIAccount(ctx context.Context, accountID int64) error {
	// 1. 从 Repo 读取账户信息
	account, err := uc.repo.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// 验证 Provider 类型
	if account.Provider != data.ProviderCodexCLI {
		return fmt.Errorf("account is not Codex CLI type: provider=%s", account.Provider)
	}

	// 验证必填字段
	if account.AccessTokenEncrypted == "" {
		return fmt.Errorf("account access token is empty")
	}
	if account.BaseAPI == "" {
		return fmt.Errorf("account base API is empty")
	}

	// 2. 解密 access token
	accessToken, err := uc.crypto.Decrypt(account.AccessTokenEncrypted)
	if err != nil {
		uc.logger.Errorw("failed to decrypt access token",
			"account_id", accountID,
			"error", err)
		return fmt.Errorf("failed to decrypt access token: %w", err)
	}

	// 3. 提取代理配置（从 metadata JSON 读取 proxy_url）
	var proxyURL string
	if account.Metadata != nil && *account.Metadata != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(*account.Metadata), &metadata); err != nil {
			uc.logger.Warnw("failed to parse metadata JSON, skipping proxy",
				"account_id", accountID,
				"error", err)
		} else if proxy, ok := metadata["proxy_url"].(string); ok {
			proxyURL = proxy
		}
	}

	// 4. 调用 OpenAI 服务验证 access token
	err = uc.openaiService.ValidateAccessToken(ctx, account.BaseAPI, accessToken, proxyURL)

	if err != nil {
		// 验证失败：记录错误、减分、更新状态
		return uc.handleValidationFailure(ctx, account, err)
	}

	// 5. 验证成功：恢复健康分数、更新状态、清除错误记录
	return uc.handleValidationSuccess(ctx, account)
}

// RefreshCodexCLIToken 刷新单个 Codex CLI 账户的 access token
// accountID: 账户 ID
// 返回: 刷新成功返回新的 token 过期时间，失败返回错误
func (uc *AccountUsecase) RefreshCodexCLIToken(ctx context.Context, accountID int64) (*time.Time, error) {
	// 1. 从 Repo 读取账户信息
	account, err := uc.repo.GetAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// 验证 Provider 类型
	if account.Provider != data.ProviderCodexCLI {
		return nil, fmt.Errorf("account is not Codex CLI type: provider=%s", account.Provider)
	}

	// 验证必填字段
	if account.RefreshTokenEncrypted == "" {
		return nil, fmt.Errorf("account refresh token is empty")
	}

	// 2. 解密 refresh token
	refreshToken, err := uc.crypto.Decrypt(account.RefreshTokenEncrypted)
	if err != nil {
		uc.logger.Errorw("failed to decrypt refresh token",
			"account_id", accountID,
			"error", err)
		return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	// 3. 提取代理配置
	var proxyURL string
	if account.Metadata != nil && *account.Metadata != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(*account.Metadata), &metadata); err != nil {
			uc.logger.Warnw("failed to parse metadata JSON, skipping proxy",
				"account_id", accountID,
				"error", err)
		} else if proxy, ok := metadata["proxy_url"].(string); ok {
			proxyURL = proxy
		}
	}

	// 4. 调用 OpenAI 服务刷新 token
	tokens, err := uc.openaiService.RefreshToken(ctx, refreshToken, proxyURL)
	if err != nil {
		uc.logger.Errorw("failed to refresh token",
			"account_id", accountID,
			"error", err)

		// 刷新失败：记录错误、减分、更新状态
		return nil, uc.handleValidationFailure(ctx, account, err)
	}

	// 验证必要字段
	if tokens.AccessToken == "" {
		return nil, fmt.Errorf("incomplete token response: missing access_token")
	}

	// 5. 加密存储新的 tokens
	accessTokenEncrypted, err := uc.crypto.Encrypt(tokens.AccessToken)
	if err != nil {
		uc.logger.Errorw("failed to encrypt new access token", "error", err)
		return nil, fmt.Errorf("failed to encrypt access token: %w", err)
	}

	// refresh token 可能不会更新，如果没有返回新的，保持原有的
	refreshTokenEncrypted := account.RefreshTokenEncrypted
	if tokens.RefreshToken != "" && tokens.RefreshToken != refreshToken {
		refreshTokenEncrypted, err = uc.crypto.Encrypt(tokens.RefreshToken)
		if err != nil {
			uc.logger.Errorw("failed to encrypt new refresh token", "error", err)
			return nil, fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
	}

	// 6. 计算新的 token 过期时间
	tokenExpiresAt := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	// 7. 更新账户信息
	account.AccessTokenEncrypted = accessTokenEncrypted
	account.RefreshTokenEncrypted = refreshTokenEncrypted
	account.TokenExpiresAt = &tokenExpiresAt

	if err := uc.repo.UpdateAccount(ctx, account); err != nil {
		uc.logger.Errorw("failed to update account with new tokens",
			"account_id", accountID,
			"error", err)
		return nil, fmt.Errorf("failed to update account: %w", err)
	}

	// 8. 刷新成功：恢复健康分数（如果需要）
	if account.HealthScore < 100 {
		if err := uc.handleValidationSuccess(ctx, account); err != nil {
			uc.logger.Warnw("failed to handle validation success after refresh",
				"account_id", accountID,
				"error", err)
		}
	}

	uc.logger.Infow("Codex CLI token refreshed successfully",
		"account_id", accountID,
		"account_name", account.Name,
		"token_expires_at", tokenExpiresAt)

	return &tokenExpiresAt, nil
}

// RefreshCodexCLITokens 批量刷新所有即将过期的 Codex CLI 账户 token
// 定时任务调用此方法（每 5 分钟）
func (uc *AccountUsecase) RefreshCodexCLITokens(ctx context.Context) error {
	startTime := time.Now()

	// 查询所有需要刷新 token 的 Codex CLI 账户
	// 查询条件：provider='codex-cli' AND status='active' AND token_expires_at < now() + 5分钟
	accounts, err := uc.repo.ListCodexCLIAccountsNeedingRefresh(ctx)
	if err != nil {
		uc.logger.Errorw("failed to list Codex CLI accounts needing refresh", "error", err)
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	totalCount := len(accounts)
	if totalCount == 0 {
		uc.logger.Infow("no Codex CLI accounts need token refresh")
		return nil
	}

	uc.logger.Infow("starting Codex CLI token refresh",
		"total_accounts", totalCount)

	// 使用 semaphore 限制并发数为 5
	semaphore := make(chan struct{}, MaxConcurrentHealthCheck)
	results := make(chan error, totalCount)

	// 并发刷新所有账户
	for _, account := range accounts {
		semaphore <- struct{}{} // 获取信号量

		go func(acc *data.Account) {
			defer func() { <-semaphore }() // 释放信号量

			// 执行 token 刷新
			_, err := uc.RefreshCodexCLIToken(ctx, acc.ID)
			results <- err
		}(account)
	}

	// 等待所有刷新完成并统计结果
	successCount := 0
	failureCount := 0
	for i := 0; i < totalCount; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	duration := time.Since(startTime)

	uc.logger.Infow("Codex CLI token refresh completed",
		"total_accounts", totalCount,
		"success_count", successCount,
		"failure_count", failureCount,
		"duration_ms", duration.Milliseconds())

	return nil
}
