package log

import (
	"os"
	"path/filepath"
	"testing"

	"QuotaLane/internal/conf"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewZapLogger_NilConfig(t *testing.T) {
	_, err := NewZapLogger(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "log config is nil")
}

func TestNewZapLogger_InvalidLevel(t *testing.T) {
	cfg := &conf.Log{
		Level:  "invalid_level",
		Format: "json",
	}

	_, err := NewZapLogger(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestNewZapLogger_ProductionMode(t *testing.T) {
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	logger, err := NewZapLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Test that logger has service field
	logger.Info("test message", zap.String("key", "value"))
}

func TestNewZapLogger_DevelopmentMode(t *testing.T) {
	cfg := &conf.Log{
		Level:  "debug",
		Format: "console",
		Env:    "development",
	}

	logger, err := NewZapLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Test debug level is enabled in development
	logger.Debug("debug message", zap.String("key", "value"))
	logger.Info("info message", zap.String("key", "value"))
}

func TestNewZapLogger_EnvironmentVariable(t *testing.T) {
	// Test QUOTALANE_ENV environment variable
	t.Setenv("QUOTALANE_ENV", "development")

	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "", // Empty, should use env var
	}

	logger, err := NewZapLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
}

func TestNewZapLogger_LogLevels(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"error level", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &conf.Log{
				Level:  tt.level,
				Format: "json",
				Env:    "production",
			}

			logger, err := NewZapLogger(cfg)
			require.NoError(t, err)
			require.NotNil(t, logger)

			// Test that logger can be created with different levels
			logger.Info("test message", zap.String("level", tt.level))
		})
	}
}

func TestNewZapLogger_FileOutput(t *testing.T) {
	// Create temporary directory for log files
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	cfg := &conf.Log{
		Level:      "info",
		Format:     "json",
		OutputFile: logFile,
		Env:        "production",
	}

	logger, err := NewZapLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Write some logs
	logger.Info("test message 1", zap.String("key", "value1"))
	logger.Info("test message 2", zap.String("key", "value2"))
	logger.Sync()

	// Verify log file was created
	_, err = os.Stat(logFile)
	assert.NoError(t, err, "log file should be created")

	// Verify log file has content
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test message 1")
	assert.Contains(t, string(content), "test message 2")
	assert.Contains(t, string(content), "QuotaLane") // service field
}

func TestNewZapLogger_JSONFormat(t *testing.T) {
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	logger, err := NewZapLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Verify logger uses JSON encoder
	// We can't directly test the output format here, but we can verify
	// the logger was created successfully with JSON config
}

func TestNewZapLogger_ConsoleFormat(t *testing.T) {
	cfg := &conf.Log{
		Level:  "info",
		Format: "console",
		Env:    "development",
	}

	logger, err := NewZapLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Verify logger uses Console encoder
	// We can't directly test the output format here, but we can verify
	// the logger was created successfully with console config
}

func TestNewZapLogger_ServiceField(t *testing.T) {
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	logger, err := NewZapLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Create temp file to capture output
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "service_test.log")
	cfg.OutputFile = logFile

	logger, err = NewZapLogger(cfg)
	require.NoError(t, err)

	logger.Info("test with service field")
	logger.Sync()

	// Read log file and verify service field
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "QuotaLane")
	assert.Contains(t, string(content), "\"service\":\"QuotaLane\"")
}

func TestLogLevelFiltering(t *testing.T) {
	tests := []struct {
		name           string
		configLevel    string
		shouldLogDebug bool
		shouldLogInfo  bool
		shouldLogWarn  bool
		shouldLogError bool
	}{
		{
			name:           "debug level logs everything",
			configLevel:    "debug",
			shouldLogDebug: true,
			shouldLogInfo:  true,
			shouldLogWarn:  true,
			shouldLogError: true,
		},
		{
			name:           "info level filters debug",
			configLevel:    "info",
			shouldLogDebug: false,
			shouldLogInfo:  true,
			shouldLogWarn:  true,
			shouldLogError: true,
		},
		{
			name:           "warn level filters debug and info",
			configLevel:    "warn",
			shouldLogDebug: false,
			shouldLogInfo:  false,
			shouldLogWarn:  true,
			shouldLogError: true,
		},
		{
			name:           "error level filters debug, info, warn",
			configLevel:    "error",
			shouldLogDebug: false,
			shouldLogInfo:  false,
			shouldLogWarn:  false,
			shouldLogError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			logFile := filepath.Join(tempDir, "level_test.log")

			cfg := &conf.Log{
				Level:      tt.configLevel,
				Format:     "json",
				OutputFile: logFile,
				Env:        "production",
			}

			logger, err := NewZapLogger(cfg)
			require.NoError(t, err)

			// Log at different levels
			logger.Debug("debug message")
			logger.Info("info message")
			logger.Warn("warn message")
			logger.Error("error message")
			logger.Sync()

			// Read log file
			content, err := os.ReadFile(logFile)
			require.NoError(t, err)
			logContent := string(content)

			// Verify expected logs are present/absent
			if tt.shouldLogDebug {
				assert.Contains(t, logContent, "debug message")
			} else {
				assert.NotContains(t, logContent, "debug message")
			}

			if tt.shouldLogInfo {
				assert.Contains(t, logContent, "info message")
			} else {
				assert.NotContains(t, logContent, "info message")
			}

			if tt.shouldLogWarn {
				assert.Contains(t, logContent, "warn message")
			} else {
				assert.NotContains(t, logContent, "warn message")
			}

			if tt.shouldLogError {
				assert.Contains(t, logContent, "error message")
			}
		})
	}
}

func TestLogFields(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "fields_test.log")

	cfg := &conf.Log{
		Level:      "info",
		Format:     "json",
		OutputFile: logFile,
		Env:        "production",
	}

	logger, err := NewZapLogger(cfg)
	require.NoError(t, err)

	// Log with various field types
	logger.Info("test message",
		zap.String("string_field", "value"),
		zap.Int("int_field", 123),
		zap.Bool("bool_field", true),
	)
	logger.Sync()

	// Read and verify log content
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	logContent := string(content)

	// Verify all fields are present
	assert.Contains(t, logContent, "test message")
	assert.Contains(t, logContent, "string_field")
	assert.Contains(t, logContent, "int_field")
	assert.Contains(t, logContent, "bool_field")
	assert.Contains(t, logContent, "timestamp")
	assert.Contains(t, logContent, "level")
	assert.Contains(t, logContent, "caller")
	assert.Contains(t, logContent, "service")
}
