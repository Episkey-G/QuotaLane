package biz

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/data"
	"QuotaLane/pkg/crypto"
	"QuotaLane/pkg/oauth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAccountRepo implements data.AccountRepo for testing
type mockAccountRepo struct {
	createAccountFunc        func(ctx context.Context, account *data.Account) error
	updateOAuthDataFunc      func(ctx context.Context, accountID int64, oauthDataEncrypted string, expiresAt time.Time) error
	listExpiringAccountsFunc func(ctx context.Context, expiryThreshold time.Time) ([]*data.Account, error)
	accounts                 []*data.Account
}

func (m *mockAccountRepo) CreateAccount(ctx context.Context, account *data.Account) error {
	if m.createAccountFunc != nil {
		return m.createAccountFunc(ctx, account)
	}
	account.ID = 123 // Mock ID
	m.accounts = append(m.accounts, account)
	return nil
}

func (m *mockAccountRepo) GetAccount(ctx context.Context, id int64) (*data.Account, error) {
	return nil, nil
}

func (m *mockAccountRepo) ListAccounts(ctx context.Context, filter *data.AccountFilter) ([]*data.Account, int32, error) {
	return nil, 0, nil
}

func (m *mockAccountRepo) UpdateAccount(ctx context.Context, account *data.Account) error {
	return nil
}

func (m *mockAccountRepo) DeleteAccount(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAccountRepo) ListExpiringAccounts(ctx context.Context, expiryThreshold time.Time) ([]*data.Account, error) {
	if m.listExpiringAccountsFunc != nil {
		return m.listExpiringAccountsFunc(ctx, expiryThreshold)
	}
	return m.accounts, nil
}

func (m *mockAccountRepo) ListAccountsByProvider(ctx context.Context, provider data.AccountProvider, status data.AccountStatus) ([]*data.Account, error) {
	return nil, nil
}

func (m *mockAccountRepo) ListCodexCLIAccountsNeedingRefresh(ctx context.Context) ([]*data.Account, error) {
	return nil, nil
}

func (m *mockAccountRepo) UpdateOAuthData(ctx context.Context, accountID int64, oauthDataEncrypted string, expiresAt time.Time) error {
	if m.updateOAuthDataFunc != nil {
		return m.updateOAuthDataFunc(ctx, accountID, oauthDataEncrypted, expiresAt)
	}
	return nil
}

func (m *mockAccountRepo) UpdateHealthScore(ctx context.Context, accountID int64, score int32) error {
	return nil
}

func (m *mockAccountRepo) UpdateAccountStatus(ctx context.Context, accountID int64, status data.AccountStatus) error {
	return nil
}

// mockOAuthProvider implements oauth.OAuthProvider for testing
type mockOAuthProvider struct {
	authURL      string
	codeVerifier string
	tokenResp    *oauth.ExtendedTokenResponse
	err          error
}

func (m *mockOAuthProvider) GenerateAuthURL(ctx context.Context, params *oauth.OAuthParams) (*oauth.OAuthURLResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &oauth.OAuthURLResponse{
		AuthURL:      m.authURL,
		CodeVerifier: m.codeVerifier,
	}, nil
}

func (m *mockOAuthProvider) ExchangeCode(ctx context.Context, code string, session *oauth.OAuthSession) (*oauth.ExtendedTokenResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokenResp, nil
}

func (m *mockOAuthProvider) RefreshToken(ctx context.Context, refreshToken string, metadata *oauth.AccountMetadata) (*oauth.ExtendedTokenResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokenResp, nil
}

func (m *mockOAuthProvider) ValidateToken(ctx context.Context, token string, metadata *oauth.AccountMetadata) error {
	return m.err
}

func (m *mockOAuthProvider) ProviderType() data.AccountProvider {
	return data.ProviderClaudeOfficial
}

// setupTestOAuth creates test dependencies
func setupTestOAuth(t *testing.T) (*AccountUsecase, *mockAccountRepo, *crypto.AESCrypto) {
	// Create crypto
	testKey := []byte("12345678901234567890123456789012") // 32 bytes
	cryptoHelper, err := crypto.NewAESCrypto(testKey)
	require.NoError(t, err)

	// Create Redis client (skip if unavailable)
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Test DB
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for testing, skipping: " + err.Error())
	}
	rdb.FlushDB(ctx)
	t.Cleanup(func() {
		rdb.FlushDB(ctx)
		rdb.Close()
	})

	// Create OAuth manager
	logger := log.DefaultLogger
	oauthManager := oauth.NewOAuthManager(rdb, logger)

	// Register mock provider
	mockProv := &mockOAuthProvider{
		authURL:      "https://console.anthropic.com/v1/oauth/authorize?code=true",
		codeVerifier: "test-verifier-123",
		tokenResp: &oauth.ExtendedTokenResponse{
			AccessToken:  "access-token-abc",
			RefreshToken: "refresh-token-xyz",
			ExpiresIn:    3600,
			Provider:     data.ProviderClaudeOfficial,
		},
	}
	oauthManager.RegisterProvider(mockProv)

	// Create mock repo
	repo := &mockAccountRepo{}

	// Create usecase
	uc := &AccountUsecase{
		repo:         repo,
		oauthManager: oauthManager,
		crypto:       cryptoHelper,
		logger:       log.NewHelper(logger),
	}

	return uc, repo, cryptoHelper
}

func TestAccountUsecase_GenerateOAuthURL(t *testing.T) {
	uc, _, _ := setupTestOAuth(t)
	ctx := context.Background()

	t.Run("Generate OAuth URL successfully", func(t *testing.T) {
		authURL, sessionID, state, err := uc.GenerateOAuthURL(
			ctx,
			v1.AccountProvider_CLAUDE_OFFICIAL,
			"socks5://localhost:1080",
			"http://localhost:9999/callback",
			[]string{"openid", "profile"},
			map[string]string{"key": "value"},
		)

		require.NoError(t, err)
		assert.NotEmpty(t, authURL)
		assert.Contains(t, authURL, "https://console.anthropic.com/v1/oauth/authorize")
		assert.NotEmpty(t, sessionID)
		assert.NotEmpty(t, state)
	})

	t.Run("Unsupported provider", func(t *testing.T) {
		_, _, _, err := uc.GenerateOAuthURL(
			ctx,
			v1.AccountProvider_ACCOUNT_PROVIDER_UNSPECIFIED,
			"", "", nil, nil,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported provider")
	})
}

func TestAccountUsecase_ExchangeOAuthCode(t *testing.T) {
	uc, repo, cryptoHelper := setupTestOAuth(t)
	ctx := context.Background()

	t.Run("Exchange code successfully with full encryption", func(t *testing.T) {
		// First generate auth URL to create session
		authURL, sessionID, _, err := uc.GenerateOAuthURL(
			ctx,
			v1.AccountProvider_CLAUDE_OFFICIAL,
			"",
			"",
			nil,
			nil,
		)
		require.NoError(t, err)
		assert.NotEmpty(t, authURL)

		// Exchange code
		accountID, accountName, status, expiresAt, err := uc.ExchangeOAuthCode(
			ctx,
			sessionID,
			"test-auth-code",
			"My Claude Account",
			"Test account for OAuth",
			100,  // RPM
			1000, // TPM
			map[string]string{"proxy_url": "socks5://localhost:1080"},
		)

		require.NoError(t, err)
		assert.Equal(t, int64(123), accountID, "Should return mocked account ID")
		assert.Equal(t, "My Claude Account", accountName)
		assert.Equal(t, "active", status)
		assert.NotNil(t, expiresAt)

		// Verify account was created in repo
		require.Len(t, repo.accounts, 1)
		account := repo.accounts[0]

		assert.Equal(t, "My Claude Account", account.Name)
		assert.Equal(t, "Test account for OAuth", account.Description)
		assert.Equal(t, data.ProviderClaudeOfficial, account.Provider)
		assert.Equal(t, int32(100), account.RpmLimit)
		assert.Equal(t, int32(1000), account.TpmLimit)
		assert.Equal(t, data.StatusActive, account.Status)
		assert.Equal(t, int32(100), account.HealthScore)

		// Verify OAuth data encryption
		assert.NotEmpty(t, account.OAuthDataEncrypted)

		// Decrypt and verify structure
		decrypted, err := cryptoHelper.Decrypt(account.OAuthDataEncrypted)
		require.NoError(t, err)

		var oauthData map[string]interface{}
		err = json.Unmarshal([]byte(decrypted), &oauthData)
		require.NoError(t, err)

		// Verify encrypted tokens exist
		assert.NotEmpty(t, oauthData["access_token_encrypted"], "access_token should be encrypted")
		assert.NotEmpty(t, oauthData["refresh_token_encrypted"], "refresh_token should be encrypted")
		assert.NotEmpty(t, oauthData["expires_at"])

		// Verify access token encryption (decrypt the encrypted token)
		accessTokenEncrypted, ok := oauthData["access_token_encrypted"].(string)
		require.True(t, ok)
		accessToken, err := cryptoHelper.Decrypt(accessTokenEncrypted)
		require.NoError(t, err)
		assert.Equal(t, "access-token-abc", accessToken)

		// Verify refresh token encryption
		refreshTokenEncrypted, ok := oauthData["refresh_token_encrypted"].(string)
		require.True(t, ok)
		refreshToken, err := cryptoHelper.Decrypt(refreshTokenEncrypted)
		require.NoError(t, err)
		assert.Equal(t, "refresh-token-xyz", refreshToken)
	})

	t.Run("Exchange code with invalid session ID", func(t *testing.T) {
		_, _, _, _, err := uc.ExchangeOAuthCode(
			ctx,
			"non-existent-session",
			"code",
			"Account",
			"",
			0, 0, nil,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to exchange code")
	})
}

func TestAccountUsecase_GetProxyConfig(t *testing.T) {
	uc, _, _ := setupTestOAuth(t)

	t.Run("Priority 1: Request-level proxy", func(t *testing.T) {
		metadata := `{"proxy_url":"http://account-proxy:8080"}`
		os.Setenv("HTTP_PROXY", "http://global-proxy:8080")
		defer os.Unsetenv("HTTP_PROXY")

		proxy := uc.getProxyConfig(metadata, "socks5://request-proxy:1080")
		assert.Equal(t, "socks5://request-proxy:1080", proxy, "Should use request-level proxy")
	})

	t.Run("Priority 2: Account-level proxy", func(t *testing.T) {
		metadata := `{"proxy_url":"http://account-proxy:8080"}`
		os.Setenv("HTTP_PROXY", "http://global-proxy:8080")
		defer os.Unsetenv("HTTP_PROXY")

		proxy := uc.getProxyConfig(metadata, "")
		assert.Equal(t, "http://account-proxy:8080", proxy, "Should use account-level proxy")
	})

	t.Run("Priority 3: Global HTTP_PROXY env", func(t *testing.T) {
		os.Setenv("HTTP_PROXY", "http://global-proxy:8080")
		defer os.Unsetenv("HTTP_PROXY")

		proxy := uc.getProxyConfig("", "")
		assert.Equal(t, "http://global-proxy:8080", proxy, "Should use global HTTP_PROXY")
	})

	t.Run("Priority 3: Global HTTPS_PROXY env (fallback)", func(t *testing.T) {
		os.Setenv("HTTPS_PROXY", "https://global-proxy:8443")
		defer os.Unsetenv("HTTPS_PROXY")

		proxy := uc.getProxyConfig("", "")
		assert.Equal(t, "https://global-proxy:8443", proxy, "Should use global HTTPS_PROXY")
	})

	t.Run("No proxy configured", func(t *testing.T) {
		proxy := uc.getProxyConfig("", "")
		assert.Empty(t, proxy, "Should return empty string when no proxy configured")
	})

	t.Run("Invalid metadata JSON", func(t *testing.T) {
		proxy := uc.getProxyConfig("{invalid json", "")
		assert.Empty(t, proxy, "Should handle invalid JSON gracefully")
	})
}

func TestProtoProviderToDataProvider(t *testing.T) {
	tests := []struct {
		name          string
		protoProvider v1.AccountProvider
		wantData      data.AccountProvider
		wantErr       bool
	}{
		{
			name:          "Claude Official",
			protoProvider: v1.AccountProvider_CLAUDE_OFFICIAL,
			wantData:      data.ProviderClaudeOfficial,
			wantErr:       false,
		},
		{
			name:          "Codex CLI",
			protoProvider: v1.AccountProvider_CODEX_CLI,
			wantData:      data.ProviderCodexCLI,
			wantErr:       false,
		},
		{
			name:          "Gemini",
			protoProvider: v1.AccountProvider_GEMINI,
			wantData:      data.ProviderGemini,
			wantErr:       false,
		},
		{
			name:          "Droid",
			protoProvider: v1.AccountProvider_DROID,
			wantData:      data.ProviderDroid,
			wantErr:       false,
		},
		{
			name:          "Unsupported provider",
			protoProvider: v1.AccountProvider_ACCOUNT_PROVIDER_UNSPECIFIED,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataProvider, err := protoProviderToDataProvider(tt.protoProvider)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported provider")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantData, dataProvider)
		})
	}
}

// Benchmark tests
func BenchmarkAccountUsecase_GetProxyConfig(b *testing.B) {
	uc, _, _ := setupTestOAuth(&testing.T{})
	metadata := `{"proxy_url":"http://proxy:8080"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = uc.getProxyConfig(metadata, "")
	}
}
