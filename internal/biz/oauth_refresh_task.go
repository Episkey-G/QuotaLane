package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"QuotaLane/internal/data"
	"QuotaLane/pkg/crypto"
	"QuotaLane/pkg/oauth"

	"github.com/go-kratos/kratos/v2/log"
)

// OAuthRefreshTask Token 自动刷新任务
type OAuthRefreshTask struct {
	repo         AccountRepo
	oauthManager *oauth.OAuthManager
	crypto       *crypto.AESCrypto
	logger       *log.Helper
}

// NewOAuthRefreshTask 创建 Token 刷新任务
func NewOAuthRefreshTask(
	repo AccountRepo,
	oauthManager *oauth.OAuthManager,
	crypto *crypto.AESCrypto,
	logger log.Logger,
) *OAuthRefreshTask {
	return &OAuthRefreshTask{
		repo:         repo,
		oauthManager: oauthManager,
		crypto:       crypto,
		logger:       log.NewHelper(logger),
	}
}

// RefreshExpiringTokens 刷新即将过期的 Token
// 执行策略：每 6 小时运行一次，刷新 2 小时内过期的 Token
// 优化说明：避免频繁刷新短期 token（如 Claude 8h），只在真正快过期时刷新
func (t *OAuthRefreshTask) RefreshExpiringTokens(ctx context.Context) error {
	// 查询 2 小时内过期的账户（优化：从 24h 改为 2h）
	expiryThreshold := time.Now().Add(2 * time.Hour)
	accounts, err := t.repo.ListExpiringAccounts(ctx, expiryThreshold)
	if err != nil {
		return fmt.Errorf("failed to list expiring accounts: %w", err)
	}

	if len(accounts) == 0 {
		t.logger.Info("No accounts need token refresh")
		return nil
	}

	t.logger.Infof("Found %d accounts with tokens expiring within 2 hours", len(accounts))

	// 刷新每个账户的 Token
	successCount := 0
	errorCount := 0

	for _, account := range accounts {
		if err := t.refreshAccountToken(ctx, account); err != nil {
			t.logger.Errorw("failed to refresh account token",
				"account_id", account.ID,
				"account_name", account.Name,
				"provider", account.Provider,
				"error", err)
			errorCount++
			continue
		}
		successCount++
	}

	t.logger.Infow("Token refresh task completed",
		"total", len(accounts),
		"success", successCount,
		"error", errorCount)

	return nil
}

// refreshAccountToken 刷新单个账户的 Token
func (t *OAuthRefreshTask) refreshAccountToken(ctx context.Context, account *data.Account) error {
	// 解密 OAuth 数据
	oauthDataJSON, err := t.crypto.Decrypt(account.OAuthDataEncrypted)
	if err != nil {
		return fmt.Errorf("failed to decrypt OAuth data: %w", err)
	}

	var oauthData map[string]interface{}
	if err := json.Unmarshal([]byte(oauthDataJSON), &oauthData); err != nil {
		return fmt.Errorf("failed to unmarshal OAuth data: %w", err)
	}

	// 提取 refresh_token_encrypted
	refreshTokenEncrypted, ok := oauthData["refresh_token_encrypted"].(string)
	if !ok || refreshTokenEncrypted == "" {
		return fmt.Errorf("refresh_token_encrypted not found in OAuth data")
	}

	// 解密 refresh_token
	refreshToken, err := t.crypto.Decrypt(refreshTokenEncrypted)
	if err != nil {
		return fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	// 构建 AccountMetadata（从 account.Metadata 中提取代理配置）
	metadata := &oauth.AccountMetadata{}
	if account.Metadata != nil {
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(*account.Metadata), &meta); err == nil {
			if proxyURL, ok := meta["proxy_url"].(string); ok {
				metadata.ProxyURL = proxyURL
			}
		}
	}

	// 调用 OAuthManager 刷新 Token
	tokenResp, err := t.oauthManager.RefreshToken(ctx, account.Provider, refreshToken, metadata)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	// 加密新的 access_token 和 refresh_token
	newAccessTokenEncrypted, err := t.crypto.Encrypt(tokenResp.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt new access token: %w", err)
	}

	newRefreshTokenEncrypted, err := t.crypto.Encrypt(tokenResp.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt new refresh token: %w", err)
	}

	// 更新 OAuth 数据
	oauthData["access_token_encrypted"] = newAccessTokenEncrypted
	oauthData["refresh_token_encrypted"] = newRefreshTokenEncrypted

	// 更新过期时间
	newExpiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	oauthData["expires_at"] = newExpiresAt.Format(time.RFC3339)

	// 如果有新的 ID Token，更新它
	if tokenResp.IDToken != "" {
		oauthData["id_token"] = tokenResp.IDToken
	}

	// 如果有新的 Scopes，更新它们
	if len(tokenResp.Scopes) > 0 {
		oauthData["scopes"] = tokenResp.Scopes
	}

	// 序列化更新后的 OAuth 数据
	updatedOAuthDataJSON, err := json.Marshal(oauthData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated OAuth data: %w", err)
	}

	// 加密整个 OAuth 数据
	updatedOAuthDataEncrypted, err := t.crypto.Encrypt(string(updatedOAuthDataJSON))
	if err != nil {
		return fmt.Errorf("failed to encrypt updated OAuth data: %w", err)
	}

	// 更新数据库
	if err := t.repo.UpdateOAuthData(ctx, account.ID, updatedOAuthDataEncrypted, newExpiresAt); err != nil {
		return fmt.Errorf("failed to update OAuth data in database: %w", err)
	}

	t.logger.Infow("successfully refreshed account token",
		"account_id", account.ID,
		"account_name", account.Name,
		"provider", account.Provider,
		"new_expires_at", newExpiresAt)

	return nil
}
