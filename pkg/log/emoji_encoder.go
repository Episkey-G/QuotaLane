package log

import (
	"fmt"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// emojiMap 定义日志类型到表情符号的映射
// 通过在日志调用时添加 "type" 字段，自动为日志添加对应的表情符号
var emojiMap = map[string]string{
	"api":          "🔗",
	"auth":         "🔓",
	"request":      "🌐",
	"success":      "✅",
	"error":        "❌",
	"warning":      "⚠️",
	"database":     "💾",
	"redis":        "📦",
	"rate_limit":   "🚦",
	"concurrency":  "⚡",
	"oauth":        "🔐",
	"token":        "🎫",
	"account":      "👤",
	"scheduler":    "🎯",
	"gateway":      "🚪",
	"startup":      "🚀",
	"performance":  "⏱️",
	"audit":        "📋",
	"security":     "🔒",
	"stream_usage": "📊",  // 流式请求 Token 使用统计
	"slow_request": "🐌",  // 慢请求警告
	"cache_stats":  "🧹",  // 缓存统计
	"error_count":  "⚠️", // 错误计数
}

// statusEmoji 根据 HTTP 状态码返回表情符号
func statusEmoji(status int) string {
	if status >= 500 {
		return "🔴"
	} else if status >= 400 {
		return "🟠"
	} else if status >= 300 {
		return "🟡"
	}
	return "🟢"
}

// EmojiConsoleEncoder 扩展 ConsoleEncoder，自动添加表情符号
// 这是一个零侵入的设计，通过包装 Zap 的 ConsoleEncoder 实现
type EmojiConsoleEncoder struct {
	zapcore.Encoder
	config zapcore.EncoderConfig
}

// NewEmojiConsoleEncoder 创建带表情符号的控制台编码器
func NewEmojiConsoleEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return &EmojiConsoleEncoder{
		Encoder: zapcore.NewConsoleEncoder(cfg),
		config:  cfg,
	}
}

// EncodeEntry 编码日志条目，自动添加表情符号
func (enc *EmojiConsoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// 提取 type 字段和 status 字段
	var logType string
	var status int64

	for _, field := range fields {
		if field.Key == "type" && field.Type == zapcore.StringType {
			logType = field.String
		} else if field.Key == "status" && (field.Type == zapcore.Int64Type || field.Type == zapcore.Int32Type) {
			status = field.Integer
		}
	}

	// 选择表情符号的优先级：
	// 1. HTTP status code (如果存在)
	// 2. type 字段映射
	// 3. 日志级别默认表情符号
	emoji := ""
	if status > 0 {
		emoji = statusEmoji(int(status))
	} else if logType != "" {
		if e, ok := emojiMap[logType]; ok {
			emoji = e
		}
	}

	// 如果还没有找到表情符号，使用日志级别的默认表情符号
	if emoji == "" {
		switch entry.Level {
		case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
			emoji = "❌"
		case zapcore.WarnLevel:
			emoji = "⚠️"
		case zapcore.InfoLevel:
			emoji = "ℹ️"
		case zapcore.DebugLevel:
			emoji = "🐛"
		}
	}

	// 修改 entry.Message 添加表情符号
	if emoji != "" {
		entry.Message = emoji + " " + entry.Message
	}

	// 调用原始 Encoder 进行实际编码
	return enc.Encoder.EncodeEntry(entry, fields)
}

// Clone 克隆编码器（Zap 内部使用）
func (enc *EmojiConsoleEncoder) Clone() zapcore.Encoder {
	return &EmojiConsoleEncoder{
		Encoder: enc.Encoder.Clone(),
		config:  enc.config,
	}
}

// AddEmojiToMap 允许外部添加自定义的表情符号映射
// 这提供了扩展性，用户可以在初始化时添加自定义类型
func AddEmojiToMap(logType, emoji string) {
	emojiMap[logType] = emoji
}

// GetEmojiMap 获取当前的表情符号映射（用于调试和测试）
func GetEmojiMap() map[string]string {
	// 返回副本，避免外部修改
	result := make(map[string]string, len(emojiMap))
	for k, v := range emojiMap {
		result[k] = v
	}
	return result
}

// formatDuration 格式化持续时间为易读格式
// 示例: 1ms, 150ms, 2.5s
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000.0
	return fmt.Sprintf("%.1fs", seconds)
}
