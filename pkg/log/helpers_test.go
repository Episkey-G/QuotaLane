package log

import (
	"bytes"
	"os"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// createTestLogger 创建用于测试的日志记录器
func createTestLogger() (*LogHelper, *bytes.Buffer) {
	// 创建内存缓冲区捕获日志输出
	buf := &bytes.Buffer{}

	// 创建简单的编码器配置
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		TimeKey:     "time",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
		EncodeTime:  zapcore.ISO8601TimeEncoder,
	}

	// 创建 Core
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(buf),
		zapcore.DebugLevel,
	)

	// 创建 Zap logger
	zapLogger := zap.New(core)

	// 创建 Kratos adapter
	kratosLogger := NewKratosAdapter(zapLogger)

	// 创建 LogHelper
	helper := NewLogHelper(kratosLogger)

	return helper, buf
}

func TestNewLogHelper(t *testing.T) {
	zapLogger := zap.NewNop()
	kratosLogger := NewKratosAdapter(zapLogger)
	helper := NewLogHelper(kratosLogger)

	if helper == nil {
		t.Fatal("NewLogHelper returned nil")
	}
}

func TestLogHelper_API(t *testing.T) {
	helper, buf := createTestLogger()

	helper.API("test API call", "endpoint", "/v1/test")

	output := buf.String()
	if output == "" {
		t.Error("API log produced no output")
	}

	// 验证输出包含 type:api 字段
	if !contains(output, "api") {
		t.Error("API log missing 'api' type field")
	}
}

func TestLogHelper_Auth(t *testing.T) {
	helper, buf := createTestLogger()

	helper.Auth("authentication successful", "user", "admin")

	output := buf.String()
	if output == "" {
		t.Error("Auth log produced no output")
	}

	if !contains(output, "auth") {
		t.Error("Auth log missing 'auth' type field")
	}
}

func TestLogHelper_Request(t *testing.T) {
	helper, buf := createTestLogger()

	helper.Request("POST", "/api/v1/messages", 200, 150)

	output := buf.String()
	if output == "" {
		t.Error("Request log produced no output")
	}

	// 验证输出包含关键字段
	if !contains(output, "POST") {
		t.Error("Request log missing method")
	}
	if !contains(output, "200") {
		t.Error("Request log missing status code")
	}
}

func TestLogHelper_Success(t *testing.T) {
	helper, buf := createTestLogger()

	helper.Success("operation completed", "operation", "create_account")

	output := buf.String()
	if output == "" {
		t.Error("Success log produced no output")
	}

	if !contains(output, "success") {
		t.Error("Success log missing 'success' type field")
	}
}

func TestLogHelper_RateLimit(t *testing.T) {
	helper, buf := createTestLogger()

	helper.RateLimit("rate limit exceeded", "account_id", "123")

	output := buf.String()
	if output == "" {
		t.Error("RateLimit log produced no output")
	}

	if !contains(output, "rate_limit") {
		t.Error("RateLimit log missing 'rate_limit' type field")
	}
}

func TestLogHelper_Database(t *testing.T) {
	helper, buf := createTestLogger()

	helper.Database("query executed", "table", "accounts")

	output := buf.String()
	if output == "" {
		t.Error("Database log produced no output")
	}

	if !contains(output, "database") {
		t.Error("Database log missing 'database' type field")
	}
}

func TestLogHelper_Redis(t *testing.T) {
	helper, buf := createTestLogger()

	helper.Redis("cache hit", "key", "account:123")

	output := buf.String()
	if output == "" {
		t.Error("Redis log produced no output")
	}

	if !contains(output, "redis") {
		t.Error("Redis log missing 'redis' type field")
	}
}

func TestLogHelper_OAuth(t *testing.T) {
	helper, buf := createTestLogger()

	helper.OAuth("token refreshed", "provider", "claude")

	output := buf.String()
	if output == "" {
		t.Error("OAuth log produced no output")
	}

	if !contains(output, "oauth") {
		t.Error("OAuth log missing 'oauth' type field")
	}
}

func TestLogHelper_AuthWithDuration(t *testing.T) {
	helper, buf := createTestLogger()

	helper.AuthWithDuration("admin", "key-123", 5)

	output := buf.String()
	if output == "" {
		t.Error("AuthWithDuration log produced no output")
	}

	// 验证包含关键信息
	if !contains(output, "admin") {
		t.Error("AuthWithDuration log missing key name")
	}
	if !contains(output, "key-123") {
		t.Error("AuthWithDuration log missing key ID")
	}
}

func TestLogHelper_RequestCompleted(t *testing.T) {
	helper, buf := createTestLogger()

	helper.RequestCompleted("admin", "account-456", "claude-sonnet-4", 1000, 500)

	output := buf.String()
	if output == "" {
		t.Error("RequestCompleted log produced no output")
	}

	// 验证包含关键信息
	if !contains(output, "admin") {
		t.Error("RequestCompleted log missing key name")
	}
	if !contains(output, "account-456") {
		t.Error("RequestCompleted log missing account ID")
	}
	if !contains(output, "claude-sonnet-4") {
		t.Error("RequestCompleted log missing model")
	}
}

func TestLogHelper_AllTypes(t *testing.T) {
	// 测试所有日志类型方法都能正常调用
	helper, _ := createTestLogger()

	// 不应该 panic
	helper.Account("account created")
	helper.Scheduler("account selected")
	helper.Gateway("request routed")
	helper.Startup("service started")
	helper.Performance("operation took 100ms")
	helper.Audit("admin action")
	helper.Security("suspicious activity")
	helper.Concurrency("slot acquired")
	helper.Token("token validated")
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestMain 设置测试环境
func TestMain(m *testing.M) {
	// 运行测试
	code := m.Run()
	os.Exit(code)
}
