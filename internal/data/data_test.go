package data

import (
	"testing"
	"time"

	"QuotaLane/internal/conf"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestNewData_WithRedis(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Create config
	c := &conf.Data{
		Redis: &conf.Data_Redis{
			Addr:         mr.Addr(),
			ReadTimeout:  durationpb.New(200 * time.Millisecond),
			WriteTimeout: durationpb.New(200 * time.Millisecond),
		},
	}

	logger := log.DefaultLogger

	// Create Redis client
	rdb, redisCleanup, err := NewRedisClient(c, logger)
	require.NoError(t, err)
	require.NotNil(t, rdb)
	defer redisCleanup()

	// Create cache client
	cache := NewCacheClient(rdb)
	require.NotNil(t, cache)

	// Create Data
	data, cleanup, err := NewData(c, logger, rdb, cache)
	require.NoError(t, err)
	require.NotNil(t, data)
	defer cleanup()

	// Verify Data fields
	assert.NotNil(t, data.redisClient)
	assert.NotNil(t, data.cache)
}

func TestNewData_WithoutRedis(t *testing.T) {
	// Create config
	c := &conf.Data{}

	logger := log.DefaultLogger

	// Create Data with nil Redis client (graceful degradation)
	data, cleanup, err := NewData(c, logger, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, data)
	defer cleanup()

	// Verify Data handles nil Redis client
	assert.Nil(t, data.redisClient)
	assert.Nil(t, data.cache)
}

func TestData_GetCache(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create cache client
	cache := NewCacheClient(rdb)

	// Create config
	c := &conf.Data{}
	logger := log.DefaultLogger

	// Create Data
	data, cleanup, err := NewData(c, logger, rdb, cache)
	require.NoError(t, err)
	defer cleanup()

	// Get cache
	retrievedCache := data.GetCache()
	assert.NotNil(t, retrievedCache)
	assert.Equal(t, cache, retrievedCache)
}

func TestData_GetRedisClient(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create cache client
	cache := NewCacheClient(rdb)

	// Create config
	c := &conf.Data{}
	logger := log.DefaultLogger

	// Create Data
	data, cleanup, err := NewData(c, logger, rdb, cache)
	require.NoError(t, err)
	defer cleanup()

	// Get Redis client
	retrievedRdb := data.GetRedisClient()
	assert.NotNil(t, retrievedRdb)
	assert.Equal(t, rdb, retrievedRdb)
}

func TestData_GetCache_NilCache(t *testing.T) {
	// Create config
	c := &conf.Data{}
	logger := log.DefaultLogger

	// Create Data with nil cache
	data, cleanup, err := NewData(c, logger, nil, nil)
	require.NoError(t, err)
	defer cleanup()

	// Get cache should return nil
	cache := data.GetCache()
	assert.Nil(t, cache)
}

func TestData_GetRedisClient_NilClient(t *testing.T) {
	// Create config
	c := &conf.Data{}
	logger := log.DefaultLogger

	// Create Data with nil Redis client
	data, cleanup, err := NewData(c, logger, nil, nil)
	require.NoError(t, err)
	defer cleanup()

	// Get Redis client should return nil
	rdb := data.GetRedisClient()
	assert.Nil(t, rdb)
}
