package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"QuotaLane/pkg/oauth/util"

	"github.com/go-kratos/kratos/v2/log"
)

// BaseProvider 提供通用的 OAuth Provider 功能
// 包含 HTTP 客户端管理、请求发送、重试逻辑等
type BaseProvider struct {
	logger  *log.Helper
	timeout time.Duration
}

// NewBaseProvider 创建 BaseProvider 实例
func NewBaseProvider(timeout time.Duration, logger log.Logger) *BaseProvider {
	return &BaseProvider{
		logger:  log.NewHelper(logger),
		timeout: timeout,
	}
}

// DoJSONRequest 发送 JSON 请求并解析响应
// method: HTTP 方法（GET, POST, PUT, DELETE）
// url: 请求 URL
// headers: 请求头（可为 nil）
// reqBody: 请求体（可为 nil）
// respBody: 响应体解析目标（可为 nil，表示不解析响应）
// proxyURL: 代理 URL（可为空）
func (b *BaseProvider) DoJSONRequest(
	ctx context.Context,
	method, url string,
	headers map[string]string,
	reqBody interface{},
	respBody interface{},
	proxyURL string,
) error {
	// 创建 HTTP 客户端
	client, err := util.CreateHTTPClient(proxyURL, b.timeout)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 序列化请求体
	var reqData []byte
	if reqBody != nil {
		reqData, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// 创建 HTTP 请求
	var req *http.Request
	if reqData != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(reqData))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置默认请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// 设置自定义请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OAuth error (HTTP %d): %s", resp.StatusCode, string(respData))
	}

	// 解析响应体
	if respBody != nil {
		if err := json.Unmarshal(respData, respBody); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// DoFormRequest 发送表单请求并解析响应
// method: HTTP 方法（GET, POST, PUT, DELETE）
// url: 请求 URL
// headers: 请求头（可为 nil）
// formData: 表单数据
// respBody: 响应体解析目标（可为 nil）
// proxyURL: 代理 URL（可为空）
func (b *BaseProvider) DoFormRequest(
	ctx context.Context,
	method, url string,
	headers map[string]string,
	formData map[string]string,
	respBody interface{},
	proxyURL string,
) error {
	// 创建 HTTP 客户端
	client, err := util.CreateHTTPClient(proxyURL, b.timeout)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 构建表单数据
	formValues := make(map[string][]string)
	for key, value := range formData {
		formValues[key] = []string{value}
	}

	// URL 编码表单数据
	encodedForm := ""
	for key, values := range formValues {
		for _, value := range values {
			if encodedForm != "" {
				encodedForm += "&"
			}
			encodedForm += key + "=" + value
		}
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBufferString(encodedForm))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置默认请求头
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// 设置自定义请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OAuth error (HTTP %d): %s", resp.StatusCode, string(respData))
	}

	// 解析响应体
	if respBody != nil {
		if err := json.Unmarshal(respData, respBody); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// GetTimeout 返回超时配置
func (b *BaseProvider) GetTimeout() time.Duration {
	return b.timeout
}

// GetLogger 返回 logger
func (b *BaseProvider) GetLogger() *log.Helper {
	return b.logger
}
