// Package middleware provides HTTP middleware for authentication, logging, and request processing.
package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	pkglog "QuotaLane/pkg/log"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// apiKeyContextKey is the context key for storing API key
	apiKeyContextKey contextKey = "api_key"
	// apiKeyMaskedContextKey is the context key for storing masked API key
	apiKeyMaskedContextKey contextKey = "api_key_masked"
)

// Auth 返回一个 HTTP 认证中间件
// 提取并验证 API Key，记录详细的认证日志
//
// 日志输出示例:
//
//	🔗 🔓 Authenticated request from key: admin (e076810a-6651-4b08-8b6c-649658e61396) in 2ms | {"type":"auth","key_id":"...","duration_ms":2}
//	🔗    User-Agent: "claude-cli/2.0.37 (external, claude-vscode, agent-sdk/0.1.37)" | {"type":"api","user_agent":"..."}
//
// 注意: 当前为简化实现，实际的 API Key 验证逻辑将在后续 Story 中实现
func Auth(logger *pkglog.LogHelper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			startTime := time.Now()

			var (
				apiKey    string
				userAgent string
			)

			// 提取 Authorization header 和 User-Agent
			if tr, ok := transport.FromServerContext(ctx); ok {
				if ht, ok := tr.(http.Transporter); ok {
					req := ht.Request()

					// 提取 Authorization header
					authHeader := req.Header.Get("Authorization")
					if authHeader != "" {
						// 支持 "Bearer {token}" 格式
						apiKey = strings.TrimPrefix(authHeader, "Bearer ")
						apiKey = strings.TrimSpace(apiKey)
					}

					// 如果 Authorization header 为空，尝试从 X-API-Key header 获取
					if apiKey == "" {
						apiKey = req.Header.Get("X-API-Key")
					}

					// 提取 User-Agent
					userAgent = req.Header.Get("User-Agent")
				}
			}

			// 如果存在 API Key，记录认证日志
			if apiKey != "" {
				// TODO: 在后续 Story 中实现实际的 API Key 验证逻辑
				// 当前仅记录日志，不做实际验证

				// 计算认证耗时
				authDuration := time.Since(startTime).Milliseconds()

				// 脱敏 API Key（仅显示前 8 位）
				maskedKey := maskAPIKey(apiKey)

				// 记录认证成功日志（模拟）
				logger.Auth(
					"Authenticated request from key: [masked] ("+maskedKey+") in "+formatDuration(authDuration),
					"api_key_masked", maskedKey,
					"duration_ms", authDuration,
				)

				// 记录 User-Agent（独立一行，更易读）
				if userAgent != "" {
					logger.API(
						"   User-Agent: \""+userAgent+"\"",
						"user_agent", userAgent,
					)
				}

				// 将 API Key 信息注入上下文（供后续处理使用）
				ctx = context.WithValue(ctx, apiKeyContextKey, apiKey)
				ctx = context.WithValue(ctx, apiKeyMaskedContextKey, maskedKey)

				// 尝试从已有的 Request Context 中提取信息并更新
				// 如果 Logging 中间件已经创建了 Request Context，我们可以复用
				// 否则这里的信息会在后续的 Logging 中间件中被使用
				reqCtx := pkglog.GetRequestContext(ctx)
				if reqCtx.RequestID != "unknown" {
					// Request Context 已存在（可能来自 Logging 中间件）
					// 注意：Context 是不可变的，我们需要创建新的 Context
					// 这里我们通过 Metadata 来传递 Key 信息
					pkglog.SetMetadata(ctx, "api_key_masked", maskedKey)
				}
			}

			// 执行后续处理
			return handler(ctx, req)
		}
	}
}

// maskAPIKey 脱敏 API Key，仅显示前 8 位
// 示例: "sk-1234567890abcdef" -> "sk-12345***"
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		// 如果 key 太短，全部脱敏
		return strings.Repeat("*", len(key))
	}

	// 显示前 8 位，其余用 *** 代替
	return key[:8] + "***"
}

// formatDuration 格式化持续时间为易读格式
// 示例: 5ms, 150ms, 2.5s
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000.0
	return fmt.Sprintf("%.1fs", seconds)
}
