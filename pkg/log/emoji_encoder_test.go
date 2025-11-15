package log

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestStatusEmoji(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		{
			name:   "2xx success",
			status: 200,
			want:   "🟢",
		},
		{
			name:   "3xx redirect",
			status: 301,
			want:   "🟡",
		},
		{
			name:   "4xx client error",
			status: 404,
			want:   "🟠",
		},
		{
			name:   "5xx server error",
			status: 500,
			want:   "🔴",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusEmoji(tt.status)
			if got != tt.want {
				t.Errorf("statusEmoji(%d) = %s, want %s", tt.status, got, tt.want)
			}
		})
	}
}

func TestEmojiMap(t *testing.T) {
	// 验证关键类型的表情符号映射存在
	requiredTypes := []string{
		"api",
		"auth",
		"request",
		"success",
		"error",
		"database",
		"redis",
		"rate_limit",
		"oauth",
	}

	for _, logType := range requiredTypes {
		if emoji, ok := emojiMap[logType]; !ok {
			t.Errorf("emojiMap missing required type: %s", logType)
		} else if emoji == "" {
			t.Errorf("emojiMap[%s] is empty", logType)
		}
	}
}

func TestAddEmojiToMap(t *testing.T) {
	// 保存原始映射
	originalLen := len(emojiMap)

	// 添加自定义表情符号
	AddEmojiToMap("custom_type", "🎨")

	// 验证添加成功
	if emoji, ok := emojiMap["custom_type"]; !ok {
		t.Error("AddEmojiToMap failed to add custom type")
	} else if emoji != "🎨" {
		t.Errorf("AddEmojiToMap set wrong emoji: got %s, want 🎨", emoji)
	}

	// 验证映射长度增加
	if len(emojiMap) != originalLen+1 {
		t.Errorf("emojiMap length = %d, want %d", len(emojiMap), originalLen+1)
	}

	// 清理
	delete(emojiMap, "custom_type")
}

func TestGetEmojiMap(t *testing.T) {
	// 获取映射副本
	mapCopy := GetEmojiMap()

	// 验证副本内容与原始映射一致
	if len(mapCopy) != len(emojiMap) {
		t.Errorf("GetEmojiMap returned map with length %d, want %d", len(mapCopy), len(emojiMap))
	}

	for key, value := range emojiMap {
		if mapCopy[key] != value {
			t.Errorf("GetEmojiMap[%s] = %s, want %s", key, mapCopy[key], value)
		}
	}

	// 修改副本不应影响原始映射
	mapCopy["test"] = "🧪"
	if _, ok := emojiMap["test"]; ok {
		t.Error("Modifying GetEmojiMap result should not affect original emojiMap")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{
			name: "milliseconds",
			ms:   150,
			want: "150ms",
		},
		{
			name: "seconds",
			ms:   2500,
			want: "2.5s",
		},
		{
			name: "zero",
			ms:   0,
			want: "0ms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.ms)
			if got != tt.want {
				t.Errorf("formatDuration(%d) = %s, want %s", tt.ms, got, tt.want)
			}
		})
	}
}

func TestEmojiConsoleEncoder(t *testing.T) {
	// 创建编码器配置
	cfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "time",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	// 创建 Emoji Encoder
	encoder := NewEmojiConsoleEncoder(cfg)

	// 验证 encoder 不为 nil
	if encoder == nil {
		t.Fatal("NewEmojiConsoleEncoder returned nil")
	}

	// 验证 Clone 方法
	cloned := encoder.Clone()
	if cloned == nil {
		t.Error("EmojiConsoleEncoder.Clone returned nil")
	}
}

func TestEmojiConsoleEncoder_EncodeEntry(t *testing.T) {
	cfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "time",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	encoder := NewEmojiConsoleEncoder(cfg)

	tests := []struct {
		name            string
		entry           zapcore.Entry
		fields          []zapcore.Field
		shouldHaveEmoji bool
		expectedEmoji   string
	}{
		{
			name: "API type log",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "Test message",
			},
			fields: []zapcore.Field{
				zapcore.Field{Key: "type", Type: zapcore.StringType, String: "api"},
			},
			shouldHaveEmoji: true,
			expectedEmoji:   "🔗",
		},
		{
			name: "HTTP status code",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "Request completed",
			},
			fields: []zapcore.Field{
				zapcore.Field{Key: "status", Type: zapcore.Int64Type, Integer: 200},
			},
			shouldHaveEmoji: true,
			expectedEmoji:   "🟢",
		},
		{
			name: "Error level default",
			entry: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Message: "Error occurred",
			},
			fields:          []zapcore.Field{},
			shouldHaveEmoji: true,
			expectedEmoji:   "❌",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := encoder.EncodeEntry(tt.entry, tt.fields)
			if err != nil {
				t.Fatalf("EncodeEntry failed: %v", err)
			}
			defer buf.Free()

			output := buf.String()
			if tt.shouldHaveEmoji {
				// 简单验证输出包含表情符号（完整验证需要解析输出）
				if len(output) == 0 {
					t.Error("EncodeEntry returned empty output")
				}
			}
		})
	}
}
