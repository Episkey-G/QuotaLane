package data

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAccount is a test struct for serialization
type TestAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Balance  int    `json:"balance"`
	IsActive bool   `json:"is_active"`
}

func setupTestCache(t *testing.T) (CacheClient, *miniredis.Miniredis) {
	// Start miniredis server
	mr := miniredis.RunT(t)

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create cache client
	cache := NewCacheClient(rdb)

	return cache, mr
}

func TestNewCacheClient(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewCacheClient(rdb)
	assert.NotNil(t, cache)
}

func TestCacheGet_Success(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Prepare test data
	account := TestAccount{
		ID:       "123",
		Name:     "Test Account",
		Balance:  1000,
		IsActive: true,
	}

	// Set value first
	key := BuildCacheKey(CacheKeyAccount, "123")
	err := cache.Set(ctx, key, account, TTLAccount)
	require.NoError(t, err)

	// Get value
	var retrieved TestAccount
	err = cache.Get(ctx, key, &retrieved)
	require.NoError(t, err)

	// Verify data
	assert.Equal(t, account.ID, retrieved.ID)
	assert.Equal(t, account.Name, retrieved.Name)
	assert.Equal(t, account.Balance, retrieved.Balance)
	assert.Equal(t, account.IsActive, retrieved.IsActive)
}

func TestCacheGet_KeyNotFound(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Try to get non-existent key
	var retrieved TestAccount
	err := cache.Get(ctx, "nonexistent:key", &retrieved)

	// Should return ErrCacheNotFound
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCacheNotFound)
}

func TestCacheGet_InvalidJSON(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Set invalid JSON manually
	key := "test:invalid"
	_ = mr.Set(key, "invalid json {{{") // Intentionally set invalid data for testing

	// Try to get and deserialize
	var retrieved TestAccount
	err := cache.Get(ctx, key, &retrieved)

	// Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestCacheSet_Success(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	account := TestAccount{
		ID:       "456",
		Name:     "Another Account",
		Balance:  2000,
		IsActive: false,
	}

	key := BuildCacheKey(CacheKeyAccount, "456")
	err := cache.Set(ctx, key, account, TTLAccount)
	require.NoError(t, err)

	// Verify key exists in miniredis
	exists := mr.Exists(key)
	assert.True(t, exists)
}

func TestCacheSet_WithTTL(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	account := TestAccount{ID: "789", Name: "TTL Test"}

	key := BuildCacheKey(CacheKeyAccount, "789")
	ttl := 1 * time.Second

	err := cache.Set(ctx, key, account, ttl)
	require.NoError(t, err)

	// Verify TTL is set in miniredis
	currentTTL := mr.TTL(key)
	assert.Greater(t, currentTTL, time.Duration(0))
	assert.LessOrEqual(t, currentTTL, ttl)
}

func TestCacheDelete_Success(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Set a value first
	account := TestAccount{ID: "111", Name: "Delete Test"}
	key := BuildCacheKey(CacheKeyAccount, "111")
	err := cache.Set(ctx, key, account, TTLAccount)
	require.NoError(t, err)

	// Verify key exists
	exists := mr.Exists(key)
	assert.True(t, exists)

	// Delete the key
	err = cache.Delete(ctx, key)
	require.NoError(t, err)

	// Verify key is deleted
	exists = mr.Exists(key)
	assert.False(t, exists)
}

func TestCacheDelete_NonExistentKey(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Delete non-existent key should not error
	err := cache.Delete(ctx, "nonexistent:key")
	assert.NoError(t, err)
}

func TestCacheExists_KeyExists(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Set a value
	account := TestAccount{ID: "222", Name: "Exists Test"}
	key := BuildCacheKey(CacheKeyUser, "222")
	err := cache.Set(ctx, key, account, TTLUser)
	require.NoError(t, err)

	// Check existence
	exists, err := cache.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCacheExists_KeyNotExists(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Check non-existent key
	exists, err := cache.Exists(ctx, "nonexistent:key")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestBuildCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		prefix   string
		parts    []string
	}{
		{
			name:     "account key",
			prefix:   CacheKeyAccount,
			parts:    []string{"123"},
			expected: "account:123",
		},
		{
			name:     "user key",
			prefix:   CacheKeyUser,
			parts:    []string{"456"},
			expected: "user:456",
		},
		{
			name:     "session key",
			prefix:   CacheKeySession,
			parts:    []string{"token123"},
			expected: "session:token123",
		},
		{
			name:     "sticky session key",
			prefix:   CacheKeySticky,
			parts:    []string{"hash456"},
			expected: "sticky:hash456",
		},
		{
			name:     "rate limit key with multiple parts",
			prefix:   CacheKeyRate,
			parts:    []string{"keyId123", "1m"},
			expected: "rate:keyId123:1m",
		},
		{
			name:     "plan key",
			prefix:   CacheKeyPlan,
			parts:    []string{"premium"},
			expected: "plan:premium",
		},
		{
			name:     "no parts",
			prefix:   CacheKeyAccount,
			parts:    []string{},
			expected: "account",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildCacheKey(tt.prefix, tt.parts...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCacheClient_AllKeyTypes(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Test all 6 key types from AC#2
	tests := []struct {
		name   string
		prefix string
		id     string
		ttl    time.Duration
	}{
		{"account", CacheKeyAccount, "acc1", TTLAccount},
		{"user", CacheKeyUser, "user1", TTLUser},
		{"session", CacheKeySession, "sess1", TTLSession},
		{"sticky", CacheKeySticky, "stick1", TTLSticky},
		{"rate", CacheKeyRate, "rate1", TTLRate},
		{"plan", CacheKeyPlan, "plan1", TTLPlan},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data
			data := map[string]interface{}{
				"id":   tt.id,
				"type": tt.name,
			}

			// Set cache
			key := BuildCacheKey(tt.prefix, tt.id)
			err := cache.Set(ctx, key, data, tt.ttl)
			require.NoError(t, err)

			// Get cache
			var retrieved map[string]interface{}
			err = cache.Get(ctx, key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, tt.id, retrieved["id"])
			assert.Equal(t, tt.name, retrieved["type"])

			// Check existence
			exists, err := cache.Exists(ctx, key)
			require.NoError(t, err)
			assert.True(t, exists)

			// Delete cache
			err = cache.Delete(ctx, key)
			require.NoError(t, err)

			// Verify deletion
			exists, err = cache.Exists(ctx, key)
			require.NoError(t, err)
			assert.False(t, exists)
		})
	}
}

func TestCacheTTLExpiration(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Set cache with short TTL
	account := TestAccount{ID: "expire", Name: "Expire Test"}
	key := BuildCacheKey(CacheKeyAccount, "expire")
	shortTTL := 100 * time.Millisecond

	err := cache.Set(ctx, key, account, shortTTL)
	require.NoError(t, err)

	// Verify key exists
	exists, err := cache.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

	// Fast forward time in miniredis
	mr.FastForward(200 * time.Millisecond)

	// Key should be expired now
	exists, err = cache.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)

	// Get should return ErrCacheNotFound
	var retrieved TestAccount
	err = cache.Get(ctx, key, &retrieved)
	assert.ErrorIs(t, err, ErrCacheNotFound)
}

func TestCacheClient_NilRedisClient(t *testing.T) {
	// Create cache with nil Redis client
	cache := NewCacheClient(nil)
	ctx := context.Background()

	// All operations should return error gracefully
	account := TestAccount{ID: "test"}

	err := cache.Set(ctx, "key", account, TTLAccount)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")

	var retrieved TestAccount
	err = cache.Get(ctx, "key", &retrieved)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")

	err = cache.Delete(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")

	exists, err := cache.Exists(ctx, "key")
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "redis client is nil")
}

func TestCacheClient_ComplexStructSerialization(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Test complex nested struct
	type Address struct {
		Street  string `json:"street"`
		City    string `json:"city"`
		ZipCode string `json:"zip_code"`
	}

	type ComplexUser struct {
		CreatedAt time.Time         `json:"created_at"`
		Addresses []Address         `json:"addresses"`
		Metadata  map[string]string `json:"metadata"`
		ID        string            `json:"id"`
		Name      string            `json:"name"`
	}

	original := ComplexUser{
		ID:   "complex1",
		Name: "Complex User",
		Addresses: []Address{
			{Street: "123 Main St", City: "Boston", ZipCode: "02101"},
			{Street: "456 Oak Ave", City: "Cambridge", ZipCode: "02139"},
		},
		Metadata: map[string]string{
			"role":   "admin",
			"status": "active",
		},
		CreatedAt: time.Now().Round(time.Second), // Round to second for JSON comparison
	}

	key := BuildCacheKey(CacheKeyUser, "complex1")

	// Set
	err := cache.Set(ctx, key, original, TTLUser)
	require.NoError(t, err)

	// Get
	var retrieved ComplexUser
	err = cache.Get(ctx, key, &retrieved)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, original.ID, retrieved.ID)
	assert.Equal(t, original.Name, retrieved.Name)
	assert.Equal(t, len(original.Addresses), len(retrieved.Addresses))
	assert.Equal(t, original.Addresses[0].Street, retrieved.Addresses[0].Street)
	assert.Equal(t, original.Metadata["role"], retrieved.Metadata["role"])
	assert.True(t, original.CreatedAt.Equal(retrieved.CreatedAt))
}
