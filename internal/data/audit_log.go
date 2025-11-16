package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

// AuditLog is the GORM model for account_audit_logs table
type AuditLog struct {
	ID         int64     `gorm:"primaryKey;column:id"`
	AccountID  int64     `gorm:"column:account_id;not null;index"`
	ActionType string    `gorm:"column:action_type;type:varchar(50);not null"`
	Details    string    `gorm:"column:details;type:json"`              // JSON string
	OperatorID int64     `gorm:"column:operator_id;default:0;not null"` // 0 = system, >0 = admin
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName specifies the table name for GORM
func (AuditLog) TableName() string {
	return "account_audit_logs"
}

// AuditLoggerImpl implements biz.AuditLogger interface
type AuditLoggerImpl struct {
	db      *gorm.DB
	logChan chan *AuditLog
	logger  *log.Helper
}

// NewAuditLogger creates a new audit logger with async channel
func NewAuditLogger(db *gorm.DB, logger log.Logger) *AuditLoggerImpl {
	al := &AuditLoggerImpl{
		db:      db,
		logChan: make(chan *AuditLog, 1000), // Buffer size 1000 to prevent blocking
		logger:  log.NewHelper(logger),
	}

	// Start background goroutine for async logging
	go al.start()

	return al
}

// start processes audit log events from channel
func (a *AuditLoggerImpl) start() {
	for event := range a.logChan {
		ctx := context.Background()
		if err := a.db.WithContext(ctx).Create(event).Error; err != nil {
			a.logger.Errorw("failed to write audit log",
				"account_id", event.AccountID,
				"action_type", event.ActionType,
				"error", err)
		} else {
			a.logger.Debugw("audit log written",
				"account_id", event.AccountID,
				"action_type", event.ActionType)
		}
	}
}

// LogHealthScoreChange logs health score change event
func (a *AuditLoggerImpl) LogHealthScoreChange(ctx context.Context, accountID int64, oldScore, newScore int, reason string) {
	details := map[string]interface{}{
		"old_score": oldScore,
		"new_score": newScore,
		"reason":    reason,
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		a.logger.Errorw("failed to marshal audit log details", "error", err)
		return
	}

	event := &AuditLog{
		AccountID:  accountID,
		ActionType: string(AuditEventHealthScoreChanged),
		Details:    string(detailsJSON),
		OperatorID: 0, // System automatic
	}

	// Send to channel (non-blocking)
	select {
	case a.logChan <- event:
		// Successfully queued
	default:
		a.logger.Warnw("audit log channel full, dropping event",
			"account_id", accountID,
			"action_type", event.ActionType)
	}
}

// LogCircuitBroken logs circuit breaker triggered event
func (a *AuditLoggerImpl) LogCircuitBroken(ctx context.Context, accountID int64, healthScore int, brokenAt time.Time) {
	details := map[string]interface{}{
		"health_score":      healthScore,
		"circuit_broken_at": brokenAt.Format(time.RFC3339),
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		a.logger.Errorw("failed to marshal audit log details", "error", err)
		return
	}

	event := &AuditLog{
		AccountID:  accountID,
		ActionType: string(AuditEventCircuitBroken),
		Details:    string(detailsJSON),
		OperatorID: 0, // System automatic
	}

	select {
	case a.logChan <- event:
	default:
		a.logger.Warnw("audit log channel full, dropping event",
			"account_id", accountID,
			"action_type", event.ActionType)
	}
}

// LogCircuitRecovered logs circuit breaker recovered event
func (a *AuditLoggerImpl) LogCircuitRecovered(ctx context.Context, accountID int64, recoverTime time.Duration, probeCount int) {
	details := map[string]interface{}{
		"recover_time_seconds": recoverTime.Seconds(),
		"probe_count":          probeCount,
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		a.logger.Errorw("failed to marshal audit log details", "error", err)
		return
	}

	event := &AuditLog{
		AccountID:  accountID,
		ActionType: string(AuditEventCircuitRecovered),
		Details:    string(detailsJSON),
		OperatorID: 0, // System automatic
	}

	select {
	case a.logChan <- event:
	default:
		a.logger.Warnw("audit log channel full, dropping event",
			"account_id", accountID,
			"action_type", event.ActionType)
	}
}

// LogHealthScoreReset logs manual health score reset event
func (a *AuditLoggerImpl) LogHealthScoreReset(ctx context.Context, accountID int64, operatorID int64, oldScore int) {
	details := map[string]interface{}{
		"old_score": oldScore,
		"new_score": 100, // Always reset to 100
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		a.logger.Errorw("failed to marshal audit log details", "error", err)
		return
	}

	event := &AuditLog{
		AccountID:  accountID,
		ActionType: string(AuditEventHealthScoreReset),
		Details:    string(detailsJSON),
		OperatorID: operatorID, // Admin ID
	}

	select {
	case a.logChan <- event:
	default:
		a.logger.Warnw("audit log channel full, dropping event",
			"account_id", accountID,
			"action_type", event.ActionType)
	}
}
