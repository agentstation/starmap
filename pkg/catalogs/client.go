package catalogs

import "context"

// Client defines the interface for provider API clients.
// Each provider implementation must satisfy this interface to fetch model information.
type Client interface {
	// ListModels retrieves all available models from the provider.
	ListModels(ctx context.Context) ([]Model, error)

	// isAPIKeyRequired returns true if the client requires an API key.
	IsAPIKeyRequired() bool

	// HasAPIKey returns true if the client has an API key.
	HasAPIKey() bool
}
