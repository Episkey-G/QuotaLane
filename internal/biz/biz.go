// Package biz contains business logic layer implementations.
// This layer holds the core business rules and domain models.
package biz

import (
	"QuotaLane/internal/data"

	"github.com/google/wire"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewAccountUsecase,
	NewOAuthRefreshTask,
	NewRateLimiterUseCase,
	NewCircuitBreakerUsecase,
	// Import data layer providers
	data.NewAccountRepo,
	data.NewRateLimitRepo,
	data.NewCircuitBreakerRepo,
	data.NewAuditLogger,
	data.NewNoopWebhookService,
	// Bind data layer implementations to biz layer interfaces
	wire.Bind(new(AccountRepo), new(*data.AccountRepo)),
	wire.Bind(new(RateLimitRepo), new(*data.RateLimitRepo)),
	wire.Bind(new(CircuitBreakerRepo), new(*data.CircuitBreakerRepo)),
	wire.Bind(new(AuditLogger), new(*data.AuditLoggerImpl)),
	wire.Bind(new(WebhookService), new(*data.NoopWebhookService)),
)
