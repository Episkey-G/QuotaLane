package biz

import (
	"context"

	"QuotaLane/internal/model"
)

// WebhookService defines the interface for webhook notifications
type WebhookService interface {
	// NotifyCircuitBroken sends notification when circuit breaker is triggered
	NotifyCircuitBroken(ctx context.Context, event *model.CircuitBrokenEvent) error

	// NotifyCircuitRecovered sends notification when circuit breaker recovers
	NotifyCircuitRecovered(ctx context.Context, event *model.CircuitRecoveredEvent) error
}
