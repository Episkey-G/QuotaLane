package providers

import (
	"context"
	"fmt"
	"time"

	"QuotaLane/internal/data"
	"QuotaLane/pkg/oauth"
	"QuotaLane/pkg/openai"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	// OpenAI Responses 验证超时
	OpenAIResponsesTimeout = 30 * time.Second
)

// OpenAIResponsesProvider OpenAI Responses Provider 实现
// 注意：OpenAI Responses 不是 OAuth 服务，只支持 ValidateToken
type OpenAIResponsesProvider struct {
	*BaseProvider
	openaiService openai.OpenAIService
}

// NewOpenAIResponsesProvider 创建 OpenAI Responses Provider 实例
func NewOpenAIResponsesProvider(openaiService openai.OpenAIService, logger log.Logger) *OpenAIResponsesProvider {
	return &OpenAIResponsesProvider{
		BaseProvider:  NewBaseProvider(OpenAIResponsesTimeout, logger),
		openaiService: openaiService,
	}
}

// GenerateAuthURL OpenAI Responses 不支持 OAuth，返回错误
func (p *OpenAIResponsesProvider) GenerateAuthURL(ctx context.Context, params *oauth.OAuthParams) (*oauth.OAuthURLResponse, error) {
	return nil, fmt.Errorf("OpenAI Responses does not support OAuth authorization")
}

// ExchangeCode OpenAI Responses 不支持 OAuth，返回错误
func (p *OpenAIResponsesProvider) ExchangeCode(ctx context.Context, code string, session *oauth.OAuthSession) (*oauth.ExtendedTokenResponse, error) {
	return nil, fmt.Errorf("OpenAI Responses does not support OAuth code exchange")
}

// RefreshToken OpenAI Responses 不支持 OAuth，返回错误
func (p *OpenAIResponsesProvider) RefreshToken(ctx context.Context, refreshToken string, metadata *oauth.AccountMetadata) (*oauth.ExtendedTokenResponse, error) {
	return nil, fmt.Errorf("OpenAI Responses does not support OAuth token refresh")
}

// ValidateToken 验证 OpenAI Responses API Key
// 通过调用 OpenAI /v1/models 端点验证 API Key 有效性
func (p *OpenAIResponsesProvider) ValidateToken(ctx context.Context, token string, metadata *oauth.AccountMetadata) error {
	// 验证必填参数
	if token == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// 获取 Base API（必填）
	baseAPI := ""
	if metadata != nil {
		baseAPI = metadata.BaseAPI
	}
	if baseAPI == "" {
		return fmt.Errorf("base API is required for OpenAI Responses validation")
	}

	// 获取代理配置（可选）
	proxyURL := ""
	if metadata != nil {
		proxyURL = metadata.ProxyURL
	}

	// 调用 OpenAI 服务验证 API Key
	if err := p.openaiService.ValidateAPIKey(ctx, baseAPI, token, proxyURL); err != nil {
		return fmt.Errorf("API key validation failed: %w", err)
	}

	return nil
}

// ProviderType 返回 Provider 类型
func (p *OpenAIResponsesProvider) ProviderType() data.AccountProvider {
	return data.ProviderOpenAIResponses
}
