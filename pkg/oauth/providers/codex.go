package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"QuotaLane/internal/data"
	"QuotaLane/pkg/oauth"
	"QuotaLane/pkg/oauth/util"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	// Codex CLI OAuth 配置
	CodexAuthorizeURL  = "https://auth.openai.com/oauth/authorize"
	CodexTokenURL      = "https://auth.openai.com/oauth/token"
	CodexClientID      = "app_EMoamEEZ73f0CkXaXp7hrann"
	CodexRedirectURI   = "http://localhost:1455/auth/callback"
	CodexScopes        = "openid profile email offline_access"
	CodexScopesRefresh = "openid profile email" // 刷新时不包含 offline_access

	// PKCE 参数（⚠️ Codex 与 Claude 长度不同）
	CodexPKCESize     = 64 // 64 字节 → hex 编码（128 字符）
	CodexPKCEEncoding = "hex"

	// 超时设置
	CodexTokenTimeout = 10 * time.Minute
)

// CodexProvider Codex CLI OAuth Provider 实现
type CodexProvider struct {
	*BaseProvider // 嵌入 BaseProvider
}

// NewCodexProvider 创建 Codex Provider 实例
func NewCodexProvider(logger log.Logger) *CodexProvider {
	return &CodexProvider{
		BaseProvider: NewBaseProvider(CodexTokenTimeout, logger),
	}
}

// GenerateAuthURL 生成 Codex CLI OAuth 授权 URL
func (p *CodexProvider) GenerateAuthURL(ctx context.Context, params *oauth.OAuthParams) (*oauth.OAuthURLResponse, error) {
	// 生成 PKCE 参数（64 字节 hex）
	codeVerifier, err := util.GenerateCodeVerifier(CodexPKCESize, CodexPKCEEncoding)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	codeChallenge := util.GenerateCodeChallenge(codeVerifier)

	// 使用默认 Redirect URI
	redirectURI := params.RedirectURI
	if redirectURI == "" {
		redirectURI = CodexRedirectURI
	}

	// 使用默认 Scopes
	scopes := CodexScopes
	if len(params.Scopes) > 0 {
		scopes = strings.Join(params.Scopes, " ")
	}

	// 构建授权 URL（⚠️ 必须包含 Codex 特定参数）
	authURL := fmt.Sprintf("%s?%s",
		CodexAuthorizeURL,
		url.Values{
			"response_type":              {"code"},
			"client_id":                  {CodexClientID},
			"redirect_uri":               {redirectURI},
			"scope":                      {scopes},
			"code_challenge":             {codeChallenge},
			"code_challenge_method":      {"S256"},
			"state":                      {params.State},
			"id_token_add_organizations": {"true"},
			"codex_cli_simplified_flow":  {"true"},
		}.Encode(),
	)

	return &oauth.OAuthURLResponse{
		AuthURL:      authURL,
		CodeVerifier: codeVerifier,
	}, nil
}

// ExchangeCode 使用授权码交换 Token
func (p *CodexProvider) ExchangeCode(ctx context.Context, code string, session *oauth.OAuthSession) (*oauth.ExtendedTokenResponse, error) {
	redirectURI := session.RedirectURI
	if redirectURI == "" {
		redirectURI = CodexRedirectURI
	}

	// 构建请求体（application/x-www-form-urlencoded）
	formData := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     CodexClientID,
		"code":          strings.TrimSpace(code),
		"redirect_uri":  redirectURI,
		"code_verifier": session.CodeVerifier,
	}

	// 解析响应
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	// 使用 BaseProvider 发送表单请求
	if err := p.DoFormRequest(ctx, "POST", CodexTokenURL, nil, formData, &tokenResp, session.ProxyURL); err != nil {
		return nil, err
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("missing access_token in response")
	}

	// ⚠️ 解析 ID Token 提取 ChatGPT Account ID
	accountID, err := p.parseIDToken(tokenResp.IDToken)
	if err != nil {
		p.GetLogger().Warnf("Failed to parse ID token: %v", err)
	}

	return &oauth.ExtendedTokenResponse{
		AccessToken:  tokenResp.AccessToken,
		IDToken:      tokenResp.IDToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
		Scopes:       strings.Split(tokenResp.Scope, " "),
		AccountID:    accountID,
	}, nil
}

// RefreshToken 刷新 Token
func (p *CodexProvider) RefreshToken(ctx context.Context, refreshToken string, metadata *oauth.AccountMetadata) (*oauth.ExtendedTokenResponse, error) {
	// ⚠️ 刷新时使用不含 offline_access 的 scopes
	formData := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     CodexClientID,
		"refresh_token": refreshToken,
		"scope":         CodexScopesRefresh,
	}

	proxyURL := ""
	if metadata != nil {
		proxyURL = metadata.ProxyURL
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	// 使用 BaseProvider 发送表单请求
	if err := p.DoFormRequest(ctx, "POST", CodexTokenURL, nil, formData, &tokenResp, proxyURL); err != nil {
		return nil, err
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("missing access_token in response")
	}

	// ⚠️ Refresh Token 回退逻辑
	finalRefreshToken := tokenResp.RefreshToken
	if finalRefreshToken == "" {
		p.GetLogger().Warnf("OpenAI did not return new refresh_token, keeping the old one")
		finalRefreshToken = refreshToken
	}

	return &oauth.ExtendedTokenResponse{
		AccessToken:  tokenResp.AccessToken,
		IDToken:      tokenResp.IDToken,
		RefreshToken: finalRefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	}, nil
}

// ValidateToken 验证 Token
func (p *CodexProvider) ValidateToken(ctx context.Context, token string, metadata *oauth.AccountMetadata) error {
	return nil
}

// ProviderType 返回 Provider 类型
func (p *CodexProvider) ProviderType() data.AccountProvider {
	return data.ProviderCodexCLI
}

// parseIDToken 解析 ID Token 提取 ChatGPT Account ID
func (p *CodexProvider) parseIDToken(idToken string) (string, error) {
	if idToken == "" {
		return "", fmt.Errorf("empty ID token")
	}

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid ID token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to unmarshal claims: %w", err)
	}

	// 提取 ChatGPT Account ID
	accountID, ok := claims["https://api.openai.com/auth.chatgpt_account_id"].(string)
	if !ok || accountID == "" {
		return "", fmt.Errorf("missing chatgpt_account_id in ID token")
	}

	return accountID, nil
}
