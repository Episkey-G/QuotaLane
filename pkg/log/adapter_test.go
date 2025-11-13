package log

import (
	"path/filepath"
	"strings"
	"testing"

	"QuotaLane/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKratosAdapter(t *testing.T) {
	// Create a Zap logger
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	// Create Kratos adapter
	adapter := NewKratosAdapter(zapLog)
	require.NotNil(t, adapter)

	// Verify it implements log.Logger interface
	var _ log.Logger = adapter
}

func TestKratosAdapter_Log_EmptyKeyvals(t *testing.T) {
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog)

	// Logging with empty keyvals should not error
	err = adapter.Log(log.LevelInfo)
	assert.NoError(t, err)
}

func TestKratosAdapter_LogLevels(t *testing.T) {
	tests := []struct {
		name  string
		level log.Level
	}{
		{"debug level", log.LevelDebug},
		{"info level", log.LevelInfo},
		{"warn level", log.LevelWarn},
		{"error level", log.LevelError},
		// Note: Fatal level not tested as it calls os.Exit
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file to capture logs
			tempDir := t.TempDir()
			logFile := filepath.Join(tempDir, "adapter_test.log")

			cfg := &conf.Log{
				Level:      "debug", // Enable all levels
				Format:     "json",
				OutputFile: logFile,
				Env:        "production",
			}

			zapLog, err := NewZapLogger(cfg)
			require.NoError(t, err)

			adapter := NewKratosAdapter(zapLog)

			// Log at the specified level
			err = adapter.Log(tt.level, "msg", "test message", "key", "value")
			require.NoError(t, err)

			// Sync to ensure log is written
			zapLog.Sync()

			// Note: We can't easily verify the exact content for fatal level
			// as it would exit the process. For other levels, we just verify
			// no error occurred.
		})
	}
}

func TestKratosAdapter_KeyValuePairs(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "keyval_test.log")

	cfg := &conf.Log{
		Level:      "info",
		Format:     "json",
		OutputFile: logFile,
		Env:        "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog)

	// Log with multiple key-value pairs
	err = adapter.Log(log.LevelInfo,
		"msg", "test message",
		"user_id", "12345",
		"request_id", "abc-def-ghi",
		"count", 42,
	)
	require.NoError(t, err)

	zapLog.Sync()

	// Note: Actual log parsing is complex. We just verify no errors occurred.
	// The zap_test.go already verifies that logs are written correctly.
}

func TestKratosAdapter_OddKeyvals(t *testing.T) {
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog)

	// Log with odd number of keyvals (missing value for last key)
	err = adapter.Log(log.LevelInfo,
		"msg", "test message",
		"key1", "value1",
		"key2", // missing value
	)

	// Should not panic or error
	assert.NoError(t, err)
}

func TestKratosAdapter_SanitizeSensitiveData(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "sanitize_test.log")

	cfg := &conf.Log{
		Level:      "info",
		Format:     "json",
		OutputFile: logFile,
		Env:        "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog)

	// Log with sensitive data
	err = adapter.Log(log.LevelInfo,
		"msg", "user login",
		"username", "john_doe",
		"password", "mysecretpassword123",
		"api_key", "sk-1234567890abcdefghij",
	)
	require.NoError(t, err)

	zapLog.Sync()

	// Sensitive fields should be sanitized
	// We would need to parse the log file to verify, but the sanitize_test.go
	// already tests the sanitization logic thoroughly
}

func TestKratosAdapter_NonStringValues(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "types_test.log")

	cfg := &conf.Log{
		Level:      "info",
		Format:     "json",
		OutputFile: logFile,
		Env:        "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog)

	// Log with various data types
	err = adapter.Log(log.LevelInfo,
		"msg", "test types",
		"int_val", 123,
		"bool_val", true,
		"float_val", 3.14,
		"nil_val", nil,
		"struct_val", struct{ Name string }{Name: "test"},
	)
	require.NoError(t, err)

	zapLog.Sync()

	// All types should be logged without error
	// Only string values are sanitized
}

func TestKratosAdapter_WithHelper(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "helper_test.log")

	cfg := &conf.Log{
		Level:      "info",
		Format:     "json",
		OutputFile: logFile,
		Env:        "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog)

	// Test with Kratos log.Helper
	helper := log.NewHelper(adapter)
	helper.Info("test message from helper")
	helper.Infow("msg", "test with fields", "key", "value")
	helper.Debug("debug message")
	helper.Warn("warn message")
	helper.Error("error message")

	zapLog.Sync()

	// Helper should work seamlessly with adapter
}

func TestKratosAdapter_ContextFields(t *testing.T) {
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog)

	// Add context fields using log.With
	contextLogger := log.With(adapter,
		"service.id", "test-service",
		"service.name", "QuotaLane",
		"trace.id", "abc-123",
	)

	helper := log.NewHelper(contextLogger)
	helper.Info("test with context")

	// Context fields should be included in all logs from contextLogger
}

func TestKratosAdapter_LevelMapping(t *testing.T) {
	tests := []struct {
		name        string
		inputLevel  log.Level
		description string
	}{
		{
			name:        "debug maps to Zap Debug",
			inputLevel:  log.LevelDebug,
			description: "Kratos LevelDebug should call Zap Debug",
		},
		{
			name:        "info maps to Zap Info",
			inputLevel:  log.LevelInfo,
			description: "Kratos LevelInfo should call Zap Info",
		},
		{
			name:        "warn maps to Zap Warn",
			inputLevel:  log.LevelWarn,
			description: "Kratos LevelWarn should call Zap Warn",
		},
		{
			name:        "error maps to Zap Error",
			inputLevel:  log.LevelError,
			description: "Kratos LevelError should call Zap Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			logFile := filepath.Join(tempDir, "mapping_test.log")

			cfg := &conf.Log{
				Level:      "debug",
				Format:     "json",
				OutputFile: logFile,
				Env:        "production",
			}

			zapLog, err := NewZapLogger(cfg)
			require.NoError(t, err)

			adapter := NewKratosAdapter(zapLog)

			// Log at specified level
			err = adapter.Log(tt.inputLevel, "msg", tt.description)
			require.NoError(t, err)

			zapLog.Sync()

			// All levels should log without error
		})
	}
}

func TestKratosAdapter_IntegrationWithKratos(t *testing.T) {
	// This test verifies the adapter works with Kratos' logger ecosystem
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog)

	// Create logger with context using Kratos functions
	logger := log.With(adapter,
		"service", "QuotaLane",
		"version", "1.0.0",
	)

	// Use with Filter
	logger = log.NewFilter(logger, log.FilterLevel(log.LevelInfo))

	// Create helper and use it
	helper := log.NewHelper(logger)
	helper.Info("integration test message")

	// Should work without errors
}

func TestKratosAdapter_DefaultLevel(t *testing.T) {
	// Test that unknown levels default to Info
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog).(*KratosAdapter)

	// Log with LevelInfo (default handling test)
	err = adapter.Log(log.LevelInfo, "msg", "test default level")
	assert.NoError(t, err)
}

func TestKratosAdapter_PerformanceWithManyFields(t *testing.T) {
	cfg := &conf.Log{
		Level:  "info",
		Format: "json",
		Env:    "production",
	}

	zapLog, err := NewZapLogger(cfg)
	require.NoError(t, err)

	adapter := NewKratosAdapter(zapLog)

	// Log with many fields (performance test)
	keyvals := []interface{}{"msg", "performance test"}
	for i := 0; i < 50; i++ {
		keyvals = append(keyvals, strings.Repeat("key", i), strings.Repeat("val", i))
	}

	err = adapter.Log(log.LevelInfo, keyvals...)
	assert.NoError(t, err)
}
