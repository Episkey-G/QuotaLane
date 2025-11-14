package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOAuthService(t *testing.T) {
	svc := NewOAuthService()
	assert.NotNil(t, svc)

	// 验证默认配置
	impl := svc.(*oauthService)
	assert.Equal(t, ClaudeOAuthTokenURL, impl.endpoint)
	assert.Equal(t, DefaultTimeout, impl.timeout)
	assert.Equal(t, DefaultMaxRetries, impl.maxRetries)
}

func TestNewOAuthServiceWithConfig(t *testing.T) {
	endpoint := "https://custom.api.com/token"
	timeout := 60 * time.Second
	maxRetries := 5

	svc := NewOAuthServiceWithConfig(endpoint, timeout, maxRetries)
	assert.NotNil(t, svc)

	impl := svc.(*oauthService)
	assert.Equal(t, endpoint, impl.endpoint)
	assert.Equal(t, timeout, impl.timeout)
	assert.Equal(t, maxRetries, impl.maxRetries)
}

func TestRefreshToken_Success(t *testing.T) {
	// 创建 Mock HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法和头
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// 验证请求体
		var req RefreshTokenRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "refresh_token", req.GrantType)
		assert.Equal(t, "test_refresh_token", req.RefreshToken)

		// 返回成功响应
		resp := TokenResponse{
			AccessToken:  "new_access_token",
			RefreshToken: "new_refresh_token",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 创建服务
	svc := NewOAuthServiceWithConfig(server.URL, 10*time.Second, 3)

	// 测试刷新
	ctx := context.Background()
	resp, err := svc.RefreshToken(ctx, "test_refresh_token", "")

	// 验证结果
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "new_access_token", resp.AccessToken)
	assert.Equal(t, "new_refresh_token", resp.RefreshToken)
	assert.Equal(t, 3600, resp.ExpiresIn)
	assert.Equal(t, "Bearer", resp.TokenType)
}

func TestRefreshToken_EmptyRefreshToken(t *testing.T) {
	svc := NewOAuthService()
	ctx := context.Background()

	resp, err := svc.RefreshToken(ctx, "", "")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "refresh_token cannot be empty")
}

func TestRefreshToken_ClientError_NoRetry(t *testing.T) {
	// 401 错误不应重试
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid_grant", "error_description": "Invalid refresh token"}`))
	}))
	defer server.Close()

	svc := NewOAuthServiceWithConfig(server.URL, 10*time.Second, 3)
	ctx := context.Background()

	resp, err := svc.RefreshToken(ctx, "invalid_token", "")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "oauth error (HTTP 401)")
	assert.Equal(t, 1, attempts, "should not retry on 4xx errors")
}

func TestRefreshToken_ServerError_WithRetry(t *testing.T) {
	// 500 错误应重试
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// 前两次返回 500
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal_server_error"}`))
		} else {
			// 第三次成功
			resp := TokenResponse{
				AccessToken:  "new_access_token",
				RefreshToken: "new_refresh_token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	svc := NewOAuthServiceWithConfig(server.URL, 10*time.Second, 3)
	ctx := context.Background()

	resp, err := svc.RefreshToken(ctx, "test_token", "")

	// 第三次应该成功
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "new_access_token", resp.AccessToken)
	assert.Equal(t, 3, attempts, "should retry on 5xx errors")
}

func TestRefreshToken_AllRetriesExhausted(t *testing.T) {
	// 所有重试都失败
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "service_unavailable"}`))
	}))
	defer server.Close()

	svc := NewOAuthServiceWithConfig(server.URL, 10*time.Second, 3)
	ctx := context.Background()

	resp, err := svc.RefreshToken(ctx, "test_token", "")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "all retry attempts exhausted")
	assert.Equal(t, 3, attempts, "should retry maxRetries times")
}

func TestRefreshToken_ContextCanceled(t *testing.T) {
	// 模拟慢响应
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewOAuthServiceWithConfig(server.URL, 10*time.Second, 3)
	ctx, cancel := context.WithCancel(context.Background())

	// 立即取消 context
	cancel()

	resp, err := svc.RefreshToken(ctx, "test_token", "")

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestRefreshToken_InvalidResponse(t *testing.T) {
	tests := []struct {
		name     string
		response TokenResponse
		errMsg   string
	}{
		{
			name: "missing_access_token",
			response: TokenResponse{
				RefreshToken: "new_refresh_token",
				ExpiresIn:    3600,
			},
			errMsg: "missing access_token",
		},
		{
			name: "missing_refresh_token",
			response: TokenResponse{
				AccessToken: "new_access_token",
				ExpiresIn:   3600,
			},
			errMsg: "missing refresh_token",
		},
		{
			name: "invalid_expires_in",
			response: TokenResponse{
				AccessToken:  "new_access_token",
				RefreshToken: "new_refresh_token",
				ExpiresIn:    0,
			},
			errMsg: "invalid expires_in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			svc := NewOAuthServiceWithConfig(server.URL, 10*time.Second, 3)
			ctx := context.Background()

			resp, err := svc.RefreshToken(ctx, "test_token", "")

			assert.Error(t, err)
			assert.Nil(t, resp)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestRefreshToken_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token": "test", invalid json`))
	}))
	defer server.Close()

	svc := NewOAuthServiceWithConfig(server.URL, 10*time.Second, 3)
	ctx := context.Background()

	resp, err := svc.RefreshToken(ctx, "test_token", "")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestCreateHTTPClient_InvalidProxyURL(t *testing.T) {
	svc := &oauthService{
		endpoint:   ClaudeOAuthTokenURL,
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
	}

	_, err := svc.createHTTPClient("invalid://proxy:1080")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported proxy scheme")
}

func TestCreateHTTPClient_HTTPProxy(t *testing.T) {
	svc := &oauthService{
		endpoint:   ClaudeOAuthTokenURL,
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
	}

	client, err := svc.createHTTPClient("http://proxy.example.com:8080")

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestRetryBackoffs(t *testing.T) {
	// 验证退避时间配置
	assert.Len(t, RetryBackoffs, 3)
	assert.Equal(t, 1*time.Second, RetryBackoffs[0])
	assert.Equal(t, 2*time.Second, RetryBackoffs[1])
	assert.Equal(t, 4*time.Second, RetryBackoffs[2])
}

// 测试实际的重试延迟时间
func TestRefreshToken_RetryTiming(t *testing.T) {
	t.Skip("Skip timing-sensitive test in CI")

	attempts := 0
	startTime := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	svc := NewOAuthServiceWithConfig(server.URL, 10*time.Second, 3)
	ctx := context.Background()

	_, err := svc.RefreshToken(ctx, "test_token", "")

	elapsed := time.Since(startTime)
	assert.Error(t, err)
	assert.Equal(t, 3, attempts)

	// 验证总耗时：1s + 2s = 3s（第三次失败后不等待）
	assert.GreaterOrEqual(t, elapsed, 3*time.Second)
	assert.Less(t, elapsed, 5*time.Second)
}

// Benchmark 测试
func BenchmarkRefreshToken(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TokenResponse{
			AccessToken:  "new_access_token",
			RefreshToken: "new_refresh_token",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := NewOAuthServiceWithConfig(server.URL, 10*time.Second, 3)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.RefreshToken(ctx, "test_token", "")
	}
}
