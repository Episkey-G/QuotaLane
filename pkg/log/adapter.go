// Package log provides logging utilities for QuotaLane service.
// It includes a Zap logger wrapper with Kratos adapter and automatic field sanitization.
package log

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
)

// KratosAdapter adapts Zap logger to Kratos log.Logger interface
type KratosAdapter struct {
	zapLogger *zap.Logger
}

// NewKratosAdapter creates a new Kratos adapter for Zap logger
func NewKratosAdapter(zapLogger *zap.Logger) log.Logger {
	return &KratosAdapter{
		zapLogger: zapLogger,
	}
}

// Log implements Kratos log.Logger interface
func (a *KratosAdapter) Log(level log.Level, keyvals ...interface{}) error {
	if len(keyvals) == 0 {
		return nil
	}

	// Extract fields from keyvals
	fields := make([]zap.Field, 0, len(keyvals)/2)

	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			key := fmt.Sprint(keyvals[i])
			value := keyvals[i+1]

			// Apply sanitization for string values and use type-specific zap field constructors
			if strValue, ok := value.(string); ok {
				sanitized := SanitizeField(key, strValue)
				fields = append(fields, zap.String(key, sanitized))
			} else {
				// For non-string types, use zap.Any
				fields = append(fields, zap.Any(key, value))
			}
		}
	}

	// Map Kratos log level to Zap methods
	switch level {
	case log.LevelDebug:
		a.zapLogger.Debug("", fields...)
	case log.LevelInfo:
		a.zapLogger.Info("", fields...)
	case log.LevelWarn:
		a.zapLogger.Warn("", fields...)
	case log.LevelError:
		a.zapLogger.Error("", fields...)
	case log.LevelFatal:
		a.zapLogger.Fatal("", fields...)
	default:
		a.zapLogger.Info("", fields...)
	}

	return nil
}
