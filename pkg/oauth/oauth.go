// Package oauth provides OAuth 2.0 utility functions for QuotaLane.
// It includes token refresh capabilities with proxy support and retry mechanisms.
package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/wire"
	"golang.org/x/net/proxy"
)

// ProviderSet is oauth providers.
var ProviderSet = wire.NewSet(NewOAuthService)

const (
	// ClaudeOAuthTokenURL Claude OAuth Token 端点
	//nolint:gosec // G101: 这是一个公开的 API 端点 URL，不是凭据
	ClaudeOAuthTokenURL = "https://api.claude.ai/v1/oauth/token"

	// DefaultTimeout 默认超时时间
	DefaultTimeout = 30 * time.Second

	// DefaultMaxRetries 默认最大重试次数
	DefaultMaxRetries = 3
)

var (
	// baseURL for OAuth endpoint (can be overridden for testing)
	baseURL = "https://api.claude.ai"
)

var (
	// RetryBackoffs 重试退避时间（指数退避：1s, 2s, 4s）
	RetryBackoffs = []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
	}
)

// TokenResponse OAuth Token 刷新响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
}

// RefreshTokenRequest OAuth Token 刷新请求
type RefreshTokenRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

// OAuthService OAuth 服务接口
//
//nolint:revive // 保持 OAuthService 命名以明确表示这是 OAuth 服务
type OAuthService interface {
	RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*TokenResponse, error)
}

// oauthService OAuth 服务实现
type oauthService struct {
	endpoint   string
	timeout    time.Duration
	maxRetries int
}

// NewOAuthService 创建 OAuth 服务
func NewOAuthService() OAuthService {
	return &oauthService{
		endpoint:   baseURL + "/v1/oauth/token",
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
	}
}

// SetBaseURL sets the base URL for OAuth endpoint (for testing)
func SetBaseURL(url string) {
	baseURL = url
}

// NewOAuthServiceWithConfig 创建带自定义配置的 OAuth 服务
func NewOAuthServiceWithConfig(endpoint string, timeout time.Duration, maxRetries int) OAuthService {
	return &oauthService{
		endpoint:   endpoint,
		timeout:    timeout,
		maxRetries: maxRetries,
	}
}

// RefreshToken 刷新 OAuth Token
// refreshToken: 用于刷新的 refresh_token
// proxyURL: 代理 URL（可选），格式如 "socks5://user:pass@host:port" 或 "http://user:pass@host:port"
func (s *oauthService) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*TokenResponse, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh_token cannot be empty")
	}

	// 创建 HTTP 客户端（支持代理）
	client, err := s.createHTTPClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 构建请求体
	reqBody := RefreshTokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 带重试的请求
	var lastErr error
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		// 如果是重试，先等待退避时间
		if attempt > 0 {
			backoff := RetryBackoffs[attempt-1]
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// 创建请求
		req, err := http.NewRequestWithContext(ctx, "POST", s.endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		// 发送请求
		resp, err := client.Do(req)
		if err != nil {
			// 网络错误，可重试
			lastErr = fmt.Errorf("attempt %d: request failed: %w", attempt+1, err)
			continue
		}

		// 读取响应
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // 忽略 Close 错误，因为已经读取了 body
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: failed to read response: %w", attempt+1, err)
			continue
		}

		// 4xx 客户端错误不重试（如 401 无效 refresh_token）
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("oauth error (HTTP %d): %s", resp.StatusCode, string(body))
		}

		// 5xx 服务器错误可重试
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("attempt %d: server error (HTTP %d): %s", attempt+1, resp.StatusCode, string(body))
			continue
		}

		// 成功响应
		if resp.StatusCode == 200 {
			var tokenResp TokenResponse
			if err := json.Unmarshal(body, &tokenResp); err != nil {
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}

			// 验证响应字段
			if tokenResp.AccessToken == "" {
				return nil, fmt.Errorf("invalid response: missing access_token")
			}
			if tokenResp.RefreshToken == "" {
				return nil, fmt.Errorf("invalid response: missing refresh_token")
			}
			if tokenResp.ExpiresIn <= 0 {
				return nil, fmt.Errorf("invalid response: invalid expires_in")
			}

			return &tokenResp, nil
		}

		// 其他状态码
		lastErr = fmt.Errorf("attempt %d: unexpected status code %d: %s", attempt+1, resp.StatusCode, string(body))
	}

	// 所有重试都失败
	return nil, fmt.Errorf("all retry attempts exhausted: %w", lastErr)
}

// createHTTPClient 创建 HTTP 客户端（支持代理）
func (s *oauthService) createHTTPClient(proxyURL string) (*http.Client, error) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 配置代理
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}

		switch parsed.Scheme {
		case "socks5", "socks5h":
			// SOCKS5 代理
			dialer, err := s.createSOCKS5Dialer(proxyURL)
			if err != nil {
				return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}

		case "http", "https":
			// HTTP/HTTPS 代理
			transport.Proxy = func(req *http.Request) (*url.URL, error) {
				return parsed, nil
			}

		default:
			return nil, fmt.Errorf("unsupported proxy scheme: %s (supported: socks5, http, https)", parsed.Scheme)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   s.timeout,
	}, nil
}

// createSOCKS5Dialer 创建 SOCKS5 代理 dialer
func (s *oauthService) createSOCKS5Dialer(proxyURL string) (proxy.Dialer, error) {
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	var auth *proxy.Auth
	if parsed.User != nil {
		password, _ := parsed.User.Password()
		auth = &proxy.Auth{
			User:     parsed.User.Username(),
			Password: password,
		}
	}

	// 去掉 scheme 前缀
	host := parsed.Host
	if !strings.Contains(host, ":") {
		host += ":1080" // SOCKS5 默认端口
	}

	return proxy.SOCKS5("tcp", host, auth, proxy.Direct)
}
