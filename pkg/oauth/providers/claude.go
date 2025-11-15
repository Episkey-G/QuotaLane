package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"QuotaLane/internal/data"
	"QuotaLane/pkg/oauth"
	"QuotaLane/pkg/oauth/util"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	// Claude OAuth 配置（从 Node.js 版本提取）
	ClaudeAuthorizeURL = "https://claude.ai/oauth/authorize"
	ClaudeTokenURL     = "https://console.anthropic.com/v1/oauth/token"
	ClaudeClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	ClaudeRedirectURI  = "https://console.anthropic.com/oauth/code/callback"
	ClaudeScopes       = "org:create_api_key user:profile user:inference"
	ClaudeUserAgent    = "claude-cli/1.0.56 (external, cli)"

	// PKCE 参数
	ClaudePKCESize     = 32 // 32 字节 → base64url 编码（约 43 字符）
	ClaudePKCEEncoding = "base64url"

	// 超时设置
	ClaudeTokenTimeout = 10 * time.Minute
)

// ClaudeProvider Claude OAuth Provider 实现
type ClaudeProvider struct {
	logger *log.Helper
}

// NewClaudeProvider 创建 Claude Provider 实例
func NewClaudeProvider(logger log.Logger) *ClaudeProvider {
	return &ClaudeProvider{
		logger: log.NewHelper(logger),
	}
}

// GenerateAuthURL 生成 Claude OAuth 授权 URL
func (p *ClaudeProvider) GenerateAuthURL(ctx context.Context, params *oauth.OAuthParams) (*oauth.OAuthURLResponse, error) {
	// 生成 PKCE 参数
	codeVerifier, err := util.GenerateCodeVerifier(ClaudePKCESize, ClaudePKCEEncoding)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	codeChallenge := util.GenerateCodeChallenge(codeVerifier)

	// 使用默认 Redirect URI（如果未提供）
	redirectURI := params.RedirectURI
	if redirectURI == "" {
		redirectURI = ClaudeRedirectURI
	}

	// 使用默认 Scopes（如果未提供）
	scopes := ClaudeScopes
	if len(params.Scopes) > 0 {
		scopes = strings.Join(params.Scopes, " ")
	}

	// 构建授权 URL（⚠️ 注意 code=true 参数必须存在）
	authURL := fmt.Sprintf("%s?%s",
		ClaudeAuthorizeURL,
		url.Values{
			"code":                  {"true"},
			"client_id":             {ClaudeClientID},
			"response_type":         {"code"},
			"redirect_uri":          {redirectURI},
			"scope":                 {scopes},
			"code_challenge":        {codeChallenge},
			"code_challenge_method": {"S256"},
			"state":                 {params.State},
		}.Encode(),
	)

	return &oauth.OAuthURLResponse{
		AuthURL:      authURL,
		CodeVerifier: codeVerifier,
	}, nil
}

// ExchangeCode 使用授权码交换 Token
func (p *ClaudeProvider) ExchangeCode(ctx context.Context, code string, session *oauth.OAuthSession) (*oauth.ExtendedTokenResponse, error) {
	// 使用默认 Redirect URI（如果 Session 未提供）
	redirectURI := session.RedirectURI
	if redirectURI == "" {
		redirectURI = ClaudeRedirectURI
	}

	// 构建请求体
	reqBody := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     ClaudeClientID,
		"code":          code,
		"redirect_uri":  redirectURI,
		"code_verifier": session.CodeVerifier,
		"state":         session.State,
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 创建 HTTP 客户端（支持代理）
	client, err := util.CreateHTTPClient(session.ProxyURL, ClaudeTokenTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ClaudeTokenURL, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ClaudeUserAgent)
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth error (HTTP %d): %s", resp.StatusCode, string(respData))
	}

	// 解析响应 JSON
	var tokenResp struct {
		AccessToken  string                 `json:"access_token"`
		RefreshToken string                 `json:"refresh_token"`
		ExpiresIn    int                    `json:"expires_in"`
		Scope        string                 `json:"scope"`
		Organization map[string]interface{} `json:"organization"`
		Account      map[string]interface{} `json:"account"`
	}

	if err := json.Unmarshal(respData, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 验证必填字段
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("missing access_token in response")
	}
	if tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("missing refresh_token in response")
	}

	// 构建返回结果
	organizations := []map[string]interface{}{}
	if tokenResp.Organization != nil {
		organizations = append(organizations, tokenResp.Organization)
	}

	return &oauth.ExtendedTokenResponse{
		AccessToken:   tokenResp.AccessToken,
		RefreshToken:  tokenResp.RefreshToken,
		ExpiresIn:     tokenResp.ExpiresIn,
		Scopes:        strings.Split(tokenResp.Scope, " "),
		Organizations: organizations,
		Metadata: map[string]interface{}{
			"account": tokenResp.Account,
		},
	}, nil
}

// RefreshToken 刷新 Token
func (p *ClaudeProvider) RefreshToken(ctx context.Context, refreshToken string, metadata *oauth.AccountMetadata) (*oauth.ExtendedTokenResponse, error) {
	// 构建请求体（⚠️ Claude 不需要 client_id）
	reqBody := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 创建 HTTP 客户端（支持代理）
	proxyURL := ""
	if metadata != nil {
		proxyURL = metadata.ProxyURL
	}

	client, err := util.CreateHTTPClient(proxyURL, ClaudeTokenTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ClaudeTokenURL, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ClaudeUserAgent)
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth error (HTTP %d): %s", resp.StatusCode, string(respData))
	}

	// 解析响应 JSON
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	if err := json.Unmarshal(respData, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 验证必填字段
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("missing access_token in response")
	}

	return &oauth.ExtendedTokenResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
		Scopes:       strings.Split(tokenResp.Scope, " "),
	}, nil
}

// ValidateToken 验证 Token 有效性
func (p *ClaudeProvider) ValidateToken(ctx context.Context, token string, metadata *oauth.AccountMetadata) error {
	// TODO: 实现 Token 验证逻辑
	return nil
}

// ProviderType 返回 Provider 类型
func (p *ClaudeProvider) ProviderType() data.AccountProvider {
	return data.ProviderClaudeOfficial
}
