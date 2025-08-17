package googleaistudio

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/sources/base"
	"github.com/agentstation/starmap/internal/sources/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func init() {
	// Register this provider client in the registry
	registry.RegisterClient(catalogs.ProviderIDGoogleAIStudio, &Client{})
}

// Client implements the catalogs.Client interface for Google AI Studio.
type Client struct {
	*base.GoogleClient
}

// NewClient creates a new Google AI Studio client (kept for backward compatibility).
func NewClient(apiKey string, provider *catalogs.Provider) *Client {
	provider.APIKeyValue = apiKey // Set the API key in the provider
	return &Client{
		GoogleClient: base.NewGoogleClient(provider, "https://generativelanguage.googleapis.com/v1beta/models"),
	}
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	c.GoogleClient = base.NewGoogleClient(provider, "https://generativelanguage.googleapis.com/v1beta/models")
}

// ListModels uses the base Google implementation.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	return c.GoogleClient.ListModels(ctx)
}

// GetModel uses the base Google implementation.
func (c *Client) GetModel(ctx context.Context, modelID string) (*catalogs.Model, error) {
	return c.GoogleClient.GetModel(ctx, modelID)
}

// ExtractModelID extracts the model ID from the full name for Google AI Studio.
func (c *Client) ExtractModelID(name string) string {
	// Extract model ID from name (format: models/gemini-pro)
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// ConvertToModel overrides the base implementation for Google AI Studio specific logic.
func (c *Client) ConvertToModel(m base.GoogleModelData) catalogs.Model {
	model := c.GoogleClient.ConvertToModel(m)

	// Google AI Studio specific customizations can be added here
	// For now, we use the base Google implementation as-is

	return model
}
