//go:build integration
// +build integration

package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"QuotaLane/internal/conf"
	"QuotaLane/internal/data"
	"QuotaLane/pkg/crypto"
	"QuotaLane/pkg/oauth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// IntegrationTestSuite holds test dependencies
type IntegrationTestSuite struct {
	db          *gorm.DB
	rdb         *redis.Client
	accountRepo data.AccountRepo
	crypto      *crypto.AESCrypto
	oauth       oauth.OAuthService
	uc          *AccountUsecase
	logger      log.Logger
}

// setupTestSuite creates test infrastructure (MySQL + Redis)
func setupTestSuite(t *testing.T) *IntegrationTestSuite {
	t.Helper()

	// 1. Setup MySQL (requires running MySQL instance)
	// Connection string format: user:password@tcp(host:port)/dbname?parseTime=true
	// Use environment variable TEST_MYSQL_DSN if set, otherwise use default
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		// Default: use docker-compose.yml MySQL service
		dsn = "root:root@tcp(127.0.0.1:3306)/quotalane?parseTime=true&loc=UTC"
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to MySQL. Ensure test database is running.\nRun: docker-compose up -d mysql redis")

	// Auto-migrate schema
	err = db.AutoMigrate(&data.Account{})
	require.NoError(t, err, "Failed to migrate schema")

	// 2. Setup Redis (requires running Redis instance)
	// Use environment variable TEST_REDIS_ADDR if set, otherwise use default
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		// Default: use docker-compose.yml Redis service
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   1, // Use DB 1 for testing
	})
	ctx := context.Background()
	err = rdb.Ping(ctx).Err()
	require.NoError(t, err, "Failed to connect to Redis")

	// 3. Create crypto service (32-byte key for AES-256)
	encryptionKey := "12345678901234567890123456789012"
	cryptoSvc, err := crypto.NewAESCrypto([]byte(encryptionKey))
	require.NoError(t, err, "Failed to create crypto service")

	// 4. Create OAuth service (will use mock server in tests)
	oauthSvc := oauth.NewOAuthService()

	// 5. Create logger
	logger := log.NewStdLogger(os.Stdout)

	// 6. Create Redis-based cache client
	cache := data.NewCacheClient(rdb)

	// 7. Create Data wrapper using NewData
	dataWrapper, cleanup, err := data.NewData(&conf.Data{}, logger, rdb, cache)
	require.NoError(t, err, "Failed to create Data wrapper")
	t.Cleanup(cleanup) // Ensure cleanup runs after test

	// 8. Create account repository
	accountRepo := data.NewAccountRepo(dataWrapper, db, logger)

	// 9. Create account usecase
	uc := NewAccountUsecase(accountRepo, cryptoSvc, oauthSvc, rdb, logger)

	return &IntegrationTestSuite{
		db:          db,
		rdb:         rdb,
		accountRepo: accountRepo,
		crypto:      cryptoSvc,
		oauth:       oauthSvc,
		uc:          uc,
		logger:      logger,
	}
}

// teardownTestSuite cleans up test resources
func (s *IntegrationTestSuite) teardownTestSuite(t *testing.T) {
	t.Helper()

	// Clean up database (use DELETE instead of TRUNCATE to avoid foreign key issues)
	s.db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	s.db.Exec("DELETE FROM api_accounts")
	s.db.Exec("SET FOREIGN_KEY_CHECKS = 1")

	// Clean up Redis
	ctx := context.Background()
	s.rdb.FlushDB(ctx)
	s.rdb.Close()
}

// TestRefreshClaudeToken_Success tests successful token refresh flow
func TestRefreshClaudeToken_Success(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.teardownTestSuite(t)

	ctx := context.Background()

	// 1. Setup mock OAuth server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/v1/oauth/token", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Return success response
		resp := oauth.TokenResponse{
			AccessToken:  "new_access_token_12345",
			RefreshToken: "new_refresh_token_67890",
			ExpiresIn:    7200, // 2 hours
			Scope:        "claude:messages:read claude:messages:write",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Create OAuth service with mock server endpoint
	mockOAuthSvc := oauth.NewOAuthServiceWithConfig(mockServer.URL+"/v1/oauth/token", 30*time.Second, 3)

	// Replace the usecase with one using the mock OAuth service
	suite.uc = NewAccountUsecase(suite.accountRepo, suite.crypto, mockOAuthSvc, suite.rdb, suite.logger)

	// 2. Create test account with expiring OAuth data
	oldAccessToken := "old_access_token_abcde"
	oldRefreshToken := "old_refresh_token_fghij"
	expiresAt := time.Now().UTC().Add(-10 * time.Minute) // Already expired

	oauthData := OAuthData{
		AccessToken:  oldAccessToken,
		RefreshToken: oldRefreshToken,
		ExpiresAt:    expiresAt,
	}
	oauthJSON, _ := json.Marshal(oauthData)
	encryptedOAuth, err := suite.crypto.Encrypt(string(oauthJSON))
	require.NoError(t, err)

	account := &data.Account{
		Name:               "Test Claude Account",
		Provider:           data.ProviderClaudeOfficial,
		Status:             data.StatusActive,
		HealthScore:        80, // Will be reset to 100 on success
		OAuthDataEncrypted: encryptedOAuth,
		OAuthExpiresAt:     &expiresAt,
		RpmLimit:           50,
		TpmLimit:           200000,
		IsCircuitBroken:    false,
		Metadata:           "{}",
	}

	err = suite.accountRepo.CreateAccount(ctx, account)
	require.NoError(t, err)
	require.NotZero(t, account.ID)

	// 3. Refresh the token
	err = suite.uc.RefreshClaudeToken(ctx, account.ID)
	require.NoError(t, err)

	// 4. Verify database updates
	updatedAccount, err := suite.accountRepo.GetAccount(ctx, account.ID)
	require.NoError(t, err)

	// Decrypt and verify new OAuth data
	decrypted, err := suite.crypto.Decrypt(updatedAccount.OAuthDataEncrypted)
	require.NoError(t, err)

	var newOAuth OAuthData
	err = json.Unmarshal([]byte(decrypted), &newOAuth)
	require.NoError(t, err)

	assert.Equal(t, "new_access_token_12345", newOAuth.AccessToken)
	assert.Equal(t, "new_refresh_token_67890", newOAuth.RefreshToken)
	assert.True(t, newOAuth.ExpiresAt.After(time.Now().UTC()))

	// Verify health score reset
	assert.Equal(t, int32(100), updatedAccount.HealthScore)

	// Verify oauth_expires_at updated
	require.NotNil(t, updatedAccount.OAuthExpiresAt)
	assert.True(t, updatedAccount.OAuthExpiresAt.After(time.Now().UTC()))

	// 5. Verify Redis failure counter cleared
	failureKey := fmt.Sprintf("%s%d", RefreshFailureKeyPrefix, account.ID)
	count, err := suite.rdb.Get(ctx, failureKey).Int64()
	assert.Equal(t, redis.Nil, err) // Key should not exist
	assert.Zero(t, count)
}

// TestRefreshClaudeToken_Failure tests failure handling
func TestRefreshClaudeToken_Failure(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.teardownTestSuite(t)

	ctx := context.Background()

	// 1. Setup mock OAuth server that fails
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token expired"}`))
	}))
	defer mockServer.Close()

	// Create OAuth service with mock server endpoint
	mockOAuthSvc := oauth.NewOAuthServiceWithConfig(mockServer.URL+"/v1/oauth/token", 30*time.Second, 3)
	suite.uc = NewAccountUsecase(suite.accountRepo, suite.crypto, mockOAuthSvc, suite.rdb, suite.logger)

	// 2. Create test account
	oauthData := OAuthData{
		AccessToken:  "access_token",
		RefreshToken: "invalid_refresh_token",
		ExpiresAt:    time.Now().UTC().Add(-1 * time.Hour),
	}
	oauthJSON, _ := json.Marshal(oauthData)
	encryptedOAuth, _ := suite.crypto.Encrypt(string(oauthJSON))

	expiresAt := time.Now().UTC().Add(-1 * time.Hour)
	account := &data.Account{
		Name:               "Test Failing Account",
		Provider:           data.ProviderClaudeOfficial,
		Status:             data.StatusActive,
		HealthScore:        100,
		OAuthDataEncrypted: encryptedOAuth,
		OAuthExpiresAt:     &expiresAt,
		RpmLimit:           50,
		TpmLimit:           200000,
		Metadata:           "{}",
	}

	err := suite.accountRepo.CreateAccount(ctx, account)
	require.NoError(t, err)

	// 3. Attempt refresh (should fail)
	err = suite.uc.RefreshClaudeToken(ctx, account.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OAuth refresh failed")

	// 4. Verify health score decreased
	updatedAccount, err := suite.accountRepo.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(80), updatedAccount.HealthScore) // 100 - 20

	// 5. Verify Redis failure counter incremented
	failureKey := fmt.Sprintf("%s%d", RefreshFailureKeyPrefix, account.ID)
	count, err := suite.rdb.Get(ctx, failureKey).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// 6. Verify TTL set (30 minutes)
	ttl, err := suite.rdb.TTL(ctx, failureKey).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, 25*time.Minute) // Should be close to 30 minutes
	assert.LessOrEqual(t, ttl, 30*time.Minute)
}

// TestRefreshClaudeToken_ConsecutiveFailures tests marking account as ERROR
func TestRefreshClaudeToken_ConsecutiveFailures(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.teardownTestSuite(t)

	ctx := context.Background()

	// Setup failing OAuth server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer mockServer.Close()

	// Create OAuth service with mock server endpoint
	mockOAuthSvc := oauth.NewOAuthServiceWithConfig(mockServer.URL+"/v1/oauth/token", 30*time.Second, 3)
	suite.uc = NewAccountUsecase(suite.accountRepo, suite.crypto, mockOAuthSvc, suite.rdb, suite.logger)

	// Create test account
	oauthData := OAuthData{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().UTC().Add(-1 * time.Hour),
	}
	oauthJSON, _ := json.Marshal(oauthData)
	encryptedOAuth, _ := suite.crypto.Encrypt(string(oauthJSON))

	expiresAt := time.Now().UTC().Add(-1 * time.Hour)
	account := &data.Account{
		Name:               "Test Consecutive Failures",
		Provider:           data.ProviderClaudeConsole,
		Status:             data.StatusActive,
		HealthScore:        100,
		OAuthDataEncrypted: encryptedOAuth,
		OAuthExpiresAt:     &expiresAt,
		RpmLimit:           50,
		TpmLimit:           200000,
		Metadata:           "{}",
	}

	err := suite.accountRepo.CreateAccount(ctx, account)
	require.NoError(t, err)

	// Trigger 3 consecutive failures
	for i := 1; i <= 3; i++ {
		err = suite.uc.RefreshClaudeToken(ctx, account.ID)
		assert.Error(t, err)

		// Check failure count
		failureKey := fmt.Sprintf("%s%d", RefreshFailureKeyPrefix, account.ID)
		count, _ := suite.rdb.Get(ctx, failureKey).Int64()
		assert.Equal(t, int64(i), count)
	}

	// Verify account marked as ERROR after 3rd failure
	updatedAccount, err := suite.accountRepo.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, data.StatusError, updatedAccount.Status)

	// Verify health score
	expectedScore := int32(100 - 20*3) // 100 - 60 = 40
	assert.Equal(t, expectedScore, updatedAccount.HealthScore)

	// Verify alert marker set
	alertKey := fmt.Sprintf("%s%d", AlertKeyPrefix, account.ID)
	alertMsg, err := suite.rdb.Get(ctx, alertKey).Result()
	require.NoError(t, err)
	assert.Contains(t, alertMsg, "marked as ERROR")
	assert.Contains(t, alertMsg, "3 consecutive refresh failures")
}

// TestAutoRefreshTokens_BatchProcessing tests concurrent batch refresh
func TestAutoRefreshTokens_BatchProcessing(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.teardownTestSuite(t)

	ctx := context.Background()

	// Setup mock OAuth server
	refreshCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCount++
		resp := oauth.TokenResponse{
			AccessToken:  fmt.Sprintf("new_access_%d", refreshCount),
			RefreshToken: fmt.Sprintf("new_refresh_%d", refreshCount),
			ExpiresIn:    7200,
			Scope:        "claude:messages:read claude:messages:write",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Create OAuth service with mock server endpoint
	mockOAuthSvc := oauth.NewOAuthServiceWithConfig(mockServer.URL+"/v1/oauth/token", 30*time.Second, 3)
	suite.uc = NewAccountUsecase(suite.accountRepo, suite.crypto, mockOAuthSvc, suite.rdb, suite.logger)

	// Create 10 expiring accounts (will expire in 5 minutes)
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	accountIDs := make([]int64, 10)

	for i := 0; i < 10; i++ {
		oauthData := OAuthData{
			AccessToken:  fmt.Sprintf("old_access_%d", i),
			RefreshToken: fmt.Sprintf("old_refresh_%d", i),
			ExpiresAt:    expiresAt,
		}
		oauthJSON, _ := json.Marshal(oauthData)
		encryptedOAuth, _ := suite.crypto.Encrypt(string(oauthJSON))

		account := &data.Account{
			Name:               fmt.Sprintf("Account %d", i),
			Provider:           data.ProviderClaudeOfficial,
			Status:             data.StatusActive,
			HealthScore:        100,
			OAuthDataEncrypted: encryptedOAuth,
			OAuthExpiresAt:     &expiresAt,
			RpmLimit:           50,
			TpmLimit:           200000,
			Metadata:           "{}",
		}

		err := suite.accountRepo.CreateAccount(ctx, account)
		require.NoError(t, err)
		accountIDs[i] = account.ID
	}

	// Execute batch refresh (threshold: 10 minutes from now)
	start := time.Now()
	err := suite.uc.AutoRefreshTokens(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err)

	// Verify all 10 accounts were refreshed
	assert.Equal(t, 10, refreshCount)

	// Verify concurrent execution (should be much faster than 10 sequential calls)
	// With 5 concurrent workers, should take roughly 2 batches of time
	// Assuming each call takes ~100ms, sequential would be ~1s, concurrent should be <500ms
	t.Logf("Batch refresh of 10 accounts took: %v", elapsed)
	assert.Less(t, elapsed, 2*time.Second, "Concurrent refresh should be faster")

	// Verify all accounts were updated
	for _, accountID := range accountIDs {
		account, err := suite.accountRepo.GetAccount(ctx, accountID)
		require.NoError(t, err)

		// Decrypt OAuth data
		decrypted, err := suite.crypto.Decrypt(account.OAuthDataEncrypted)
		require.NoError(t, err)

		var newOAuth OAuthData
		err = json.Unmarshal([]byte(decrypted), &newOAuth)
		require.NoError(t, err)

		// Verify token was updated
		assert.Contains(t, newOAuth.AccessToken, "new_access_")
		assert.Contains(t, newOAuth.RefreshToken, "new_refresh_")

		// Verify expires_at updated
		require.NotNil(t, account.OAuthExpiresAt)
		assert.True(t, account.OAuthExpiresAt.After(time.Now().UTC()))
	}
}

// TestAutoRefreshTokens_PartialFailures tests batch refresh with some failures
func TestAutoRefreshTokens_PartialFailures(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.teardownTestSuite(t)

	ctx := context.Background()

	// Setup mock OAuth server that fails for specific refresh tokens
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		refreshToken := r.FormValue("refresh_token")

		// Fail for tokens containing "fail"
		if refreshToken == "fail_token" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}

		// Success for others
		resp := oauth.TokenResponse{
			AccessToken:  "new_access",
			RefreshToken: "new_refresh",
			ExpiresIn:    7200,
			Scope:        "claude:messages:read",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Create OAuth service with mock server endpoint
	mockOAuthSvc := oauth.NewOAuthServiceWithConfig(mockServer.URL+"/v1/oauth/token", 30*time.Second, 3)
	suite.uc = NewAccountUsecase(suite.accountRepo, suite.crypto, mockOAuthSvc, suite.rdb, suite.logger)

	expiresAt := time.Now().UTC().Add(5 * time.Minute)

	// Create 3 accounts: 2 success, 1 failure
	tokens := []string{"success_1", "fail_token", "success_2"}
	for _, token := range tokens {
		oauthData := OAuthData{
			AccessToken:  "access",
			RefreshToken: token,
			ExpiresAt:    expiresAt,
		}
		oauthJSON, _ := json.Marshal(oauthData)
		encryptedOAuth, _ := suite.crypto.Encrypt(string(oauthJSON))

		account := &data.Account{
			Name:               "Account " + token,
			Provider:           data.ProviderClaudeOfficial,
			Status:             data.StatusActive,
			HealthScore:        100,
			OAuthDataEncrypted: encryptedOAuth,
			OAuthExpiresAt:     &expiresAt,
			RpmLimit:           50,
			TpmLimit:           200000,
			Metadata:           "{}",
		}

		err := suite.accountRepo.CreateAccount(ctx, account)
		require.NoError(t, err)
	}

	// Execute batch refresh
	err := suite.uc.AutoRefreshTokens(ctx)

	// Should NOT return error (partial success is acceptable)
	assert.NoError(t, err)
}

// TestListExpiringAccounts tests query filtering
func TestListExpiringAccounts(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.teardownTestSuite(t)

	ctx := context.Background()

	now := time.Now().UTC()

	// Create test accounts with different expiry times and statuses
	testCases := []struct {
		name       string
		provider   data.AccountProvider
		status     data.AccountStatus
		expiresAt  *time.Time
		shouldFind bool
	}{
		{
			name:       "Expiring Claude Official",
			provider:   data.ProviderClaudeOfficial,
			status:     data.StatusActive,
			expiresAt:  ptrTime(now.Add(5 * time.Minute)),
			shouldFind: true,
		},
		{
			name:       "Expiring Claude Console",
			provider:   data.ProviderClaudeConsole,
			status:     data.StatusActive,
			expiresAt:  ptrTime(now.Add(8 * time.Minute)),
			shouldFind: true,
		},
		{
			name:       "Already Expired",
			provider:   data.ProviderClaudeOfficial,
			status:     data.StatusActive,
			expiresAt:  ptrTime(now.Add(-10 * time.Minute)),
			shouldFind: true,
		},
		{
			name:       "Not Expiring Soon",
			provider:   data.ProviderClaudeOfficial,
			status:     data.StatusActive,
			expiresAt:  ptrTime(now.Add(2 * time.Hour)),
			shouldFind: false,
		},
		{
			name:       "Inactive Account",
			provider:   data.ProviderClaudeOfficial,
			status:     data.StatusInactive,
			expiresAt:  ptrTime(now.Add(5 * time.Minute)),
			shouldFind: false,
		},
		{
			name:       "Wrong Provider (Gemini)",
			provider:   data.ProviderGemini,
			status:     data.StatusActive,
			expiresAt:  ptrTime(now.Add(5 * time.Minute)),
			shouldFind: false,
		},
		{
			name:       "No OAuth Data",
			provider:   data.ProviderClaudeOfficial,
			status:     data.StatusActive,
			expiresAt:  nil,
			shouldFind: false,
		},
	}

	for _, tc := range testCases {
		account := &data.Account{
			Name:           tc.name,
			Provider:       tc.provider,
			Status:         tc.status,
			HealthScore:    100,
			OAuthExpiresAt: tc.expiresAt,
			RpmLimit:       50,
			TpmLimit:       200000,
			Metadata:       "{}",
		}

		// Add minimal OAuth data for accounts that should have it
		if tc.expiresAt != nil {
			oauthData := OAuthData{
				AccessToken:  "access",
				RefreshToken: "refresh",
				ExpiresAt:    *tc.expiresAt,
			}
			oauthJSON, _ := json.Marshal(oauthData)
			encrypted, _ := suite.crypto.Encrypt(string(oauthJSON))
			account.OAuthDataEncrypted = encrypted
		}

		err := suite.accountRepo.CreateAccount(ctx, account)
		require.NoError(t, err)
	}

	// Query for accounts expiring in next 10 minutes
	threshold := now.Add(10 * time.Minute)
	accounts, err := suite.accountRepo.ListExpiringAccounts(ctx, threshold)
	require.NoError(t, err)

	// Verify results
	expectedCount := 0
	for _, tc := range testCases {
		if tc.shouldFind {
			expectedCount++
		}
	}

	assert.Equal(t, expectedCount, len(accounts))

	// Verify all returned accounts match criteria
	for _, account := range accounts {
		assert.True(t, account.Provider == data.ProviderClaudeOfficial ||
			account.Provider == data.ProviderClaudeConsole)
		assert.Equal(t, data.StatusActive, account.Status)
		assert.NotNil(t, account.OAuthExpiresAt)
		assert.True(t, account.OAuthExpiresAt.Before(threshold) ||
			account.OAuthExpiresAt.Equal(threshold))
	}
}

// Helper function to create time pointer
func ptrTime(t time.Time) *time.Time {
	return &t
}
