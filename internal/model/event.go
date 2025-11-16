package model

import "time"

// CircuitBrokenEvent represents a circuit breaker triggered event
type CircuitBrokenEvent struct {
	AccountID       int64
	AccountName     string
	HealthScore     int
	CircuitBrokenAt time.Time
}

// CircuitRecoveredEvent represents a circuit breaker recovered event
type CircuitRecoveredEvent struct {
	AccountID   int64
	AccountName string
	ProbeCount  int
	RecoverTime time.Duration
}

// CircuitState represents the current circuit breaker state
type CircuitState struct {
	IsCircuitBroken  bool
	CircuitBrokenAt  *time.Time
	IsHalfOpen       bool
	SuccessCount     int
	BackoffRetryTime time.Time // 下次允许试探的时间
}
