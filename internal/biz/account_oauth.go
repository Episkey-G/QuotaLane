package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/data"
	"QuotaLane/pkg/oauth"
)

// GenerateOAuthURL 生成 OAuth 授权 URL
func (uc *AccountUsecase) GenerateOAuthURL(
	ctx context.Context,
	provider v1.AccountProvider,
	proxyURL string,
	redirectURI string,
	scopes []string,
	metadata map[string]string,
) (authURL string, sessionID string, state string, err error) {
	// 将 Proto Provider 转换为 Data Provider
	dataProvider, err := protoProviderToDataProvider(provider)
	if err != nil {
		return "", "", "", fmt.Errorf("unsupported provider: %w", err)
	}

	// 构建 OAuth 参数
	params := &oauth.OAuthParams{
		ProxyURL:    proxyURL,
		RedirectURI: redirectURI,
		Scopes:      scopes,
		Metadata:    metadata,
	}

	// 调用 OAuthManager 生成授权 URL
	resp, err := uc.oauthManager.GenerateAuthURL(ctx, dataProvider, params)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate auth URL: %w", err)
	}

	return resp.AuthURL, resp.SessionID, resp.State, nil
}

// ExchangeOAuthCode 交换 OAuth 授权码并创建账户
func (uc *AccountUsecase) ExchangeOAuthCode(
	ctx context.Context,
	sessionID string,
	code string,
	name string,
	description string,
	rpmLimit int32,
	tpmLimit int32,
	metadata map[string]string,
) (accountID int64, accountName string, status string, tokenExpiresAt *time.Time, err error) {
	// 调用 OAuthManager 交换授权码
	tokenResp, err := uc.oauthManager.ExchangeCode(ctx, sessionID, code)
	if err != nil {
		return 0, "", "", nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// 加密存储 access_token 和 refresh_token
	accessTokenEncrypted, err := uc.crypto.Encrypt(tokenResp.AccessToken)
	if err != nil {
		return 0, "", "", nil, fmt.Errorf("failed to encrypt access token: %w", err)
	}

	refreshTokenEncrypted, err := uc.crypto.Encrypt(tokenResp.RefreshToken)
	if err != nil {
		return 0, "", "", nil, fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	// 计算 token 过期时间
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// 构建 OAuth 数据（包含 ID Token、Organizations 等额外信息）
	oauthData := map[string]interface{}{
		"access_token_encrypted":  accessTokenEncrypted,
		"refresh_token_encrypted": refreshTokenEncrypted,
		"id_token":                tokenResp.IDToken,
		"scopes":                  tokenResp.Scopes,
		"organizations":           tokenResp.Organizations,
		"account_id":              tokenResp.AccountID, // Codex CLI ChatGPT Account ID
		"expires_at":              expiresAt.Format(time.RFC3339),
	}

	oauthDataJSON, err := json.Marshal(oauthData)
	if err != nil {
		return 0, "", "", nil, fmt.Errorf("failed to marshal OAuth data: %w", err)
	}

	// 加密整个 OAuth 数据
	oauthDataEncrypted, err := uc.crypto.Encrypt(string(oauthDataJSON))
	if err != nil {
		return 0, "", "", nil, fmt.Errorf("failed to encrypt OAuth data: %w", err)
	}

	// 序列化 metadata
	metadataJSON := ""
	if len(metadata) > 0 {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return 0, "", "", nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(metadataBytes)
	}

	// 准备 metadata 指针
	var metadataPtr *string
	if metadataJSON != "" {
		metadataPtr = &metadataJSON
	}

	// 创建账户记录
	account := &data.Account{
		Name:               name,
		Description:        description,
		Provider:           tokenResp.Provider,
		OAuthDataEncrypted: oauthDataEncrypted,
		TokenExpiresAt:     &expiresAt,
		Metadata:           metadataPtr,
		RpmLimit:           rpmLimit,
		TpmLimit:           tpmLimit,
		HealthScore:        100,
		Status:             data.StatusActive,
	}

	// 保存到数据库
	if err := uc.repo.CreateAccount(ctx, account); err != nil {
		return 0, "", "", nil, fmt.Errorf("failed to create account: %w", err)
	}

	uc.logger.Infof("OAuth account created successfully: id=%d, name=%s, provider=%s",
		account.ID, account.Name, account.Provider)

	return account.ID, account.Name, string(account.Status), &expiresAt, nil
}

// getProxyConfig 获取代理配置（三层优先级）
func (uc *AccountUsecase) getProxyConfig(accountMetadata string, requestProxy string) string {
	// 优先级 1: 请求级代理（RPC 参数）
	if requestProxy != "" {
		return requestProxy
	}

	// 优先级 2: 账户级代理（从 Account.Metadata 读取）
	if accountMetadata != "" {
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(accountMetadata), &meta); err == nil {
			if proxyURL, ok := meta["proxy_url"].(string); ok && proxyURL != "" {
				return proxyURL
			}
		}
	}

	// 优先级 3: 全局代理（环境变量）
	if httpProxy := os.Getenv("HTTP_PROXY"); httpProxy != "" {
		return httpProxy
	}
	if httpsProxy := os.Getenv("HTTPS_PROXY"); httpsProxy != "" {
		return httpsProxy
	}

	// 无代理
	return ""
}

// protoProviderToDataProvider 将 Proto Provider 转换为 Data Provider
func protoProviderToDataProvider(provider v1.AccountProvider) (data.AccountProvider, error) {
	switch provider {
	case v1.AccountProvider_CLAUDE_OFFICIAL:
		return data.ProviderClaudeOfficial, nil
	case v1.AccountProvider_CODEX_CLI:
		return data.ProviderCodexCLI, nil
	case v1.AccountProvider_GEMINI:
		return data.ProviderGemini, nil
	case v1.AccountProvider_DROID:
		return data.ProviderDroid, nil
	default:
		return "", fmt.Errorf("unsupported provider: %v", provider)
	}
}
