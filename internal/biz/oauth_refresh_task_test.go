package biz

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"QuotaLane/internal/data"
	"QuotaLane/pkg/crypto"
	"QuotaLane/pkg/oauth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRefreshTask(t *testing.T) (*OAuthRefreshTask, *mockAccountRepo, *crypto.AESCrypto) {
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

	// Register mock provider with refresh capability
	mockProv := &mockOAuthProvider{
		tokenResp: &oauth.ExtendedTokenResponse{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			ExpiresIn:    3600,
			IDToken:      "new-id-token",
			Scopes:       []string{"openid", "profile"},
			Provider:     data.ProviderClaudeOfficial,
		},
	}
	oauthManager.RegisterProvider(mockProv)

	// Create mock repo
	repo := &mockAccountRepo{}

	// Create task
	task := NewOAuthRefreshTask(repo, oauthManager, cryptoHelper, logger)

	return task, repo, cryptoHelper
}

func TestOAuthRefreshTask_RefreshExpiringTokens(t *testing.T) {
	task, repo, cryptoHelper := setupTestRefreshTask(t)
	ctx := context.Background()

	t.Run("No expiring accounts", func(t *testing.T) {
		repo.accounts = []*data.Account{} // Empty list

		err := task.RefreshExpiringTokens(ctx)
		assert.NoError(t, err)
	})

	t.Run("Refresh single expiring account successfully", func(t *testing.T) {
		// Prepare encrypted OAuth data
		accessTokenEncrypted, _ := cryptoHelper.Encrypt("old-access-token")
		refreshTokenEncrypted, _ := cryptoHelper.Encrypt("old-refresh-token")

		oauthData := map[string]interface{}{
			"access_token_encrypted":  accessTokenEncrypted,
			"refresh_token_encrypted": refreshTokenEncrypted,
			"expires_at":              time.Now().Add(12 * time.Hour).Format(time.RFC3339),
			"id_token":                "old-id-token",
			"scopes":                  []string{"openid"},
		}
		oauthDataJSON, _ := json.Marshal(oauthData)
		oauthDataEncrypted, _ := cryptoHelper.Encrypt(string(oauthDataJSON))

		expiresAt := time.Now().Add(12 * time.Hour)
		repo.accounts = []*data.Account{
			{
				ID:                 456,
				Name:               "Expiring Account",
				Provider:           data.ProviderClaudeOfficial,
				OAuthDataEncrypted: oauthDataEncrypted,
				TokenExpiresAt:     &expiresAt,
			},
		}

		// Track updates
		var updatedOAuthData string
		var updatedExpiresAt time.Time
		repo.updateOAuthDataFunc = func(ctx context.Context, accountID int64, oauthDataEncrypted string, expiresAt time.Time) error {
			assert.Equal(t, int64(456), accountID)
			updatedOAuthData = oauthDataEncrypted
			updatedExpiresAt = expiresAt
			return nil
		}

		err := task.RefreshExpiringTokens(ctx)
		require.NoError(t, err)

		// Verify OAuth data was updated
		assert.NotEmpty(t, updatedOAuthData)
		assert.NotZero(t, updatedExpiresAt)
		assert.True(t, updatedExpiresAt.After(time.Now()), "New expires_at should be in the future")

		// Decrypt and verify new tokens
		decrypted, err := cryptoHelper.Decrypt(updatedOAuthData)
		require.NoError(t, err)

		var newOAuthData map[string]interface{}
		err = json.Unmarshal([]byte(decrypted), &newOAuthData)
		require.NoError(t, err)

		// Decrypt access token
		newAccessTokenEncrypted, ok := newOAuthData["access_token_encrypted"].(string)
		require.True(t, ok)
		newAccessToken, err := cryptoHelper.Decrypt(newAccessTokenEncrypted)
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", newAccessToken)

		// Decrypt refresh token
		newRefreshTokenEncrypted, ok := newOAuthData["refresh_token_encrypted"].(string)
		require.True(t, ok)
		newRefreshToken, err := cryptoHelper.Decrypt(newRefreshTokenEncrypted)
		require.NoError(t, err)
		assert.Equal(t, "new-refresh-token", newRefreshToken)

		// Verify ID token updated
		assert.Equal(t, "new-id-token", newOAuthData["id_token"])

		// Verify scopes updated
		scopes, ok := newOAuthData["scopes"].([]interface{})
		require.True(t, ok)
		assert.Len(t, scopes, 2)
		assert.Equal(t, "openid", scopes[0])
		assert.Equal(t, "profile", scopes[1])
	})

	t.Run("Refresh multiple accounts with partial failures", func(t *testing.T) {
		// Account 1: Valid account
		accessTokenEncrypted1, _ := cryptoHelper.Encrypt("token-1")
		refreshTokenEncrypted1, _ := cryptoHelper.Encrypt("refresh-1")
		oauthData1 := map[string]interface{}{
			"access_token_encrypted":  accessTokenEncrypted1,
			"refresh_token_encrypted": refreshTokenEncrypted1,
			"expires_at":              time.Now().Add(12 * time.Hour).Format(time.RFC3339),
		}
		oauthDataJSON1, _ := json.Marshal(oauthData1)
		oauthDataEncrypted1, _ := cryptoHelper.Encrypt(string(oauthDataJSON1))
		expiresAt1 := time.Now().Add(12 * time.Hour)

		// Account 2: Missing refresh_token_encrypted (should fail)
		oauthData2 := map[string]interface{}{
			"access_token_encrypted": "some-token",
			// Missing "refresh_token_encrypted"
		}
		oauthDataJSON2, _ := json.Marshal(oauthData2)
		oauthDataEncrypted2, _ := cryptoHelper.Encrypt(string(oauthDataJSON2))
		expiresAt2 := time.Now().Add(6 * time.Hour)

		repo.accounts = []*data.Account{
			{
				ID:                 101,
				Name:               "Valid Account",
				Provider:           data.ProviderClaudeOfficial,
				OAuthDataEncrypted: oauthDataEncrypted1,
				TokenExpiresAt:     &expiresAt1,
			},
			{
				ID:                 102,
				Name:               "Broken Account",
				Provider:           data.ProviderClaudeOfficial,
				OAuthDataEncrypted: oauthDataEncrypted2,
				TokenExpiresAt:     &expiresAt2,
			},
		}

		updateCount := 0
		repo.updateOAuthDataFunc = func(ctx context.Context, accountID int64, oauthDataEncrypted string, expiresAt time.Time) error {
			updateCount++
			return nil
		}

		err := task.RefreshExpiringTokens(ctx)
		assert.NoError(t, err, "Task should complete even with partial failures")
		assert.Equal(t, 1, updateCount, "Only valid account should be updated")
	})

	t.Run("Refresh with account-level proxy", func(t *testing.T) {
		// Prepare account with proxy metadata
		accessTokenEncrypted, _ := cryptoHelper.Encrypt("access")
		refreshTokenEncrypted, _ := cryptoHelper.Encrypt("refresh")
		oauthData := map[string]interface{}{
			"access_token_encrypted":  accessTokenEncrypted,
			"refresh_token_encrypted": refreshTokenEncrypted,
			"expires_at":              time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		oauthDataJSON, _ := json.Marshal(oauthData)
		oauthDataEncrypted, _ := cryptoHelper.Encrypt(string(oauthDataJSON))

		metadata := `{"proxy_url":"socks5://localhost:1080"}`
		expiresAt := time.Now().Add(1 * time.Hour)

		repo.accounts = []*data.Account{
			{
				ID:                 789,
				Name:               "Proxy Account",
				Provider:           data.ProviderClaudeOfficial,
				OAuthDataEncrypted: oauthDataEncrypted,
				TokenExpiresAt:     &expiresAt,
				Metadata:           &metadata,
			},
		}

		repo.updateOAuthDataFunc = func(ctx context.Context, accountID int64, oauthDataEncrypted string, expiresAt time.Time) error {
			return nil
		}

		err := task.RefreshExpiringTokens(ctx)
		assert.NoError(t, err)
	})
}

func TestOAuthRefreshTask_RefreshAccountToken(t *testing.T) {
	task, repo, cryptoHelper := setupTestRefreshTask(t)
	ctx := context.Background()

	t.Run("Refresh token successfully", func(t *testing.T) {
		// Prepare encrypted OAuth data
		accessTokenEncrypted, _ := cryptoHelper.Encrypt("old-access")
		refreshTokenEncrypted, _ := cryptoHelper.Encrypt("old-refresh")
		oauthData := map[string]interface{}{
			"access_token_encrypted":  accessTokenEncrypted,
			"refresh_token_encrypted": refreshTokenEncrypted,
			"expires_at":              time.Now().Add(6 * time.Hour).Format(time.RFC3339),
		}
		oauthDataJSON, _ := json.Marshal(oauthData)
		oauthDataEncrypted, _ := cryptoHelper.Encrypt(string(oauthDataJSON))

		expiresAt := time.Now().Add(6 * time.Hour)
		account := &data.Account{
			ID:                 999,
			Name:               "Test Account",
			Provider:           data.ProviderClaudeOfficial,
			OAuthDataEncrypted: oauthDataEncrypted,
			TokenExpiresAt:     &expiresAt,
		}

		repo.updateOAuthDataFunc = func(ctx context.Context, accountID int64, oauthDataEncrypted string, expiresAt time.Time) error {
			assert.Equal(t, int64(999), accountID)
			return nil
		}

		err := task.refreshAccountToken(ctx, account)
		assert.NoError(t, err)
	})

	t.Run("Missing refresh_token_encrypted", func(t *testing.T) {
		oauthData := map[string]interface{}{
			"access_token_encrypted": "some-token",
			// Missing refresh_token_encrypted
		}
		oauthDataJSON, _ := json.Marshal(oauthData)
		oauthDataEncrypted, _ := cryptoHelper.Encrypt(string(oauthDataJSON))

		expiresAt := time.Now().Add(1 * time.Hour)
		account := &data.Account{
			ID:                 888,
			Provider:           data.ProviderClaudeOfficial,
			OAuthDataEncrypted: oauthDataEncrypted,
			TokenExpiresAt:     &expiresAt,
		}

		err := task.refreshAccountToken(ctx, account)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "refresh_token_encrypted not found")
	})

	t.Run("Decryption failure", func(t *testing.T) {
		// Invalid encrypted data
		account := &data.Account{
			ID:                 777,
			Provider:           data.ProviderClaudeOfficial,
			OAuthDataEncrypted: "invalid-encrypted-data",
		}

		err := task.refreshAccountToken(ctx, account)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decrypt OAuth data")
	})
}

func TestOAuthRefreshTask_2HourThreshold(t *testing.T) {
	task, repo, cryptoHelper := setupTestRefreshTask(t)
	ctx := context.Background()

	t.Run("Verify 2-hour threshold query", func(t *testing.T) {
		var capturedThreshold time.Time
		repo.listExpiringAccountsFunc = func(ctx context.Context, expiryThreshold time.Time) ([]*data.Account, error) {
			capturedThreshold = expiryThreshold
			return []*data.Account{}, nil
		}

		now := time.Now()
		err := task.RefreshExpiringTokens(ctx)
		require.NoError(t, err)

		// Verify threshold is approximately 2 hours from now (optimized from 24h to 2h)
		// Use a more lenient comparison to account for time calculation precision
		expectedThreshold := now.Add(2 * time.Hour)
		diff := capturedThreshold.Sub(expectedThreshold)
		assert.Less(t, diff.Abs(), 5*time.Second, "Threshold should be approximately 2 hours from now (within 5 seconds)")
	})

	t.Run("Account expiring in 1 hour should be refreshed", func(t *testing.T) {
		// Create account expiring in 1 hour (within 2-hour threshold)
		accessTokenEncrypted, _ := cryptoHelper.Encrypt("token")
		refreshTokenEncrypted, _ := cryptoHelper.Encrypt("refresh")
		oauthData := map[string]interface{}{
			"access_token_encrypted":  accessTokenEncrypted,
			"refresh_token_encrypted": refreshTokenEncrypted,
			"expires_at":              time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		oauthDataJSON, _ := json.Marshal(oauthData)
		oauthDataEncrypted, _ := cryptoHelper.Encrypt(string(oauthDataJSON))

		expiresAt := time.Now().Add(1 * time.Hour)
		account := &data.Account{
			ID:                 111,
			Provider:           data.ProviderClaudeOfficial,
			OAuthDataEncrypted: oauthDataEncrypted,
			TokenExpiresAt:     &expiresAt,
		}

		// Set up the mock to return the account
		repo.listExpiringAccountsFunc = func(ctx context.Context, expiryThreshold time.Time) ([]*data.Account, error) {
			return []*data.Account{account}, nil
		}

		updated := false
		repo.updateOAuthDataFunc = func(ctx context.Context, accountID int64, oauthDataEncrypted string, expiresAt time.Time) error {
			updated = true
			return nil
		}

		err := task.RefreshExpiringTokens(ctx)
		assert.NoError(t, err)
		assert.True(t, updated, "Account expiring in 1 hour should be refreshed")
	})
}

func TestNewOAuthRefreshTask(t *testing.T) {
	t.Run("Create task with all dependencies", func(t *testing.T) {
		repo := &mockAccountRepo{}
		rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
		logger := log.DefaultLogger
		oauthManager := oauth.NewOAuthManager(rdb, logger)
		cryptoHelper, _ := crypto.NewAESCrypto([]byte("12345678901234567890123456789012"))

		task := NewOAuthRefreshTask(repo, oauthManager, cryptoHelper, logger)

		assert.NotNil(t, task)
		assert.NotNil(t, task.repo)
		assert.NotNil(t, task.oauthManager)
		assert.NotNil(t, task.crypto)
		assert.NotNil(t, task.logger)
	})
}

// Benchmark tests
func BenchmarkOAuthRefreshTask_RefreshAccountToken(b *testing.B) {
	task, _, cryptoHelper := setupTestRefreshTask(&testing.T{})

	// Prepare test account
	accessTokenEncrypted, _ := cryptoHelper.Encrypt("access")
	refreshTokenEncrypted, _ := cryptoHelper.Encrypt("refresh")
	oauthData := map[string]interface{}{
		"access_token_encrypted":  accessTokenEncrypted,
		"refresh_token_encrypted": refreshTokenEncrypted,
		"expires_at":              time.Now().Add(6 * time.Hour).Format(time.RFC3339),
	}
	oauthDataJSON, _ := json.Marshal(oauthData)
	oauthDataEncrypted, _ := cryptoHelper.Encrypt(string(oauthDataJSON))

	expiresAt := time.Now().Add(6 * time.Hour)
	account := &data.Account{
		ID:                 1,
		Provider:           data.ProviderClaudeOfficial,
		OAuthDataEncrypted: oauthDataEncrypted,
		TokenExpiresAt:     &expiresAt,
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = task.refreshAccountToken(ctx, account)
	}
}
