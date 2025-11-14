// Package conf provides configuration management using Viper.
// It supports loading configuration from YAML files and environment variables,
// with CLI flag overrides.
package conf

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/protobuf/types/known/durationpb"
)

// NewBootstrap creates and initializes a Bootstrap configuration.
// It loads configuration from the specified config file path, applies defaults,
// and allows overrides from environment variables prefixed with QUOTALANE_.
//
// Configuration priority: CLI flags > Environment variables > Config file > Defaults
//
// Required environment variables:
//   - MYSQL_DSN or QUOTALANE_DATA_DATABASE_SOURCE: MySQL connection string
//   - JWT_SECRET or QUOTALANE_AUTH_JWT_SECRET: JWT signing secret
//   - ENCRYPTION_KEY or QUOTALANE_AUTH_ENCRYPTION_KEY: Data encryption key
//
// Parameters:
//   - configPath: Path to the configuration file or directory
//
// Returns:
//   - *Bootstrap: Loaded configuration
//   - error: Configuration loading or validation error
func NewBootstrap(configPath string) (*Bootstrap, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Enable environment variable support with QUOTALANE_ prefix
	v.SetEnvPrefix("QUOTALANE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Allow direct environment variable names (without QUOTALANE_ prefix) for compatibility
	// Bind specific environment variables for required fields
	_ = v.BindEnv("data.database.source", "MYSQL_DSN", "QUOTALANE_DATA_DATABASE_SOURCE")
	_ = v.BindEnv("data.redis.addr", "QUOTALANE_DATA_REDIS_ADDR")
	_ = v.BindEnv("auth.jwt.secret", "JWT_SECRET", "QUOTALANE_AUTH_JWT_SECRET")
	_ = v.BindEnv("auth.encryption.key", "ENCRYPTION_KEY", "QUOTALANE_AUTH_ENCRYPTION_KEY")

	// Load configuration file
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			// If config file is specified but not found, return error
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}
	}

	// Parse configuration into Bootstrap structure
	bc := &Bootstrap{
		Server: &Server{
			Http: &Server_HTTP{
				Network: v.GetString("server.http.network"),
				Addr:    v.GetString("server.http.addr"),
				Timeout: durationpb.New(v.GetDuration("server.http.timeout")),
			},
			Grpc: &Server_GRPC{
				Network: v.GetString("server.grpc.network"),
				Addr:    v.GetString("server.grpc.addr"),
				Timeout: durationpb.New(v.GetDuration("server.grpc.timeout")),
			},
		},
		Data: &Data{
			Database: &Data_Database{
				Driver: v.GetString("data.database.driver"),
				Source: v.GetString("data.database.source"),
			},
			Redis: &Data_Redis{
				Network:      v.GetString("data.redis.network"),
				Addr:         v.GetString("data.redis.addr"),
				ReadTimeout:  durationpb.New(v.GetDuration("data.redis.read_timeout")),
				WriteTimeout: durationpb.New(v.GetDuration("data.redis.write_timeout")),
			},
		},
		Auth: &Auth{
			Jwt: &Auth_JWT{
				Secret:  v.GetString("auth.jwt.secret"),
				Expires: durationpb.New(v.GetDuration("auth.jwt.expires")),
			},
			Encryption: &Auth_Encryption{
				Key: v.GetString("auth.encryption.key"),
			},
		},
		Log: &Log{
			Level:  v.GetString("log.level"),
			Format: v.GetString("log.format"),
		},
	}

	// Validate required fields
	if err := Validate(bc); err != nil {
		return nil, err
	}

	return bc, nil
}

// setDefaults sets default configuration values.
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.http.network", "tcp")
	v.SetDefault("server.http.addr", ":8080")
	v.SetDefault("server.http.timeout", 10*time.Minute)

	v.SetDefault("server.grpc.network", "tcp")
	v.SetDefault("server.grpc.addr", ":9000")
	v.SetDefault("server.grpc.timeout", 10*time.Minute)

	// Data defaults
	v.SetDefault("data.database.driver", "mysql")
	// Note: data.database.source (MYSQL_DSN) is required from environment

	v.SetDefault("data.redis.network", "tcp")
	v.SetDefault("data.redis.addr", "127.0.0.1:6379")
	v.SetDefault("data.redis.read_timeout", 200*time.Millisecond)
	v.SetDefault("data.redis.write_timeout", 200*time.Millisecond)

	// Auth defaults
	// Note: auth.jwt.secret and auth.encryption.key are required from environment
	v.SetDefault("auth.jwt.expires", 24*time.Hour)

	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
}

// Validate checks that all required configuration fields are present and valid.
// It returns an error listing all missing required fields.
func Validate(bc *Bootstrap) error {
	var missingFields []string

	// Check required database configuration
	if bc.Data == nil || bc.Data.Database == nil || bc.Data.Database.Source == "" {
		missingFields = append(missingFields, "data.database.source (MYSQL_DSN)")
	}

	// Check required auth configuration
	if bc.Auth == nil || bc.Auth.Jwt == nil || bc.Auth.Jwt.Secret == "" {
		missingFields = append(missingFields, "auth.jwt.secret (JWT_SECRET)")
	}

	if bc.Auth == nil || bc.Auth.Encryption == nil || bc.Auth.Encryption.Key == "" {
		missingFields = append(missingFields, "auth.encryption.key (ENCRYPTION_KEY)")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required configuration fields: %s", strings.Join(missingFields, ", "))
	}

	return nil
}
