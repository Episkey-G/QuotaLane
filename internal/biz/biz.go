// Package biz contains business logic layer implementations.
// This layer holds the core business rules and domain models.
package biz

import "github.com/google/wire"

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewAccountUsecase,
)
