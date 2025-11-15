// Package oauth provides OAuth service handlers for different AI providers.
package oauth

import (
	"context"

	v1 "QuotaLane/api/v1"
)

// Handler defines the interface for OAuth provider-specific logic.
// Each AI provider (Claude, Codex, etc.) implements this interface.
type Handler interface {
	// GenerateAuthURL generates the OAuth authorization URL for this provider.
	GenerateAuthURL(ctx context.Context, req *v1.GenerateOAuthURLRequest) (*v1.GenerateOAuthURLResponse, error)

	// ExchangeCode exchanges the authorization code for tokens and creates an account.
	ExchangeCode(ctx context.Context, req *v1.ExchangeOAuthCodeRequest) (*v1.ExchangeOAuthCodeResponse, error)

	// ProviderType returns the provider type this handler supports.
	ProviderType() v1.AccountProvider
}
