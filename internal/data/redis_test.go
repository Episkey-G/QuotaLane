package data

import (
	"QuotaLane/internal/conf"
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestNewRedisClient_Success(t *testing.T) {
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
	client, cleanup, err := NewRedisClient(c, logger)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer cleanup()

	// Verify connection with Ping
	ctx := context.Background()
	err = client.Ping(ctx).Err()
	assert.NoError(t, err)
}

func TestNewRedisClient_ConnectionFailure(t *testing.T) {
	// Use invalid address to simulate connection failure
	c := &conf.Data{
		Redis: &conf.Data_Redis{
			Addr:         "localhost:99999", // Invalid port
			ReadTimeout:  durationpb.New(200 * time.Millisecond),
			WriteTimeout: durationpb.New(200 * time.Millisecond),
		},
	}

	logger := log.DefaultLogger

	// Create Redis client (should not panic, graceful degradation)
	client, cleanup, err := NewRedisClient(c, logger)
	defer cleanup()

	// Should return error but not nil client
	assert.Error(t, err)
	assert.NotNil(t, client)

	// Connection should fail
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = client.Ping(ctx).Err()
	assert.Error(t, err)
}

func TestNewRedisClient_NilConfig(t *testing.T) {
	logger := log.DefaultLogger

	// Test with nil config
	client, cleanup, err := NewRedisClient(nil, logger)
	defer cleanup()

	assert.NoError(t, err)
	assert.Nil(t, client)
}

func TestNewRedisClient_EmptyAddress(t *testing.T) {
	logger := log.DefaultLogger

	c := &conf.Data{
		Redis: &conf.Data_Redis{
			Addr:         "", // Empty address
			ReadTimeout:  durationpb.New(200 * time.Millisecond),
			WriteTimeout: durationpb.New(200 * time.Millisecond),
		},
	}

	// Should handle empty address gracefully
	client, cleanup, err := NewRedisClient(c, logger)
	defer cleanup()

	assert.NoError(t, err)
	assert.Nil(t, client)
}

func TestNewRedisClient_PoolConfiguration(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	c := &conf.Data{
		Redis: &conf.Data_Redis{
			Addr:         mr.Addr(),
			ReadTimeout:  durationpb.New(200 * time.Millisecond),
			WriteTimeout: durationpb.New(200 * time.Millisecond),
		},
	}

	logger := log.DefaultLogger

	client, cleanup, err := NewRedisClient(c, logger)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer cleanup()

	// Verify pool configuration through Options
	opts := client.Options()
	assert.Equal(t, 100, opts.PoolSize, "PoolSize should be 100")
	assert.Equal(t, 10, opts.MinIdleConns, "MinIdleConns should be 10")
	assert.Equal(t, 3*time.Second, opts.DialTimeout, "DialTimeout should be 3s")
	assert.Equal(t, 200*time.Millisecond, opts.ReadTimeout, "ReadTimeout should match config")
	assert.Equal(t, 200*time.Millisecond, opts.WriteTimeout, "WriteTimeout should match config")
}

func TestNewRedisClient_CleanupFunction(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	c := &conf.Data{
		Redis: &conf.Data_Redis{
			Addr:         mr.Addr(),
			ReadTimeout:  durationpb.New(200 * time.Millisecond),
			WriteTimeout: durationpb.New(200 * time.Millisecond),
		},
	}

	logger := log.DefaultLogger

	client, cleanup, err := NewRedisClient(c, logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify client is connected
	ctx := context.Background()
	err = client.Ping(ctx).Err()
	require.NoError(t, err)

	// Call cleanup
	cleanup()

	// After cleanup, operations should fail
	err = client.Ping(ctx).Err()
	assert.Error(t, err)
}
