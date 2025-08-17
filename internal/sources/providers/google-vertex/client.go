package googlevertex

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentstation/starmap/internal/sources/base"
	"github.com/agentstation/starmap/internal/sources/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func init() {
	// Register this provider client in the registry
	registry.RegisterClient(catalogs.ProviderIDGoogleVertex, &Client{})
}

// Client implements the catalogs.Client interface for Google Vertex AI.
type Client struct {
	*base.GoogleClient
}

// NewClient creates a new Google Vertex AI client (kept for backward compatibility).
func NewClient(apiKey string, provider *catalogs.Provider) *Client {
	provider.APIKeyValue = apiKey // Set the API key in the provider
	// Default project and location for Google Vertex
	projectID := "your-project-id"
	location := "us-central1"
	baseURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/models", location, projectID, location)
	return &Client{
		GoogleClient: base.NewGoogleClient(provider, baseURL),
	}
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	// Default project and location for Google Vertex
	projectID := "your-project-id"
	location := "us-central1"
	baseURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/models", location, projectID, location)
	c.GoogleClient = base.NewGoogleClient(provider, baseURL)
}

// ListModels uses the base Google implementation.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	return c.GoogleClient.ListModels(ctx)
}

// GetModel uses the base Google implementation.
func (c *Client) GetModel(ctx context.Context, modelID string) (*catalogs.Model, error) {
	return c.GoogleClient.GetModel(ctx, modelID)
}

// ExtractModelID extracts the model ID from the full name for Google Vertex AI.
func (c *Client) ExtractModelID(name string) string {
	// Extract model ID from the name (format: projects/PROJECT/locations/LOCATION/models/MODEL_ID)
	if strings.Contains(name, "/models/") {
		parts := strings.Split(name, "/models/")
		if len(parts) > 1 {
			return parts[1]
		}
	}
	return name
}

// ConvertToModel overrides the base implementation for Google Vertex AI specific logic.
func (c *Client) ConvertToModel(m base.GoogleModelData) catalogs.Model {
	model := c.GoogleClient.ConvertToModel(m)

	// Google Vertex AI specific customizations can be added here
	// For now, we use the base Google implementation as-is

	return model
}
