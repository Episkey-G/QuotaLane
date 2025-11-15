package openai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OpenAI OAuth 配置常量
const (
	OAuthBaseURL     = "https://auth.openai.com"
	OAuthClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	OAuthRedirectURI = "http://localhost:1455/auth/callback"
	OAuthScope       = "openid profile email offline_access"
)

// PKCEParams PKCE 授权码流程参数
type PKCEParams struct {
	CodeVerifier  string
	CodeChallenge string
	State         string
}

// OAuthTokens OAuth token 响应
type OAuthTokens struct {
	AccessToken   string   `json:"access_token"`
	RefreshToken  string   `json:"refresh_token"`
	IDToken       string   `json:"id_token"`
	TokenType     string   `json:"token_type"`
	ExpiresIn     int64    `json:"expires_in"` // 秒
	Scope         string   `json:"scope"`
	Organizations []string `json:"-"` // 从 ID token 解析
}

// GeneratePKCE 生成 PKCE 参数（RFC 7636）
// 注意：使用 hex 编码而不是 base64url，以与 claude-relay-service 和 OpenAI 的实现保持一致
func GeneratePKCE() (*PKCEParams, error) {
	// 1. 生成 code_verifier（128 字符的 hex 字符串）
	// 使用 64 bytes 随机数据 → hex 编码 → 128 字符
	// 参考 claude-relay-service: crypto.randomBytes(64).toString('hex')
	verifierBytes := make([]byte, 64)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeVerifier := fmt.Sprintf("%x", verifierBytes) // hex 编码

	// 2. 生成 code_challenge = BASE64URL(SHA256(code_verifier))
	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	// 3. 生成随机 state（防止 CSRF 攻击）
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	state := fmt.Sprintf("%x", stateBytes) // hex 编码，与 claude-relay-service 一致

	// 打印 PKCE 参数详细信息（调试用）
	log.Printf("[DEBUG] ==================== PKCE Generation ====================")
	log.Printf("[DEBUG] Code Verifier: %s (length: %d)", codeVerifier, len(codeVerifier))
	log.Printf("[DEBUG] Code Challenge: %s (length: %d)", codeChallenge, len(codeChallenge))
	log.Printf("[DEBUG] State: %s (length: %d)", state, len(state))
	log.Printf("[DEBUG] =======================================================")

	return &PKCEParams{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		State:         state,
	}, nil
}

// GenerateAuthURL 生成 OAuth 授权 URL
func (s *openAIService) GenerateAuthURL(pkce *PKCEParams) string {
	params := url.Values{
		"response_type":              {"code"},
		"client_id":                  {OAuthClientID},
		"redirect_uri":               {OAuthRedirectURI},
		"scope":                      {OAuthScope},
		"code_challenge":             {pkce.CodeChallenge},
		"code_challenge_method":      {"S256"},
		"state":                      {pkce.State},
		"id_token_add_organizations": {"true"}, // 返回组织信息
		"codex_cli_simplified_flow":  {"true"}, // Codex CLI 简化流程
	}

	return fmt.Sprintf("%s/oauth/authorize?%s", OAuthBaseURL, params.Encode())
}

// ExchangeCode 交换授权码获取 token
func (s *openAIService) ExchangeCode(ctx context.Context, code string, codeVerifier string, proxyURL string) (*OAuthTokens, error) {
	if code == "" || codeVerifier == "" {
		return nil, fmt.Errorf("code and code_verifier are required")
	}

	// 解析 code 参数：支持完整的回调 URL 或纯 code 值
	originalCode := code
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "http://") || strings.HasPrefix(code, "https://") {
		// 情况 1: 完整的回调 URL（例如：http://localhost:1455/auth/callback?code=xxx&state=yyy）
		parsedURL, err := url.Parse(code)
		if err != nil {
			return nil, fmt.Errorf("invalid callback URL format: %w", err)
		}
		extractedCode := parsedURL.Query().Get("code")
		if extractedCode == "" {
			return nil, fmt.Errorf("callback URL does not contain 'code' parameter")
		}
		log.Printf("[DEBUG] Parsed code from URL: %s -> %s", originalCode, extractedCode)
		code = extractedCode
	}
	// 情况 2: 纯 code 值（例如：ac_xxxxx）- 直接使用

	// 打印 PKCE 参数详细信息（调试用）
	log.Printf("[DEBUG] ==================== Token Exchange Debug ====================")
	log.Printf("[DEBUG] Authorization Code: %s (length: %d)", code, len(code))
	log.Printf("[DEBUG] Code Verifier: %s (length: %d)", codeVerifier, len(codeVerifier))
	log.Printf("[DEBUG] Redirect URI: %s", OAuthRedirectURI)
	log.Printf("[DEBUG] Client ID: %s", OAuthClientID)
	log.Printf("[DEBUG] Proxy URL: %s", proxyURL)
	log.Printf("[DEBUG] ============================================================")

	// 准备 token 交换请求参数（按照 claude-relay-service 的顺序）
	// 注意：手动构建以确保参数顺序与 claude-relay-service 一致
	requestBody := fmt.Sprintf(
		"grant_type=authorization_code&code=%s&redirect_uri=%s&client_id=%s&code_verifier=%s",
		url.QueryEscape(code),
		url.QueryEscape(OAuthRedirectURI),
		url.QueryEscape(OAuthClientID),
		url.QueryEscape(codeVerifier),
	)

	tokenURL := fmt.Sprintf("%s/oauth/token", OAuthBaseURL)
	log.Printf("[DEBUG] Token URL: %s", tokenURL)
	log.Printf("[DEBUG] Request Body: %s", requestBody)

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// 配置 HTTP 客户端（复用现有代理逻辑）
	client, err := s.createHTTPClient(proxyURL, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 发送请求
	log.Printf("[DEBUG] Sending token exchange request...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[DEBUG] Request failed: %v", err)
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[DEBUG] Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("[DEBUG] Response Status: %d", resp.StatusCode)
	log.Printf("[DEBUG] Response Body: %s", string(body))

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// 解析 JSON 响应
	var tokens OAuthTokens
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// 验证必要字段
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		return nil, fmt.Errorf("incomplete token response: missing access_token or refresh_token")
	}

	return &tokens, nil
}

// RefreshToken 刷新 access token
func (s *openAIService) RefreshToken(ctx context.Context, refreshToken string, proxyURL string) (*OAuthTokens, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh_token is required")
	}

	// 准备刷新 token 请求参数
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {OAuthClientID},
	}

	tokenURL := fmt.Sprintf("%s/oauth/token", OAuthBaseURL)

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// 配置 HTTP 客户端
	client, err := s.createHTTPClient(proxyURL, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 发送请求（包含重试机制）
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d failed: %w", attempt, err)
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return nil, lastErr
		}
		defer resp.Body.Close()

		// 读取响应
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// 检查 HTTP 状态码
		if resp.StatusCode != http.StatusOK {
			// 400: invalid_grant（refresh token 已过期或被撤销）
			if resp.StatusCode == http.StatusBadRequest {
				return nil, fmt.Errorf("refresh token invalid or expired (HTTP 400): %s", string(body))
			}
			lastErr = fmt.Errorf("token refresh failed (HTTP %d): %s", resp.StatusCode, string(body))
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return nil, lastErr
		}

		// 解析 JSON 响应
		var tokens OAuthTokens
		if err := json.Unmarshal(body, &tokens); err != nil {
			return nil, fmt.Errorf("failed to parse token response: %w", err)
		}

		// 验证必要字段
		if tokens.AccessToken == "" {
			return nil, fmt.Errorf("incomplete token response: missing access_token")
		}

		// 注意：refresh token 响应可能不包含新的 refresh_token
		// 如果没有返回新的，应该继续使用旧的
		if tokens.RefreshToken == "" {
			tokens.RefreshToken = refreshToken
		}

		return &tokens, nil
	}

	return nil, fmt.Errorf("refresh token failed after 3 attempts: %w", lastErr)
}

// ValidateAccessToken 使用 access token 验证账户（调用 GET /v1/models）
// ⚠️ 注意：此方法不应该用于验证 OAuth access token！
// OpenAI OAuth token 无法访问 /v1/models 等 API 端点（永远返回 401）
// 此方法仅用于验证 API Key 类型的账户
// 对于 OAuth 账户，应该使用 ValidateIDToken 方法
func (s *openAIService) ValidateAccessToken(ctx context.Context, baseAPI string, accessToken string, proxyURL string) error {
	if baseAPI == "" || accessToken == "" {
		return fmt.Errorf("baseAPI and accessToken are required")
	}

	// 规范化 baseAPI
	baseAPI = strings.TrimSuffix(baseAPI, "/")

	// 构建验证端点
	endpoint := fmt.Sprintf("%s/v1/models", baseAPI)

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 OAuth Bearer token 认证头
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/json")

	// 配置 HTTP 客户端
	client, err := s.createHTTPClient(proxyURL, 15*time.Second)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 发送请求（包含重试机制）
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d failed: %w", attempt, err)
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return lastErr
		}
		defer resp.Body.Close()

		// 检查 HTTP 状态码
		switch resp.StatusCode {
		case http.StatusOK:
			// 验证成功
			return nil
		case http.StatusUnauthorized:
			// 401: access token 无效或已过期
			return fmt.Errorf("invalid or expired access token (HTTP 401)")
		case http.StatusForbidden:
			// 403: 没有权限
			return fmt.Errorf("access forbidden (HTTP 403)")
		case http.StatusTooManyRequests:
			// 429: 速率限制
			return fmt.Errorf("rate limited (HTTP 429)")
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
			// 5xx: 服务器错误，可以重试
			lastErr = fmt.Errorf("server error (HTTP %d)", resp.StatusCode)
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return lastErr
		default:
			// 其他错误
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("validation failed (HTTP %d): %s", resp.StatusCode, string(body))
		}
	}

	return fmt.Errorf("validation failed after 3 attempts: %w", lastErr)
}

// IDTokenClaims ID Token JWT payload 结构
type IDTokenClaims struct {
	Sub           string                 `json:"sub"`                         // Subject (user ID)
	Email         string                 `json:"email"`                       // User email
	EmailVerified bool                   `json:"email_verified"`              // Email verification status
	Name          string                 `json:"name"`                        // User name
	Exp           int64                  `json:"exp"`                         // Expiration time (Unix timestamp)
	Iat           int64                  `json:"iat"`                         // Issued at (Unix timestamp)
	Aud           []string               `json:"aud"`                         // Audience (client ID array)
	Iss           string                 `json:"iss"`                         // Issuer
	AuthClaims    map[string]interface{} `json:"https://api.openai.com/auth"` // OpenAI specific claims
}

// ValidateIDToken 验证 OpenAI OAuth ID Token
// 这是验证 OAuth 账户的正确方法（不依赖于 API 端点调用）
// 参考 claude-relay-service: src/routes/admin.js:7228-7248
func (s *openAIService) ValidateIDToken(idToken string) (*IDTokenClaims, error) {
	if idToken == "" {
		return nil, fmt.Errorf("idToken cannot be empty")
	}

	// 1. 解析 JWT（格式: header.payload.signature）
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid ID token format: expected 3 parts, got %d", len(parts))
	}

	// 2. 解码 payload（base64url 编码）
	// 注意：Go 的 base64.RawURLEncoding 对应 Node.js 的 base64url（无填充）
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode ID token payload: %w", err)
	}

	// 3. 解析 JSON payload
	var claims IDTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse ID token claims: %w", err)
	}

	// 4. 验证必要字段
	if claims.Sub == "" {
		return nil, fmt.Errorf("ID token missing 'sub' claim")
	}
	if len(claims.Aud) == 0 {
		return nil, fmt.Errorf("ID token missing 'aud' claim")
	}
	if claims.Iss == "" {
		return nil, fmt.Errorf("ID token missing 'iss' claim")
	}

	// 5. 验证 token 是否过期
	now := time.Now().Unix()
	if claims.Exp > 0 && now > claims.Exp {
		return nil, fmt.Errorf("ID token has expired (exp: %d, now: %d)", claims.Exp, now)
	}

	// 6. 验证 issuer（可选但推荐）
	expectedIssuer := "https://auth.openai.com/"
	if claims.Iss != expectedIssuer {
		log.Printf("Warning: ID token issuer mismatch: expected %s, got %s", expectedIssuer, claims.Iss)
	}

	// 7. 验证 audience（可选但推荐）
	// aud 是数组，检查是否包含我们的 client ID
	audValid := false
	for _, aud := range claims.Aud {
		if aud == OAuthClientID {
			audValid = true
			break
		}
	}
	if !audValid {
		log.Printf("Warning: ID token audience mismatch: expected %s in %v", OAuthClientID, claims.Aud)
	}

	// 注意：我们不验证签名，因为：
	// 1. token 是直接从 OpenAI token 端点获取的（已经通过 HTTPS 验证）
	// 2. 验证签名需要获取 JWKS（增加复杂度和网络请求）
	// 3. claude-relay-service 也没有验证签名
	// 4. 主要目的是验证 token 格式正确且未过期

	return &claims, nil
}
