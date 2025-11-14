// Package oauth provides OAuth 2.0 utilities and Wire providers.
package oauth

import "github.com/google/wire"

// ProviderSet is the Wire provider set for OAuth services.
var ProviderSet = wire.NewSet(
	NewOAuthService,
)
