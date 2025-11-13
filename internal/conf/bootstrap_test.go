package conf

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBootstrap_Defaults(t *testing.T) {
	// Create a minimal valid config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `server:
  http:
    addr: :8080
  grpc:
    addr: :9000
data:
  database:
    driver: mysql
  redis:
    addr: 127.0.0.1:6379
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set required environment variables
	t.Setenv("MYSQL_DSN", "user:pass@tcp(localhost:3306)/testdb")
	t.Setenv("JWT_SECRET", "test-jwt-secret-key")
	t.Setenv("ENCRYPTION_KEY", "test-encryption-key-12345678")

	// Load configuration
	bc, err := NewBootstrap(configPath)
	require.NoError(t, err)
	require.NotNil(t, bc)

	// Verify server defaults
	assert.Equal(t, ":8080", bc.Server.Http.Addr)
	assert.Equal(t, "tcp", bc.Server.Http.Network)
	assert.Equal(t, 10*time.Minute, bc.Server.Http.Timeout.AsDuration())

	assert.Equal(t, ":9000", bc.Server.Grpc.Addr)
	assert.Equal(t, "tcp", bc.Server.Grpc.Network)
	assert.Equal(t, 10*time.Minute, bc.Server.Grpc.Timeout.AsDuration())

	// Verify data defaults
	assert.Equal(t, "mysql", bc.Data.Database.Driver)
	assert.Equal(t, "user:pass@tcp(localhost:3306)/testdb", bc.Data.Database.Source)

	assert.Equal(t, "127.0.0.1:6379", bc.Data.Redis.Addr)
	assert.Equal(t, "tcp", bc.Data.Redis.Network)
	assert.Equal(t, 200*time.Millisecond, bc.Data.Redis.ReadTimeout.AsDuration())
	assert.Equal(t, 200*time.Millisecond, bc.Data.Redis.WriteTimeout.AsDuration())

	// Verify auth values from environment
	assert.Equal(t, "test-jwt-secret-key", bc.Auth.Jwt.Secret)
	assert.Equal(t, 24*time.Hour, bc.Auth.Jwt.Expires.AsDuration())
	assert.Equal(t, "test-encryption-key-12345678", bc.Auth.Encryption.Key)

	// Verify log defaults
	assert.Equal(t, "info", bc.Log.Level)
	assert.Equal(t, "json", bc.Log.Format)
}

func TestNewBootstrap_EnvOverrides(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectedVal func(*Bootstrap) bool
		description string
	}{
		{
			name: "override_http_addr",
			envVars: map[string]string{
				"CLAUDDY_SERVER_HTTP_ADDR": ":9999",
				"MYSQL_DSN":                "user:pass@tcp(localhost:3306)/testdb",
				"JWT_SECRET":               "test-jwt-secret",
				"ENCRYPTION_KEY":           "test-encryption-key-1234",
			},
			expectedVal: func(bc *Bootstrap) bool {
				return bc.Server.Http.Addr == ":9999"
			},
			description: "CLAUDDY_SERVER_HTTP_ADDR should override default :8080",
		},
		{
			name: "override_redis_addr",
			envVars: map[string]string{
				"CLAUDDY_DATA_REDIS_ADDR": "redis.example.com:6379",
				"MYSQL_DSN":               "user:pass@tcp(localhost:3306)/testdb",
				"JWT_SECRET":              "test-jwt-secret",
				"ENCRYPTION_KEY":          "test-encryption-key-1234",
			},
			expectedVal: func(bc *Bootstrap) bool {
				return bc.Data.Redis.Addr == "redis.example.com:6379"
			},
			description: "CLAUDDY_DATA_REDIS_ADDR should override default",
		},
		{
			name: "override_log_level",
			envVars: map[string]string{
				"CLAUDDY_LOG_LEVEL": "debug",
				"MYSQL_DSN":         "user:pass@tcp(localhost:3306)/testdb",
				"JWT_SECRET":        "test-jwt-secret",
				"ENCRYPTION_KEY":    "test-encryption-key-1234",
			},
			expectedVal: func(bc *Bootstrap) bool {
				return bc.Log.Level == "debug"
			},
			description: "CLAUDDY_LOG_LEVEL should override default info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			configContent := `server:
  http:
    addr: :8080
data:
  redis:
    addr: 127.0.0.1:6379
`
			err := os.WriteFile(configPath, []byte(configContent), 0644)
			require.NoError(t, err)

			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Load configuration
			bc, err := NewBootstrap(configPath)
			require.NoError(t, err, tt.description)
			require.NotNil(t, bc)

			// Verify expected override
			assert.True(t, tt.expectedVal(bc), tt.description)
		})
	}
}

func TestNewBootstrap_MissingRequired(t *testing.T) {
	tests := []struct {
		name          string
		envVars       map[string]string
		expectedError string
	}{
		{
			name: "missing_mysql_dsn",
			envVars: map[string]string{
				"JWT_SECRET":     "test-jwt-secret",
				"ENCRYPTION_KEY": "test-encryption-key",
			},
			expectedError: "data.database.source (MYSQL_DSN)",
		},
		{
			name: "missing_jwt_secret",
			envVars: map[string]string{
				"MYSQL_DSN":      "user:pass@tcp(localhost:3306)/testdb",
				"ENCRYPTION_KEY": "test-encryption-key",
			},
			expectedError: "auth.jwt.secret (JWT_SECRET)",
		},
		{
			name: "missing_encryption_key",
			envVars: map[string]string{
				"MYSQL_DSN":  "user:pass@tcp(localhost:3306)/testdb",
				"JWT_SECRET": "test-jwt-secret",
			},
			expectedError: "auth.encryption.key (ENCRYPTION_KEY)",
		},
		{
			name:          "missing_all_required",
			envVars:       map[string]string{},
			expectedError: "missing required configuration fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			configContent := `server:
  http:
    addr: :8080
`
			err := os.WriteFile(configPath, []byte(configContent), 0644)
			require.NoError(t, err)

			// Clear all relevant environment variables first to ensure isolation
			os.Unsetenv("MYSQL_DSN")
			os.Unsetenv("CLAUDDY_DATA_DATABASE_SOURCE")
			os.Unsetenv("JWT_SECRET")
			os.Unsetenv("CLAUDDY_AUTH_JWT_SECRET")
			os.Unsetenv("ENCRYPTION_KEY")
			os.Unsetenv("CLAUDDY_AUTH_ENCRYPTION_KEY")

			// Set only the environment variables specified for this test
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Load configuration - should fail
			bc, err := NewBootstrap(configPath)
			if err == nil {
				t.Logf("Bootstrap unexpectedly succeeded. Auth: %+v", bc.Auth)
			}
			assert.Error(t, err, "Expected error for missing required fields")
			assert.Nil(t, bc, "Bootstrap should be nil when validation fails")
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestNewBootstrap_ConfigFileNotFound(t *testing.T) {
	// Set required environment variables
	t.Setenv("MYSQL_DSN", "user:pass@tcp(localhost:3306)/testdb")
	t.Setenv("JWT_SECRET", "test-jwt-secret")
	t.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	// Try to load non-existent config file
	bc, err := NewBootstrap("/non/existent/config.yaml")
	assert.Error(t, err)
	assert.Nil(t, bc)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestNewBootstrap_EmptyConfigPath(t *testing.T) {
	// Set required environment variables
	t.Setenv("MYSQL_DSN", "user:pass@tcp(localhost:3306)/testdb")
	t.Setenv("JWT_SECRET", "test-jwt-secret")
	t.Setenv("ENCRYPTION_KEY", "test-encryption-key-1234")

	// Load with empty config path (should use defaults + env vars)
	bc, err := NewBootstrap("")
	require.NoError(t, err)
	require.NotNil(t, bc)

	// Verify defaults were applied
	assert.Equal(t, ":8080", bc.Server.Http.Addr)
	assert.Equal(t, ":9000", bc.Server.Grpc.Addr)
	assert.Equal(t, "user:pass@tcp(localhost:3306)/testdb", bc.Data.Database.Source)
	assert.Equal(t, "test-jwt-secret", bc.Auth.Jwt.Secret)
	assert.Equal(t, "test-encryption-key-1234", bc.Auth.Encryption.Key)
}

func TestNewBootstrap_PriorityOrder(t *testing.T) {
	// Create config file with one value
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `server:
  http:
    addr: :7777
data:
  redis:
    addr: 127.0.0.1:6379
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment variable that should override file value
	t.Setenv("CLAUDDY_SERVER_HTTP_ADDR", ":8888")
	t.Setenv("MYSQL_DSN", "user:pass@tcp(localhost:3306)/testdb")
	t.Setenv("JWT_SECRET", "test-jwt-secret")
	t.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	// Load configuration
	bc, err := NewBootstrap(configPath)
	require.NoError(t, err)
	require.NotNil(t, bc)

	// Environment variable should win over file value
	assert.Equal(t, ":8888", bc.Server.Http.Addr, "Environment variable should override config file")
}

func TestValidate_AllFieldsPresent(t *testing.T) {
	bc := &Bootstrap{
		Server: &Server{
			Http: &Server_HTTP{Addr: ":8080"},
			Grpc: &Server_GRPC{Addr: ":9000"},
		},
		Data: &Data{
			Database: &Data_Database{
				Driver: "mysql",
				Source: "user:pass@tcp(localhost:3306)/testdb",
			},
			Redis: &Data_Redis{Addr: "127.0.0.1:6379"},
		},
		Auth: &Auth{
			Jwt:        &Auth_JWT{Secret: "test-jwt-secret"},
			Encryption: &Auth_Encryption{Key: "test-encryption-key"},
		},
		Log: &Log{
			Level:  "info",
			Format: "json",
		},
	}

	err := Validate(bc)
	assert.NoError(t, err)
}

func TestValidate_NilBootstrap(t *testing.T) {
	err := Validate(&Bootstrap{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required configuration fields")
}
