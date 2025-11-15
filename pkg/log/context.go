package log

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// contextKey 是用于存储 RequestContext 的私有 key 类型
type contextKey string

const requestContextKey contextKey = "quotalane_request_context"

// RequestContext 存储请求追踪信息
// 通过 Context 传递，实现跨函数、跨模块的请求追踪
type RequestContext struct {
	RequestID string                 // 唯一请求 ID (10位短ID，如 mgrn0zfqda)
	KeyName   string                 // API Key 名称
	KeyID     string                 // API Key ID
	AccountID string                 // 账户 ID
	StartTime time.Time              // 请求开始时间
	Metadata  map[string]interface{} // 扩展元数据
}

var (
	randSource = rand.NewSource(time.Now().UnixNano())
	randMutex  sync.Mutex
	// base36 字符集（小写字母 + 数字）
	base36Chars = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// GenerateRequestID 生成10位随机请求ID
// 格式: 小写字母+数字，例如 mgrn0zfqda
// 性能优化：使用 base36 编码，避免 UUID 的开销
func GenerateRequestID() string {
	randMutex.Lock()
	defer randMutex.Unlock()

	// 生成10位随机字符串
	b := make([]byte, 10)
	for i := range b {
		b[i] = base36Chars[randSource.Int63()%36]
	}
	return string(b)
}

// WithRequestContext 将 RequestContext 注入到 Context 中
// 通常在中间件中调用，为整个请求生命周期提供追踪信息
func WithRequestContext(ctx context.Context, requestID, keyName, keyID, accountID string) context.Context {
	reqCtx := &RequestContext{
		RequestID: requestID,
		KeyName:   keyName,
		KeyID:     keyID,
		AccountID: accountID,
		StartTime: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
	return context.WithValue(ctx, requestContextKey, reqCtx)
}

// GetRequestContext 从 Context 中提取 RequestContext
// 如果不存在，返回一个默认的空 RequestContext
func GetRequestContext(ctx context.Context) *RequestContext {
	if ctx == nil {
		return &RequestContext{
			RequestID: "unknown",
			Metadata:  make(map[string]interface{}),
		}
	}

	if reqCtx, ok := ctx.Value(requestContextKey).(*RequestContext); ok {
		return reqCtx
	}

	// 返回默认值，避免 nil 检查
	return &RequestContext{
		RequestID: "unknown",
		Metadata:  make(map[string]interface{}),
	}
}

// GetRequestID 从 Context 中提取 Request ID
// 便捷方法，避免调用者需要处理 RequestContext 结构
func GetRequestID(ctx context.Context) string {
	return GetRequestContext(ctx).RequestID
}

// GetKeyName 从 Context 中提取 Key Name
func GetKeyName(ctx context.Context) string {
	return GetRequestContext(ctx).KeyName
}

// GetAccountID 从 Context 中提取 Account ID
func GetAccountID(ctx context.Context) string {
	return GetRequestContext(ctx).AccountID
}

// SetMetadata 设置 RequestContext 的元数据
// 用于在请求处理过程中添加额外的追踪信息
func SetMetadata(ctx context.Context, key string, value interface{}) {
	reqCtx := GetRequestContext(ctx)
	if reqCtx.Metadata == nil {
		reqCtx.Metadata = make(map[string]interface{})
	}
	reqCtx.Metadata[key] = value
}

// GetMetadata 获取 RequestContext 的元数据
func GetMetadata(ctx context.Context, key string) (interface{}, bool) {
	reqCtx := GetRequestContext(ctx)
	if reqCtx.Metadata == nil {
		return nil, false
	}
	value, ok := reqCtx.Metadata[key]
	return value, ok
}

// GetElapsedTime 获取请求已执行时间（毫秒）
func GetElapsedTime(ctx context.Context) int64 {
	reqCtx := GetRequestContext(ctx)
	if reqCtx.StartTime.IsZero() {
		return 0
	}
	return time.Since(reqCtx.StartTime).Milliseconds()
}
