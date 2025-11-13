package log

import (
	"fmt"
	"os"
	"strings"

	"QuotaLane/internal/conf"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewZapLogger creates a new Zap logger based on the provided configuration
func NewZapLogger(cfg *conf.Log) (*zap.Logger, error) {
	if cfg == nil {
		return nil, fmt.Errorf("log config is nil")
	}

	// Determine environment: use QUOTALANE_ENV env var if cfg.Env is empty
	env := cfg.Env
	if env == "" {
		env = os.Getenv("QUOTALANE_ENV")
		if env == "" {
			env = "production" // default to production
		}
	}

	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
	}

	// Create encoder config with required fields
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Choose encoder based on format
	var encoder zapcore.Encoder
	format := strings.ToLower(cfg.Format)
	if format == "console" || env == "development" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Create cores for different output targets
	var cores []zapcore.Core

	// Core 1: INFO+ (but below ERROR) → stdout
	stdoutCore := zapcore.NewCore(
		encoder,
		zapcore.Lock(os.Stdout),
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= level && lvl < zapcore.ErrorLevel
		}),
	)
	cores = append(cores, stdoutCore)

	// Core 2: ERROR+ → stderr
	stderrCore := zapcore.NewCore(
		encoder,
		zapcore.Lock(os.Stderr),
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.ErrorLevel
		}),
	)
	cores = append(cores, stderrCore)

	// Core 3: All logs → file with rotation (if output_file is specified)
	if cfg.OutputFile != "" {
		fileWriter := zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.OutputFile,
			MaxSize:    100, // megabytes
			MaxAge:     7,   // days
			MaxBackups: 7,
			Compress:   true,
		})

		fileCore := zapcore.NewCore(
			encoder,
			fileWriter,
			level, // use configured level for file output
		)
		cores = append(cores, fileCore)
	}

	// Combine all cores using Tee
	core := zapcore.NewTee(cores...)

	// Create logger with caller and stacktrace options
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(zap.String("service", "QuotaLane")),
	)

	return logger, nil
}
