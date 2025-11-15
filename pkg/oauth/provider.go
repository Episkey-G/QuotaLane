package oauth

import (
	"context"
	"time"

	"QuotaLane/internal/data"
)

// OAuthProvider 定义通用的 OAuth 授权接口
// 支持 PKCE Flow（Claude, Codex）和 Device Flow（未来扩展：Gemini, Droid）
// 新增平台步骤：实现此接口 → 注册到 OAuthManager → 添加 Proto 枚举
type OAuthProvider interface {
	// GenerateAuthURL 生成 OAuth 授权 URL
	// 返回授权 URL、PKCE/Device 参数等
	GenerateAuthURL(ctx context.Context, params *OAuthParams) (*OAuthURLResponse, error)

	// ExchangeCode 使用授权码交换 access_token 和 refresh_token
	// PKCE Flow: 使用 code_verifier 验证
	// Device Flow: 使用 device_code
	ExchangeCode(ctx context.Context, code string, session *OAuthSession) (*ExtendedTokenResponse, error)

	// RefreshToken 刷新 access_token
	// 使用 refresh_token 获取新的 access_token
	RefreshToken(ctx context.Context, refreshToken string, metadata *AccountMetadata) (*ExtendedTokenResponse, error)

	// ValidateToken 验证 token 有效性
	// 可选实现：调用上游 API 验证 token
	ValidateToken(ctx context.Context, token string, metadata *AccountMetadata) error

	// ProviderType 返回 Provider 类型
	ProviderType() data.AccountProvider
}

// OAuthParams OAuth 授权请求参数
type OAuthParams struct {
	ProxyURL    string
	RedirectURI string
	State       string
	Scopes      []string
	Metadata    map[string]string
}

// OAuthURLResponse OAuth 授权 URL 响应
type OAuthURLResponse struct {
	AuthURL         string
	SessionID       string
	State           string
	CodeVerifier    string
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresIn       int
	Interval        int
}

// OAuthSession Redis Session 数据结构
type OAuthSession struct {
	Provider     data.AccountProvider
	CodeVerifier string
	State        string
	DeviceCode   string
	ProxyURL     string
	RedirectURI  string
	CreatedAt    time.Time
	Metadata     map[string]string
}

// ExtendedTokenResponse Token 交换/刷新响应（扩展版，包含更多字段）
type ExtendedTokenResponse struct {
	AccessToken   string
	RefreshToken  string
	IDToken       string
	ExpiresIn     int
	Scopes        []string
	Organizations []map[string]interface{}
	AccountID     string
	Metadata      map[string]interface{}
	Provider      data.AccountProvider
}

// AccountMetadata 账户元数据
type AccountMetadata struct {
	ProxyURL    string
	BaseAPI     string
	Region      string
	RedirectURI string
	Extra       map[string]interface{}
}
