package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"QuotaLane/internal/data"
	"QuotaLane/pkg/oauth/util"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

const (
	// SessionKeyPrefix Redis Session 键前缀
	SessionKeyPrefix = "oauth_session:"

	// SessionTTL Session 过期时间（10 分钟）
	SessionTTL = 10 * time.Minute
)

// OAuthManager OAuth 管理器
// 负责 Provider 注册、Session 管理、授权 URL 生成、Code 交换
type OAuthManager struct {
	providers map[data.AccountProvider]OAuthProvider
	redis     *redis.Client
	logger    *log.Helper
}

// NewOAuthManager 创建 OAuthManager 实例
func NewOAuthManager(redis *redis.Client, logger log.Logger) *OAuthManager {
	return &OAuthManager{
		providers: make(map[data.AccountProvider]OAuthProvider),
		redis:     redis,
		logger:    log.NewHelper(logger),
	}
}

// RegisterProvider 注册 OAuth Provider
func (m *OAuthManager) RegisterProvider(p OAuthProvider) {
	m.providers[p.ProviderType()] = p
	m.logger.Infof("Registered OAuth provider: %s", p.ProviderType())
}

// GenerateAuthURL 生成 OAuth 授权 URL
func (m *OAuthManager) GenerateAuthURL(ctx context.Context, provider data.AccountProvider, params *OAuthParams) (*OAuthURLResponse, error) {
	// 获取 Provider
	p, ok := m.providers[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported OAuth provider: %v", provider)
	}

	// 生成 Session ID
	sessionID, err := util.GenerateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	// 生成 State（如果未提供）
	if params.State == "" {
		state, err := util.GenerateState()
		if err != nil {
			return nil, fmt.Errorf("failed to generate state: %w", err)
		}
		params.State = state
	}

	// 调用 Provider 生成授权 URL
	resp, err := p.GenerateAuthURL(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("provider failed to generate auth URL: %w", err)
	}

	// 构建 OAuth Session
	session := &OAuthSession{
		Provider:     provider,
		CodeVerifier: resp.CodeVerifier,
		State:        params.State,
		DeviceCode:   resp.DeviceCode,
		ProxyURL:     params.ProxyURL,
		RedirectURI:  params.RedirectURI,
		CreatedAt:    time.Now(),
		Metadata:     params.Metadata,
	}

	// 保存 Session 到 Redis
	if err := m.SaveSession(ctx, sessionID, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// 填充 SessionID 并返回
	resp.SessionID = sessionID
	resp.State = params.State

	m.logger.Infof("Generated OAuth URL for provider %s, session_id=%s", provider, sessionID)
	return resp, nil
}

// ExchangeCode 使用授权码交换 Token
func (m *OAuthManager) ExchangeCode(ctx context.Context, sessionID, code string) (*ExtendedTokenResponse, error) {
	// 加载 Session
	session, err := m.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// 获取 Provider
	p, ok := m.providers[session.Provider]
	if !ok {
		return nil, fmt.Errorf("unsupported OAuth provider: %v", session.Provider)
	}

	// 调用 Provider 交换 Code
	tokenResp, err := p.ExchangeCode(ctx, code, session)
	if err != nil {
		return nil, fmt.Errorf("provider failed to exchange code: %w", err)
	}

	// 成功后删除 Session（防止重放攻击）
	if err := m.DeleteSession(ctx, sessionID); err != nil {
		m.logger.Warnf("Failed to delete session %s: %v", sessionID, err)
	}

	// 填充 Provider 类型
	tokenResp.Provider = session.Provider

	m.logger.Infof("Exchanged OAuth code for provider %s, session_id=%s", session.Provider, sessionID)
	return tokenResp, nil
}

// RefreshToken 刷新 Token
func (m *OAuthManager) RefreshToken(ctx context.Context, provider data.AccountProvider, refreshToken string, metadata *AccountMetadata) (*ExtendedTokenResponse, error) {
	// 获取 Provider
	p, ok := m.providers[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported OAuth provider: %v", provider)
	}

	// 调用 Provider 刷新 Token
	tokenResp, err := p.RefreshToken(ctx, refreshToken, metadata)
	if err != nil {
		return nil, fmt.Errorf("provider failed to refresh token: %w", err)
	}

	// 填充 Provider 类型
	tokenResp.Provider = provider

	m.logger.Infof("Refreshed OAuth token for provider %s", provider)
	return tokenResp, nil
}

// SaveSession 保存 Session 到 Redis
func (m *OAuthManager) SaveSession(ctx context.Context, sessionID string, session *OAuthSession) error {
	key := SessionKeyPrefix + sessionID

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := m.redis.Set(ctx, key, data, SessionTTL).Err(); err != nil {
		return fmt.Errorf("failed to save session to Redis: %w", err)
	}

	return nil
}

// LoadSession 从 Redis 加载 Session
func (m *OAuthManager) LoadSession(ctx context.Context, sessionID string) (*OAuthSession, error) {
	key := SessionKeyPrefix + sessionID

	data, err := m.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("session not found or expired: %s", sessionID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to load session from Redis: %w", err)
	}

	var session OAuthSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// DeleteSession 删除 Session
func (m *OAuthManager) DeleteSession(ctx context.Context, sessionID string) error {
	key := SessionKeyPrefix + sessionID

	if err := m.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session from Redis: %w", err)
	}

	return nil
}
