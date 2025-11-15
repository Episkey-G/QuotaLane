// Package openai provides OpenAI API utility functions for QuotaLane.
// It includes API key validation with proxy support and retry mechanisms.
package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const (
	// DefaultTimeout 默认超时时间
	DefaultTimeout = 15 * time.Second

	// DefaultMaxRetries 默认最大重试次数
	DefaultMaxRetries = 3

	// UserAgent QuotaLane 的 User-Agent
	UserAgent = "QuotaLane/1.0"
)

var (
	// RetryBackoffs 重试退避时间（指数退避：1s, 2s, 4s）
	RetryBackoffs = []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
	}
)

// ModelsResponse OpenAI /v1/models 端点响应
type ModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
	Object string `json:"object"`
}

// ErrorResponse OpenAI 错误响应
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// OpenAIService OpenAI 服务接口
type OpenAIService interface {
	// API Key 验证
	ValidateAPIKey(ctx context.Context, baseAPI, apiKey, proxyURL string) error

	// OAuth 授权流程
	GenerateAuthURL(pkce *PKCEParams) string
	ExchangeCode(ctx context.Context, code string, codeVerifier string, proxyURL string) (*OAuthTokens, error)
	RefreshToken(ctx context.Context, refreshToken string, proxyURL string) (*OAuthTokens, error)

	// Token 验证
	ValidateAccessToken(ctx context.Context, baseAPI string, accessToken string, proxyURL string) error
	ValidateIDToken(idToken string) (*IDTokenClaims, error)
}

// openAIService OpenAI 服务实现
type openAIService struct {
	timeout    time.Duration
	maxRetries int
}

// NewOpenAIService 创建 OpenAI 服务
func NewOpenAIService() OpenAIService {
	return &openAIService{
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
	}
}

// NewOpenAIServiceWithConfig 创建带自定义配置的 OpenAI 服务
func NewOpenAIServiceWithConfig(timeout time.Duration, maxRetries int) OpenAIService {
	return &openAIService{
		timeout:    timeout,
		maxRetries: maxRetries,
	}
}

// ValidateAPIKey 验证 OpenAI API Key
// baseAPI: API 基础地址，如 "https://api.codex.openai.com"
// apiKey: OpenAI API Key（sk-... 格式）
// proxyURL: 代理 URL（可选），格式如 "socks5://user:pass@host:port" 或 "http://user:pass@host:port"
func (s *openAIService) ValidateAPIKey(ctx context.Context, baseAPI, apiKey, proxyURL string) error {
	if baseAPI == "" {
		return fmt.Errorf("baseAPI cannot be empty")
	}
	if apiKey == "" {
		return fmt.Errorf("apiKey cannot be empty")
	}

	// 规范化 Base API（去除尾部斜杠）
	baseAPI = strings.TrimSuffix(baseAPI, "/")

	// 构建健康检查端点 URL
	endpoint := fmt.Sprintf("%s/v1/models", baseAPI)

	// 创建 HTTP 客户端（支持代理）
	client, err := s.createHTTPClient(proxyURL, s.timeout)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 带重试的请求
	var lastErr error
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		// 如果是重试，先等待退避时间
		if attempt > 0 {
			backoff := RetryBackoffs[attempt-1]
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		// 创建请求
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("User-Agent", UserAgent)

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

		// 成功响应（API Key 有效）
		if resp.StatusCode == 200 {
			// 验证响应格式
			var modelsResp ModelsResponse
			if err := json.Unmarshal(body, &modelsResp); err != nil {
				return fmt.Errorf("invalid response format: %w", err)
			}
			return nil
		}

		// 401 Unauthorized（API Key 无效，不重试）
		if resp.StatusCode == 401 {
			var errResp ErrorResponse
			_ = json.Unmarshal(body, &errResp) // 尝试解析错误消息，失败也无妨
			errMsg := string(body)
			if errResp.Error.Message != "" {
				errMsg = errResp.Error.Message
			}
			return fmt.Errorf("invalid API key (HTTP 401): %s", errMsg)
		}

		// 429 Too Many Requests（限流，可重试）
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("attempt %d: rate limited (HTTP 429): %s", attempt+1, string(body))
			continue
		}

		// 5xx 服务器错误（可重试）
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("attempt %d: server error (HTTP %d): %s", attempt+1, resp.StatusCode, string(body))
			continue
		}

		// 其他 4xx 客户端错误（不重试）
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("client error (HTTP %d): %s", resp.StatusCode, string(body))
		}

		// 其他状态码
		lastErr = fmt.Errorf("attempt %d: unexpected status code %d: %s", attempt+1, resp.StatusCode, string(body))
	}

	// 所有重试都失败
	return fmt.Errorf("all retry attempts exhausted: %w", lastErr)
}

// createHTTPClient 创建 HTTP 客户端（支持代理和自定义超时）
func (s *openAIService) createHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	// 如果未指定超时，使用默认超时
	if timeout == 0 {
		timeout = s.timeout
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // 增加 TCP 连接超时到 30 秒
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second, // 增加 TLS 握手超时到 30 秒（与整体请求超时一致）
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
		Timeout:   timeout,
	}, nil
}

// createSOCKS5Dialer 创建 SOCKS5 代理 dialer
func (s *openAIService) createSOCKS5Dialer(proxyURL string) (proxy.Dialer, error) {
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
