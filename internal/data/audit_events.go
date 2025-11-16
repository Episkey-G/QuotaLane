package data

// AuditEventType defines audit event type constants.
// These constants are used for audit logging in account_audit_logs table.
type AuditEventType string

const (
	// AuditEventHealthScoreChanged is logged when account health score changes
	AuditEventHealthScoreChanged AuditEventType = "HEALTH_SCORE_CHANGED"

	// AuditEventCircuitBroken is logged when circuit breaker is triggered
	AuditEventCircuitBroken AuditEventType = "CIRCUIT_BROKEN"

	// AuditEventCircuitRecovered is logged when circuit breaker recovers
	AuditEventCircuitRecovered AuditEventType = "CIRCUIT_RECOVERED"

	// AuditEventHealthScoreReset is logged when admin manually resets health score
	AuditEventHealthScoreReset AuditEventType = "HEALTH_SCORE_RESET"
)

// String returns the string representation of AuditEventType
func (e AuditEventType) String() string {
	return string(e)
}
