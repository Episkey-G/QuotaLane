package oauth

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"QuotaLane/internal/data"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements OAuthProvider for testing
type mockProvider struct {
	providerType data.AccountProvider
	authURL      string
	codeVerifier string
	tokenResp    *ExtendedTokenResponse
	err          error
}

func (m *mockProvider) GenerateAuthURL(ctx context.Context, params *OAuthParams) (*OAuthURLResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &OAuthURLResponse{
		AuthURL:      m.authURL,
		CodeVerifier: m.codeVerifier,
	}, nil
}

func (m *mockProvider) ExchangeCode(ctx context.Context, code string, session *OAuthSession) (*ExtendedTokenResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokenResp, nil
}

func (m *mockProvider) RefreshToken(ctx context.Context, refreshToken string, metadata *AccountMetadata) (*ExtendedTokenResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokenResp, nil
}

func (m *mockProvider) ValidateToken(ctx context.Context, token string, metadata *AccountMetadata) error {
	return m.err
}

func (m *mockProvider) ProviderType() data.AccountProvider {
	return m.providerType
}

// setupTestRedis creates a Redis client for testing
// Note: This requires a running Redis instance (can be started with docker-compose)
func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use test DB
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for testing, skipping: " + err.Error())
	}

	// Clean test DB
	client.FlushDB(ctx)

	t.Cleanup(func() {
		client.FlushDB(ctx)
		client.Close()
	})

	return client
}

func TestNewOAuthManager(t *testing.T) {
	rdb := setupTestRedis(t)
	logger := log.DefaultLogger

	manager := NewOAuthManager(rdb, logger)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.providers)
	assert.NotNil(t, manager.redis)
	assert.NotNil(t, manager.logger)
}

func TestOAuthManager_RegisterProvider(t *testing.T) {
	rdb := setupTestRedis(t)
	logger := log.DefaultLogger
	manager := NewOAuthManager(rdb, logger)

	mockProv := &mockProvider{
		providerType: data.ProviderClaudeOfficial,
		authURL:      "https://claude.ai/oauth/authorize",
	}

	t.Run("Register provider successfully", func(t *testing.T) {
		manager.RegisterProvider(mockProv)
		assert.NotNil(t, manager.providers[data.ProviderClaudeOfficial])
		assert.Equal(t, mockProv, manager.providers[data.ProviderClaudeOfficial])
	})

	t.Run("Register multiple providers", func(t *testing.T) {
		mockProv2 := &mockProvider{
			providerType: data.ProviderCodexCLI,
			authURL:      "https://auth.openai.com/oauth/authorize",
		}
		manager.RegisterProvider(mockProv2)

		assert.Len(t, manager.providers, 2)
		assert.NotNil(t, manager.providers[data.ProviderClaudeOfficial])
		assert.NotNil(t, manager.providers[data.ProviderCodexCLI])
	})
}

func TestOAuthManager_GenerateAuthURL(t *testing.T) {
	rdb := setupTestRedis(t)
	logger := log.DefaultLogger
	manager := NewOAuthManager(rdb, logger)

	mockProv := &mockProvider{
		providerType: data.ProviderClaudeOfficial,
		authURL:      "https://claude.ai/oauth/authorize?code=true",
		codeVerifier: "test-code-verifier-123",
	}
	manager.RegisterProvider(mockProv)

	ctx := context.Background()

	t.Run("Generate auth URL successfully", func(t *testing.T) {
		params := &OAuthParams{
			State:       "test-state",
			ProxyURL:    "http://proxy.example.com:8080",
			RedirectURI: "https://example.com/callback",
		}

		resp, err := manager.GenerateAuthURL(ctx, data.ProviderClaudeOfficial, params)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.SessionID)
		assert.Equal(t, "test-state", resp.State)
		assert.Equal(t, mockProv.authURL, resp.AuthURL)

		// Verify session saved to Redis
		session, err := manager.LoadSession(ctx, resp.SessionID)
		require.NoError(t, err)
		assert.Equal(t, data.ProviderClaudeOfficial, session.Provider)
		assert.Equal(t, "test-code-verifier-123", session.CodeVerifier)
		assert.Equal(t, "test-state", session.State)
		assert.Equal(t, "http://proxy.example.com:8080", session.ProxyURL)
	})

	t.Run("Generate auth URL with auto-generated state", func(t *testing.T) {
		params := &OAuthParams{
			ProxyURL: "",
		}

		resp, err := manager.GenerateAuthURL(ctx, data.ProviderClaudeOfficial, params)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.State, "state should be auto-generated")
		assert.Len(t, resp.State, 64, "state should be 64 hex chars")
	})

	t.Run("Unsupported provider", func(t *testing.T) {
		params := &OAuthParams{}
		_, err := manager.GenerateAuthURL(ctx, data.ProviderGemini, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported OAuth provider")
	})
}

func TestOAuthManager_ExchangeCode(t *testing.T) {
	rdb := setupTestRedis(t)
	logger := log.DefaultLogger
	manager := NewOAuthManager(rdb, logger)

	mockTokenResp := &ExtendedTokenResponse{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		ExpiresIn:    3600,
		Scopes:       []string{"openid", "profile"},
	}

	mockProv := &mockProvider{
		providerType: data.ProviderClaudeOfficial,
		tokenResp:    mockTokenResp,
	}
	manager.RegisterProvider(mockProv)

	ctx := context.Background()

	t.Run("Exchange code successfully", func(t *testing.T) {
		// First generate auth URL to create session
		params := &OAuthParams{State: "test-state"}
		authResp, err := manager.GenerateAuthURL(ctx, data.ProviderClaudeOfficial, params)
		require.NoError(t, err)

		// Exchange code
		tokenResp, err := manager.ExchangeCode(ctx, authResp.SessionID, "auth-code-123")
		require.NoError(t, err)
		assert.Equal(t, "access-token-123", tokenResp.AccessToken)
		assert.Equal(t, "refresh-token-456", tokenResp.RefreshToken)
		assert.Equal(t, 3600, tokenResp.ExpiresIn)
		assert.Equal(t, data.ProviderClaudeOfficial, tokenResp.Provider)

		// Verify session deleted after successful exchange
		_, err = manager.LoadSession(ctx, authResp.SessionID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found or expired")
	})

	t.Run("Session not found", func(t *testing.T) {
		_, err := manager.ExchangeCode(ctx, "non-existent-session", "code")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load session")
	})

	t.Run("Session expired (TTL test)", func(t *testing.T) {
		// Create session with very short TTL
		session := &OAuthSession{
			Provider:     data.ProviderClaudeOfficial,
			CodeVerifier: "test",
			State:        "test",
		}
		sessionID := "short-ttl-session"
		key := SessionKeyPrefix + sessionID
		data, _ := json.Marshal(session)
		rdb.Set(ctx, key, data, 1*time.Second)

		// Wait for expiration
		time.Sleep(2 * time.Second)

		_, err := manager.ExchangeCode(ctx, sessionID, "code")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found or expired")
	})
}

func TestOAuthManager_RefreshToken(t *testing.T) {
	rdb := setupTestRedis(t)
	logger := log.DefaultLogger
	manager := NewOAuthManager(rdb, logger)

	mockTokenResp := &ExtendedTokenResponse{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresIn:    3600,
	}

	mockProv := &mockProvider{
		providerType: data.ProviderClaudeOfficial,
		tokenResp:    mockTokenResp,
	}
	manager.RegisterProvider(mockProv)

	ctx := context.Background()

	t.Run("Refresh token successfully", func(t *testing.T) {
		metadata := &AccountMetadata{
			ProxyURL: "http://proxy.example.com:8080",
		}

		tokenResp, err := manager.RefreshToken(ctx, data.ProviderClaudeOfficial, "old-refresh-token", metadata)
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", tokenResp.AccessToken)
		assert.Equal(t, "new-refresh-token", tokenResp.RefreshToken)
		assert.Equal(t, data.ProviderClaudeOfficial, tokenResp.Provider)
	})

	t.Run("Unsupported provider", func(t *testing.T) {
		_, err := manager.RefreshToken(ctx, data.ProviderGemini, "token", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported OAuth provider")
	})
}

func TestOAuthManager_SessionManagement(t *testing.T) {
	rdb := setupTestRedis(t)
	logger := log.DefaultLogger
	manager := NewOAuthManager(rdb, logger)

	ctx := context.Background()

	t.Run("Save and load session", func(t *testing.T) {
		session := &OAuthSession{
			Provider:     data.ProviderClaudeOfficial,
			CodeVerifier: "test-verifier",
			State:        "test-state",
			ProxyURL:     "http://proxy:8080",
			RedirectURI:  "https://callback.com",
			CreatedAt:    time.Now(),
			Metadata:     map[string]string{"key": "value"},
		}

		err := manager.SaveSession(ctx, "test-session-123", session)
		require.NoError(t, err)

		loaded, err := manager.LoadSession(ctx, "test-session-123")
		require.NoError(t, err)
		assert.Equal(t, session.Provider, loaded.Provider)
		assert.Equal(t, session.CodeVerifier, loaded.CodeVerifier)
		assert.Equal(t, session.State, loaded.State)
		assert.Equal(t, session.ProxyURL, loaded.ProxyURL)
	})

	t.Run("Delete session", func(t *testing.T) {
		session := &OAuthSession{
			Provider: data.ProviderClaudeOfficial,
		}
		manager.SaveSession(ctx, "delete-test", session)

		err := manager.DeleteSession(ctx, "delete-test")
		require.NoError(t, err)

		_, err = manager.LoadSession(ctx, "delete-test")
		assert.Error(t, err)
	})

	t.Run("Session TTL verification", func(t *testing.T) {
		session := &OAuthSession{
			Provider: data.ProviderClaudeOfficial,
		}
		manager.SaveSession(ctx, "ttl-test", session)

		// Check TTL
		ttl := rdb.TTL(ctx, SessionKeyPrefix+"ttl-test").Val()
		assert.Greater(t, ttl, 9*time.Minute, "TTL should be close to 10 minutes")
		assert.LessOrEqual(t, ttl, SessionTTL)
	})
}

// Benchmark tests
func BenchmarkOAuthManager_GenerateAuthURL(b *testing.B) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	defer rdb.Close()

	logger := log.DefaultLogger
	manager := NewOAuthManager(rdb, logger)
	mockProv := &mockProvider{
		providerType: data.ProviderClaudeOfficial,
		authURL:      "https://claude.ai/oauth/authorize",
		codeVerifier: "test",
	}
	manager.RegisterProvider(mockProv)

	ctx := context.Background()
	params := &OAuthParams{State: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GenerateAuthURL(ctx, data.ProviderClaudeOfficial, params)
	}
}

func BenchmarkOAuthManager_SaveSession(b *testing.B) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	defer rdb.Close()

	logger := log.DefaultLogger
	manager := NewOAuthManager(rdb, logger)
	ctx := context.Background()

	session := &OAuthSession{
		Provider:     data.ProviderClaudeOfficial,
		CodeVerifier: "test-verifier",
		State:        "test-state",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := "bench-session"
		_ = manager.SaveSession(ctx, sessionID, session)
	}
}
