package providers

import (
	"context"
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
	// ClaudeAuthorizeURL is the Claude OAuth authorization endpoint.
	ClaudeAuthorizeURL = "https://claude.ai/oauth/authorize"
	// ClaudeTokenURL is the Claude OAuth token endpoint.
	ClaudeTokenURL = "https://console.anthropic.com/v1/oauth/token"
	// ClaudeClientID is the Claude OAuth client ID.
	ClaudeClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	// ClaudeRedirectURI is the Claude OAuth redirect URI.
	ClaudeRedirectURI = "https://console.anthropic.com/oauth/code/callback"
	// ClaudeScopes is the default OAuth scopes for Claude.
	ClaudeScopes = "org:create_api_key user:profile user:inference"
	// ClaudeUserAgent is the User-Agent header for Claude requests.
	ClaudeUserAgent = "claude-cli/1.0.56 (external, cli)"

	// ClaudePKCESize is the PKCE code verifier size (32 bytes → ~43 chars base64url).
	ClaudePKCESize = 32
	// ClaudePKCEEncoding is the PKCE code verifier encoding method.
	ClaudePKCEEncoding = "base64url"

	// ClaudeTokenTimeout is the timeout for Claude token requests.
	ClaudeTokenTimeout = 10 * time.Minute
)

// ClaudeProvider Claude OAuth Provider 实现
type ClaudeProvider struct {
	*BaseProvider // 嵌入 BaseProvider
}

// NewClaudeProvider 创建 Claude Provider 实例
func NewClaudeProvider(logger log.Logger) *ClaudeProvider {
	return &ClaudeProvider{
		BaseProvider: NewBaseProvider(ClaudeTokenTimeout, logger),
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

	// 构建请求头（Claude 特定）
	headers := map[string]string{
		"User-Agent": ClaudeUserAgent,
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

	// 使用 BaseProvider 发送 JSON 请求
	if err := p.DoJSONRequest(ctx, "POST", ClaudeTokenURL, headers, reqBody, &tokenResp, session.ProxyURL); err != nil {
		return nil, err
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

	// 构建请求头（Claude 特定）
	headers := map[string]string{
		"User-Agent": ClaudeUserAgent,
	}

	// 解析响应 JSON
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	// 获取代理 URL
	proxyURL := ""
	if metadata != nil {
		proxyURL = metadata.ProxyURL
	}

	// 使用 BaseProvider 发送 JSON 请求
	if err := p.DoJSONRequest(ctx, "POST", ClaudeTokenURL, headers, reqBody, &tokenResp, proxyURL); err != nil {
		return nil, err
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
