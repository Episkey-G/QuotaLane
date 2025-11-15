package oauth

import (
	"context"
	"fmt"

	v1 "QuotaLane/api/v1"

	"github.com/go-kratos/kratos/v2/log"
)

// Registry manages OAuth handlers for different AI providers.
type Registry struct {
	handlers map[v1.AccountProvider]Handler
	logger   *log.Helper
}

// NewRegistry creates a new OAuth handler registry.
func NewRegistry(logger log.Logger) *Registry {
	return &Registry{
		handlers: make(map[v1.AccountProvider]Handler),
		logger:   log.NewHelper(logger),
	}
}

// Register registers an OAuth handler for a specific provider.
func (r *Registry) Register(handler Handler) {
	providerType := handler.ProviderType()
	r.handlers[providerType] = handler
	r.logger.Infof("Registered OAuth handler for provider: %s", providerType)
}

// GenerateAuthURL generates OAuth authorization URL using the appropriate handler.
func (r *Registry) GenerateAuthURL(ctx context.Context, req *v1.GenerateOAuthURLRequest) (*v1.GenerateOAuthURLResponse, error) {
	handler, ok := r.handlers[req.Provider]
	if !ok {
		return nil, fmt.Errorf("no OAuth handler registered for provider: %v", req.Provider)
	}

	return handler.GenerateAuthURL(ctx, req)
}

// ExchangeCode exchanges OAuth code using the appropriate handler.
// Note: Currently tries each handler sequentially.
// TODO: Optimize by storing provider type in OAuth session and retrieving it first.
func (r *Registry) ExchangeCode(ctx context.Context, req *v1.ExchangeOAuthCodeRequest) (*v1.ExchangeOAuthCodeResponse, error) {
	// Try each registered handler until one succeeds
	// Handlers will return error if session doesn't match their provider type
	var lastErr error

	for providerType, handler := range r.handlers {
		resp, err := handler.ExchangeCode(ctx, req)
		if err != nil {
			r.logger.Debugf("Handler %s cannot process session %s: %v", providerType, req.SessionId, err)
			lastErr = err
			continue
		}
		// Success
		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no OAuth handler could process this exchange request: %w", lastErr)
	}
	return nil, fmt.Errorf("no OAuth handler registered")
}

// GetHandler returns the handler for a specific provider.
func (r *Registry) GetHandler(provider v1.AccountProvider) (Handler, error) {
	handler, ok := r.handlers[provider]
	if !ok {
		return nil, fmt.Errorf("no OAuth handler registered for provider: %v", provider)
	}
	return handler, nil
}
