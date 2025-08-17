package catalogs

import "context"

// Client defines the interface for provider API clients.
// Each provider implementation must satisfy this interface to fetch model information.
type Client interface {
	// ListModels retrieves all available models from the provider.
	ListModels(ctx context.Context) ([]Model, error)

	// GetModel retrieves a specific model by its ID.
	GetModel(ctx context.Context, modelID string) (*Model, error)
}
