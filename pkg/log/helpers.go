package log

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
)

// LogHelper 扩展 Kratos log.Helper，提供便捷的日志方法
// 通过在日志调用时自动添加 "type" 字段，触发 EmojiConsoleEncoder 的表情符号映射
type LogHelper struct {
	*log.Helper
}

// NewLogHelper 创建增强的日志辅助器
func NewLogHelper(logger log.Logger) *LogHelper {
	return &LogHelper{
		Helper: log.NewHelper(logger),
	}
}

// API 记录 API 相关日志（表情符号: 🔗）
func (h *LogHelper) API(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "api")
	h.Infow(allKvs...)
}

// Auth 记录认证相关日志（表情符号: 🔓）
func (h *LogHelper) Auth(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "auth")
	h.Infow(allKvs...)
}

// Request 记录 HTTP 请求日志（表情符号: 🌐 或根据状态码）
func (h *LogHelper) Request(method, url string, status int, durationMs int64, kvs ...interface{}) {
	msg := fmt.Sprintf("%s %s - %d (%dms)", method, url, status, durationMs)
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs,
		"type", "request",
		"method", method,
		"url", url,
		"status", status,
		"duration_ms", durationMs,
	)
	h.Infow(allKvs...)
}

// RateLimit 记录速率限制日志（表情符号: 🚦）
func (h *LogHelper) RateLimit(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "rate_limit")
	h.Warnw(allKvs...)
}

// Success 记录成功操作日志（表情符号: ✅）
func (h *LogHelper) Success(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "success")
	h.Infow(allKvs...)
}

// Database 记录数据库操作日志（表情符号: 💾）
func (h *LogHelper) Database(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "database")
	h.Debugw(allKvs...)
}

// Redis 记录 Redis 操作日志（表情符号: 📦）
func (h *LogHelper) Redis(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "redis")
	h.Debugw(allKvs...)
}

// OAuth 记录 OAuth 相关日志（表情符号: 🔐）
func (h *LogHelper) OAuth(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "oauth")
	h.Infow(allKvs...)
}

// Token 记录 Token 相关日志（表情符号: 🎫）
func (h *LogHelper) Token(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "token")
	h.Infow(allKvs...)
}

// Account 记录账户相关日志（表情符号: 👤）
func (h *LogHelper) Account(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "account")
	h.Infow(allKvs...)
}

// Scheduler 记录调度器相关日志（表情符号: 🎯）
func (h *LogHelper) Scheduler(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "scheduler")
	h.Infow(allKvs...)
}

// Gateway 记录网关相关日志（表情符号: 🚪）
func (h *LogHelper) Gateway(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "gateway")
	h.Infow(allKvs...)
}

// Startup 记录启动相关日志（表情符号: 🚀）
func (h *LogHelper) Startup(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "startup")
	h.Infow(allKvs...)
}

// Performance 记录性能相关日志（表情符号: ⏱️）
func (h *LogHelper) Performance(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "performance")
	h.Infow(allKvs...)
}

// Audit 记录审计日志（表情符号: 📋）
func (h *LogHelper) Audit(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "audit")
	h.Infow(allKvs...)
}

// Security 记录安全相关日志（表情符号: 🔒）
func (h *LogHelper) Security(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "security")
	h.Warnw(allKvs...)
}

// Concurrency 记录并发控制日志（表情符号: ⚡）
func (h *LogHelper) Concurrency(msg string, kvs ...interface{}) {
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "type", "concurrency")
	h.Infow(allKvs...)
}

// AuthWithDuration 记录带耗时的认证日志（便捷方法）
func (h *LogHelper) AuthWithDuration(keyName, keyID string, durationMs int64, kvs ...interface{}) {
	msg := fmt.Sprintf("Authenticated request from key: %s (%s) in %dms", keyName, keyID, durationMs)
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs, "key_name", keyName, "key_id", keyID, "duration_ms", durationMs, "type", "auth")
	h.Infow(allKvs...)
}

// RequestCompleted 记录请求完成日志（便捷方法）
func (h *LogHelper) RequestCompleted(keyName, accountID, model string, inputTokens, outputTokens int64, kvs ...interface{}) {
	msg := fmt.Sprintf("API request completed - Key: %s, Account: %s, Model: %s, Input: %d tokens, Output: %d tokens",
		keyName, accountID, model, inputTokens, outputTokens)
	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs,
		"key_name", keyName,
		"account_id", accountID,
		"model", model,
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"type", "success",
	)
	h.Infow(allKvs...)
}

// ========== Context-Aware 日志方法 ==========
// 以下方法自动从 Context 提取追踪信息（Request ID, Key Name, Account ID 等）

// StreamUsage 记录流式请求的 Token 使用统计（表情符号: 📊）
// 自动从 Context 提取 Request ID 和账户信息
func (h *LogHelper) StreamUsage(ctx context.Context, model string, inputTokens, outputTokens, cacheCreate, cacheRead int64, kvs ...interface{}) {
	reqCtx := GetRequestContext(ctx)
	totalTokens := inputTokens + outputTokens + cacheCreate + cacheRead

	msg := fmt.Sprintf("[%s] Stream usage recorded - Model: %s | Input: %d, Output: %d, Cache Create: %d, Cache Read: %d | Total: %d tokens",
		reqCtx.RequestID, model, inputTokens, outputTokens, cacheCreate, cacheRead, totalTokens)

	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs,
		"request_id", reqCtx.RequestID,
		"key_name", reqCtx.KeyName,
		"account_id", reqCtx.AccountID,
		"model", model,
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"cache_create_tokens", cacheCreate,
		"cache_read_tokens", cacheRead,
		"total_tokens", totalTokens,
		"type", "stream_usage",
	)
	h.Infow(allKvs...)
}

// SlowRequest 记录慢请求警告（表情符号: 🐌）
// threshold: 慢请求阈值（毫秒），超过此值触发警告
func (h *LogHelper) SlowRequest(ctx context.Context, method, url string, duration, threshold int64, kvs ...interface{}) {
	reqCtx := GetRequestContext(ctx)

	msg := fmt.Sprintf("[%s] Slow request detected | %s %s | %dms (threshold: %dms)",
		reqCtx.RequestID, method, url, duration, threshold)

	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs,
		"request_id", reqCtx.RequestID,
		"key_name", reqCtx.KeyName,
		"method", method,
		"url", url,
		"duration_ms", duration,
		"threshold_ms", threshold,
		"type", "slow_request",
	)
	h.Warnw(allKvs...)
}

// RequestWithContext 记录带 Context 的 HTTP 请求日志
// 自动从 Context 提取 Request ID 并检测慢请求
func (h *LogHelper) RequestWithContext(ctx context.Context, method, url string, status int, durationMs int64, kvs ...interface{}) {
	reqCtx := GetRequestContext(ctx)

	msg := fmt.Sprintf("%s %s - %d (%dms) | RequestID: %s",
		method, url, status, durationMs, reqCtx.RequestID)

	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs,
		"type", "request",
		"request_id", reqCtx.RequestID,
		"key_name", reqCtx.KeyName,
		"account_id", reqCtx.AccountID,
		"method", method,
		"url", url,
		"status", status,
		"duration_ms", durationMs,
	)
	h.Infow(allKvs...)

	// 自动检测慢请求（阈值 1000ms）
	if durationMs > 1000 {
		h.SlowRequest(ctx, method, url, durationMs, 1000)
	}
}

// CacheStats 记录缓存统计信息（表情符号: 🧹）
func (h *LogHelper) CacheStats(ctx context.Context, cacheName string, size, maxSize, hits, misses, evictions int64, kvs ...interface{}) {
	var hitRate float64
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	msg := fmt.Sprintf("Cache stats - %s | Size: %d/%d, Hit Rate: %.2f%%, Evictions: %d",
		cacheName, size, maxSize, hitRate, evictions)

	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs,
		"cache_name", cacheName,
		"size", size,
		"max_size", maxSize,
		"hits", hits,
		"misses", misses,
		"evictions", evictions,
		"hit_rate", fmt.Sprintf("%.2f%%", hitRate),
		"total_requests", total,
		"type", "cache_stats",
	)
	h.Infow(allKvs...)
}

// ErrorCount 记录错误计数（表情符号: ⚠️）
func (h *LogHelper) ErrorCount(ctx context.Context, errorType string, count int64, kvs ...interface{}) {
	reqCtx := GetRequestContext(ctx)

	msg := fmt.Sprintf("[%s] Error count - Type: %s, Count: %d",
		reqCtx.RequestID, errorType, count)

	allKvs := append([]interface{}{"msg", msg}, kvs...)
	allKvs = append(allKvs,
		"request_id", reqCtx.RequestID,
		"account_id", reqCtx.AccountID,
		"error_type", errorType,
		"count", count,
		"type", "error_count",
	)
	h.Warnw(allKvs...)
}

// APIWithContext 记录带 Context 的 API 日志
func (h *LogHelper) APIWithContext(ctx context.Context, msg string, kvs ...interface{}) {
	reqCtx := GetRequestContext(ctx)

	fullMsg := fmt.Sprintf("[%s] %s", reqCtx.RequestID, msg)

	allKvs := append([]interface{}{"msg", fullMsg}, kvs...)
	allKvs = append(allKvs,
		"request_id", reqCtx.RequestID,
		"key_name", reqCtx.KeyName,
		"type", "api",
	)
	h.Infow(allKvs...)
}
