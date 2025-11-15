package middleware

import (
	"context"
	"strings"
	"time"

	pkglog "QuotaLane/pkg/log"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// Logging 返回一个记录 HTTP 请求日志的中间件
// 自动生成 Request ID、检测慢请求、注入 Request Context
//
// 日志输出示例:
//
//	🟢 POST /api/v1/messages - 200 (542ms) | RequestID: mgrn0zfqda
//	🐌 [mgrn0zfqda] Slow request detected | POST /api/v1/messages | 13438ms
func Logging(logger *pkglog.LogHelper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			startTime := time.Now()

			var (
				method    string
				path      string
				ip        string
				userAgent string
				requestID string
				keyName   string
				accountID string
			)

			// 提取请求信息
			if tr, ok := transport.FromServerContext(ctx); ok {
				method = tr.Operation()
				path = tr.Operation()

				// 提取 HTTP 特定信息
				if ht, ok := tr.(http.Transporter); ok {
					httpReq := ht.Request()
					method = httpReq.Method
					path = httpReq.URL.Path
					if httpReq.URL.RawQuery != "" {
						path = path + "?" + httpReq.URL.RawQuery
					}

					// 提取客户端 IP
					ip = extractClientIP(httpReq)

					// 提取 User-Agent
					userAgent = httpReq.Header.Get("User-Agent")

					// 提取或生成 Request ID
					requestID = httpReq.Header.Get("X-Request-ID")
					if requestID == "" {
						requestID = pkglog.GenerateRequestID()
					}

					// 尝试从其他中间件（如 Auth）提取的信息
					// 这些信息可能在 Context 中已经存在
					if existingCtx := pkglog.GetRequestContext(ctx); existingCtx.RequestID != "unknown" {
						keyName = existingCtx.KeyName
						accountID = existingCtx.AccountID
					}
				}
			}

			// 将 Request Context 注入到 Context 中
			// 这样后续的所有日志调用都可以自动提取这些信息
			ctx = pkglog.WithRequestContext(ctx, requestID, keyName, "", accountID)

			// 执行实际的处理逻辑
			reply, err := handler(ctx, req)

			// 计算耗时
			duration := time.Since(startTime).Milliseconds()

			// 确定 HTTP 状态码
			status := 200
			if err != nil {
				// 从错误中提取状态码（Kratos 错误处理）
				status = extractHTTPStatus(err)
			}

			// 使用 Context-aware 日志方法
			logger.RequestWithContext(ctx, method, path, status, duration,
				"ip", ip,
				"user_agent", userAgent,
			)

			return reply, err
		}
	}
}

// extractClientIP 从请求中提取客户端真实 IP
// 优先级: X-Real-IP > X-Forwarded-For > RemoteAddr
func extractClientIP(req *http.Request) string {
	// 尝试从 X-Real-IP header 获取
	if ip := req.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// 尝试从 X-Forwarded-For header 获取（取第一个 IP）
	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 使用 RemoteAddr
	return req.RemoteAddr
}

// extractHTTPStatus 从 Kratos 错误中提取 HTTP 状态码
func extractHTTPStatus(err error) int {
	// 默认返回 500（内部错误）
	// TODO: 根据实际的错误类型映射到具体的 HTTP 状态码
	// 可以使用 Kratos 的 errors.FromError 提取错误码
	if err != nil {
		return 500
	}
	return 200
}

// generateRequestID 已移至 pkg/log/context.go
// 此处保留向后兼容性
func generateRequestID() string {
	return pkglog.GenerateRequestID()
}
