package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateAPIKey_Success tests successful API key validation
func TestValidateAPIKey_Success(t *testing.T) {
	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法
		assert.Equal(t, "GET", r.Method)

		// 验证路径
		assert.Equal(t, "/v1/models", r.URL.Path)

		// 验证请求头
		assert.Equal(t, "Bearer sk-test-key", r.Header.Get("Authorization"))
		assert.Equal(t, UserAgent, r.Header.Get("User-Agent"))

		// 返回成功响应
		resp := ModelsResponse{
			Object: "list",
			Data: []struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				OwnedBy string `json:"owned_by"`
			}{
				{
					ID:      "gpt-3.5-turbo",
					Object:  "model",
					Created: 1677610602,
					OwnedBy: "openai",
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-test-key", "")

	// 验证结果
	assert.NoError(t, err)
}

// TestValidateAPIKey_InvalidAPIKey tests 401 Unauthorized response
func TestValidateAPIKey_InvalidAPIKey(t *testing.T) {
	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回 401 错误
		errResp := ErrorResponse{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Invalid authentication",
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			},
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(errResp)
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-invalid-key", "")

	// 验证结果
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API key (HTTP 401)")
	assert.Contains(t, err.Error(), "Invalid authentication")
}

// TestValidateAPIKey_RateLimited tests 429 rate limit with retry
func TestValidateAPIKey_RateLimited(t *testing.T) {
	callCount := 0

	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// 前 2 次返回 429，第 3 次返回成功
		if callCount < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
			return
		}

		// 第 3 次成功
		resp := ModelsResponse{
			Object: "list",
			Data: []struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				OwnedBy string `json:"owned_by"`
			}{},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证
	startTime := time.Now()
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-test-key", "")
	duration := time.Since(startTime)

	// 验证结果
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount, "should retry 2 times and succeed on 3rd attempt")
	// 验证退避时间（应该至少等待 1s + 2s = 3s）
	assert.GreaterOrEqual(t, duration, 3*time.Second, "should wait for backoff time")
}

// TestValidateAPIKey_ServerError tests 5xx server error with retry
func TestValidateAPIKey_ServerError(t *testing.T) {
	callCount := 0

	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// 前 2 次返回 500，第 3 次返回成功
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": {"message": "Internal server error"}}`))
			return
		}

		// 第 3 次成功
		resp := ModelsResponse{Object: "list", Data: []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		}{}}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-test-key", "")

	// 验证结果
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount, "should retry 2 times and succeed on 3rd attempt")
}

// TestValidateAPIKey_AllRetriesFail tests all retry attempts exhausted
func TestValidateAPIKey_AllRetriesFail(t *testing.T) {
	callCount := 0

	// Mock OpenAI server (always return 500)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": {"message": "Server error"}}`))
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-test-key", "")

	// 验证结果
	require.Error(t, err)
	assert.Equal(t, 3, callCount, "should attempt 3 times before failing")
	assert.Contains(t, err.Error(), "all retry attempts exhausted")
	assert.Contains(t, err.Error(), "server error (HTTP 500)")
}

// TestValidateAPIKey_EmptyBaseAPI tests empty baseAPI parameter
func TestValidateAPIKey_EmptyBaseAPI(t *testing.T) {
	service := NewOpenAIService()

	err := service.ValidateAPIKey(context.Background(), "", "sk-test-key", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "baseAPI cannot be empty")
}

// TestValidateAPIKey_EmptyAPIKey tests empty apiKey parameter
func TestValidateAPIKey_EmptyAPIKey(t *testing.T) {
	service := NewOpenAIService()

	err := service.ValidateAPIKey(context.Background(), "https://api.openai.com", "", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiKey cannot be empty")
}

// TestValidateAPIKey_BaseAPIWithTrailingSlash tests baseAPI normalization
func TestValidateAPIKey_BaseAPIWithTrailingSlash(t *testing.T) {
	requestedPath := ""

	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		resp := ModelsResponse{Object: "list", Data: []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		}{}}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证（baseAPI 带尾部斜杠）
	err := service.ValidateAPIKey(context.Background(), server.URL+"/", "sk-test-key", "")

	// 验证结果
	assert.NoError(t, err)
	assert.Equal(t, "/v1/models", requestedPath, "should strip trailing slash from baseAPI")
}

// TestValidateAPIKey_ContextCancellation tests context cancellation
func TestValidateAPIKey_ContextCancellation(t *testing.T) {
	// Mock OpenAI server (slow response)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())

	// 立即取消
	cancel()

	// 调用验证
	err := service.ValidateAPIKey(ctx, server.URL, "sk-test-key", "")

	// 验证结果
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestValidateAPIKey_Timeout tests request timeout
func TestValidateAPIKey_Timeout(t *testing.T) {
	// Mock OpenAI server (slow response)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Second) // 超过 15 秒超时
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 创建服务（使用短超时时间加速测试）
	service := NewOpenAIServiceWithConfig(1*time.Second, 1)

	// 调用验证
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-test-key", "")

	// 验证结果
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "timeout")
}

// TestValidateAPIKey_InvalidResponseFormat tests invalid JSON response
func TestValidateAPIKey_InvalidResponseFormat(t *testing.T) {
	// Mock OpenAI server (invalid JSON)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-test-key", "")

	// 验证结果
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid response format")
}

// TestValidateAPIKey_InvalidProxyURL tests invalid proxy URL
func TestValidateAPIKey_InvalidProxyURL(t *testing.T) {
	service := NewOpenAIService()

	err := service.ValidateAPIKey(context.Background(), "https://api.openai.com", "sk-test-key", "://invalid-proxy-url")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid proxy URL")
}

// TestValidateAPIKey_UnsupportedProxyScheme tests unsupported proxy scheme
func TestValidateAPIKey_UnsupportedProxyScheme(t *testing.T) {
	service := NewOpenAIService()

	err := service.ValidateAPIKey(context.Background(), "https://api.openai.com", "sk-test-key", "ftp://proxy.example.com:8080")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported proxy scheme")
}

// TestNewOpenAIService tests default service creation
func TestNewOpenAIService(t *testing.T) {
	service := NewOpenAIService()

	assert.NotNil(t, service)

	// 验证类型
	impl, ok := service.(*openAIService)
	require.True(t, ok, "should return *openAIService")

	// 验证默认值
	assert.Equal(t, DefaultTimeout, impl.timeout)
	assert.Equal(t, DefaultMaxRetries, impl.maxRetries)
}

// TestNewOpenAIServiceWithConfig tests custom service creation
func TestNewOpenAIServiceWithConfig(t *testing.T) {
	customTimeout := 30 * time.Second
	customMaxRetries := 5

	service := NewOpenAIServiceWithConfig(customTimeout, customMaxRetries)

	assert.NotNil(t, service)

	// 验证类型
	impl, ok := service.(*openAIService)
	require.True(t, ok, "should return *openAIService")

	// 验证自定义值
	assert.Equal(t, customTimeout, impl.timeout)
	assert.Equal(t, customMaxRetries, impl.maxRetries)
}

// TestValidateAPIKey_403Forbidden tests 403 Forbidden error (no retry)
func TestValidateAPIKey_403Forbidden(t *testing.T) {
	callCount := 0

	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": {"message": "Forbidden"}}`))
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-test-key", "")

	// 验证结果
	require.Error(t, err)
	assert.Equal(t, 1, callCount, "should not retry on 4xx errors (except 429)")
	assert.Contains(t, err.Error(), "client error (HTTP 403)")
}

// TestValidateAPIKey_RetryBackoffTiming tests retry backoff timing
func TestValidateAPIKey_RetryBackoffTiming(t *testing.T) {
	callTimes := []time.Time{}

	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callTimes = append(callTimes, time.Now())

		// 前 2 次返回 500，第 3 次成功
		if len(callTimes) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp := ModelsResponse{Object: "list", Data: []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		}{}}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-test-key", "")

	// 验证结果
	assert.NoError(t, err)
	require.Len(t, callTimes, 3, "should make 3 attempts")

	// 验证第一次和第二次之间的间隔（应该约为 1 秒）
	interval1 := callTimes[1].Sub(callTimes[0])
	assert.GreaterOrEqual(t, interval1, 1*time.Second, "first backoff should be ~1s")
	assert.LessOrEqual(t, interval1, 1500*time.Millisecond, "first backoff should be ~1s")

	// 验证第二次和第三次之间的间隔（应该约为 2 秒）
	interval2 := callTimes[2].Sub(callTimes[1])
	assert.GreaterOrEqual(t, interval2, 2*time.Second, "second backoff should be ~2s")
	assert.LessOrEqual(t, interval2, 2500*time.Millisecond, "second backoff should be ~2s")
}

// TestValidateAPIKey_UnexpectedStatusCode tests unexpected status code handling
func TestValidateAPIKey_UnexpectedStatusCode(t *testing.T) {
	callCount := 0

	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusTeapot) // 418 I'm a teapot
		_, _ = w.Write([]byte(`{"error": {"message": "I'm a teapot"}}`))
	}))
	defer server.Close()

	// 创建服务
	service := NewOpenAIService()

	// 调用验证
	err := service.ValidateAPIKey(context.Background(), server.URL, "sk-test-key", "")

	// 验证结果
	require.Error(t, err)
	assert.Equal(t, 1, callCount, "should not retry on unexpected 4xx")
	assert.Contains(t, err.Error(), "client error (HTTP 418)")
}

// TestCreateHTTPClient_SOCKS5WithAuth tests SOCKS5 proxy with authentication
func TestCreateHTTPClient_SOCKS5WithAuth(t *testing.T) {
	service := &openAIService{
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
	}

	// 创建 SOCKS5 客户端（带认证）
	client, err := service.createHTTPClient("socks5://user:pass@localhost:1080")

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, DefaultTimeout, client.Timeout)
}

// TestCreateHTTPClient_SOCKS5WithoutAuth tests SOCKS5 proxy without authentication
func TestCreateHTTPClient_SOCKS5WithoutAuth(t *testing.T) {
	service := &openAIService{
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
	}

	// 创建 SOCKS5 客户端（无认证）
	client, err := service.createHTTPClient("socks5://localhost:1080")

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

// TestCreateHTTPClient_SOCKS5DefaultPort tests SOCKS5 proxy with default port
func TestCreateHTTPClient_SOCKS5DefaultPort(t *testing.T) {
	service := &openAIService{
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
	}

	// 创建 SOCKS5 客户端（无端口，应该使用默认 1080）
	client, err := service.createHTTPClient("socks5://localhost")

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, client)
}
