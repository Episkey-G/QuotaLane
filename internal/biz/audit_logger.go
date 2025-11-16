package biz

import (
	"context"
	"time"
)

// AuditEventType defines the type of audit event
type AuditEventType string

const (
	AuditEventHealthScoreChanged AuditEventType = "HEALTH_SCORE_CHANGED"
	AuditEventCircuitBroken      AuditEventType = "CIRCUIT_BROKEN"
	AuditEventCircuitRecovered   AuditEventType = "CIRCUIT_RECOVERED"
	AuditEventHealthScoreReset   AuditEventType = "HEALTH_SCORE_RESET"
)

// AuditLogger defines the interface for audit logging
type AuditLogger interface {
	// LogHealthScoreChange logs health score change event
	LogHealthScoreChange(ctx context.Context, accountID int64, oldScore, newScore int, reason string)

	// LogCircuitBroken logs circuit breaker triggered event
	LogCircuitBroken(ctx context.Context, accountID int64, healthScore int, brokenAt time.Time)

	// LogCircuitRecovered logs circuit breaker recovered event
	LogCircuitRecovered(ctx context.Context, accountID int64, recoverTime time.Duration, probeCount int)

	// LogHealthScoreReset logs manual health score reset event
	LogHealthScoreReset(ctx context.Context, accountID int64, operatorID int64, oldScore int)
}
