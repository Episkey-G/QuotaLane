//go:build ignore
// +build ignore

package main

import (
	"QuotaLane/internal/conf"
	pkglog "QuotaLane/pkg/log"
)

func main() {
	// 创建日志配置
	logConf := &conf.Log{
		Level:  "debug",
		Format: "console", // 使用 console 格式以启用 Emoji Encoder
		Env:    "development",
	}

	// 创建 Zap logger
	zapLogger, err := pkglog.NewZapLogger(logConf)
	if err != nil {
		panic(err)
	}

	// 创建 Kratos adapter
	kratosLogger := pkglog.NewKratosAdapter(zapLogger)

	// 创建 LogHelper
	helper := pkglog.NewLogHelper(kratosLogger)

	// 测试各种日志类型
	println("=== 测试日志输出格式 ===\n")

	helper.Startup("QuotaLane service starting", "version", "1.0.0", "port", 8080)
	helper.API("Processing API request", "endpoint", "/api/v1/messages", "method", "POST")
	helper.Auth("User authenticated successfully", "user", "admin", "duration_ms", 15)
	helper.Request("POST", "/api/v1/messages", 200, 542, "ip", "192.168.1.100", "user_agent", "claude-cli/2.0.37")
	helper.Database("Query executed successfully", "table", "accounts", "duration_ms", 5)
	helper.Redis("Cache hit", "key", "account:123", "ttl", 3600)
	helper.OAuth("Token refreshed", "provider", "claude", "expires_in", 3600)
	helper.Token("API key validated", "key_id", "key-123")
	helper.Account("Account created", "account_id", "acc-456", "provider", "claude-official")
	helper.Scheduler("Account selected", "account_id", "acc-456", "score", 95)
	helper.Gateway("Request routed", "upstream", "claude-api", "duration_ms", 120)
	helper.Performance("Operation completed", "operation", "create_account", "duration_ms", 250)
	helper.Audit("Admin action", "admin", "root", "action", "delete_account")
	helper.Security("Suspicious activity detected", "ip", "10.0.0.1", "reason", "too many failed attempts")
	helper.Concurrency("Concurrency slot acquired", "account_id", "acc-456", "slots_used", 5)
	helper.Success("Request completed successfully", "request_id", "req-789")
	helper.RateLimit("Rate limit exceeded", "account_id", "acc-456", "limit", 100, "current", 105)

	// 测试便捷方法
	helper.AuthWithDuration("admin", "e076810a-6651-4b08-8b6c-649658e61396", 2)
	helper.RequestCompleted("admin", "ac6ba3ef-c1ce-41b1-bb2c-44d481702e89", "claude-sonnet-4-5-20250929", 1000, 500)

	println("\n=== 日志输出完成 ===")
}
