package openai

import "github.com/google/wire"

// ProviderSet is openai providers.
var ProviderSet = wire.NewSet(NewOpenAIService)
