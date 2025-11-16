package data

import (
	"context"

	"QuotaLane/internal/model"

	"github.com/go-kratos/kratos/v2/log"
)

// NoopWebhookService is a Phase 1 implementation that only logs events
// Phase 2 will implement HTTPWebhookService with actual HTTP POST requests
type NoopWebhookService struct {
	logger *log.Helper
}

// NewNoopWebhookService creates a new noop webhook service
func NewNoopWebhookService(logger log.Logger) *NoopWebhookService {
	return &NoopWebhookService{
		logger: log.NewHelper(logger),
	}
}

// NotifyCircuitBroken logs circuit broken event (webhook disabled in Phase 1)
func (s *NoopWebhookService) NotifyCircuitBroken(ctx context.Context, event *model.CircuitBrokenEvent) error {
	s.logger.Infow("circuit broken (webhook disabled - Phase 1)",
		"account_id", event.AccountID,
		"account_name", event.AccountName,
		"health_score", event.HealthScore,
		"circuit_broken_at", event.CircuitBrokenAt)
	return nil
}

// NotifyCircuitRecovered logs circuit recovered event (webhook disabled in Phase 1)
func (s *NoopWebhookService) NotifyCircuitRecovered(ctx context.Context, event *model.CircuitRecoveredEvent) error {
	s.logger.Infow("circuit recovered (webhook disabled - Phase 1)",
		"account_id", event.AccountID,
		"account_name", event.AccountName,
		"probe_count", event.ProbeCount,
		"recover_time", event.RecoverTime)
	return nil
}
