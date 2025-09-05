// Package googleaistudio provides a client for interacting with the Google AI Studio API.
package googleaistudio

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/sources/providers/baseclient"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Client implements the catalogs.Client interface for Google AI Studio.
type Client struct {
	*baseclient.GoogleClient
}

// IsAPIKeyRequired returns true if the client requires an API key.
func (c *Client) IsAPIKeyRequired() bool {
	return c.GoogleClient.IsAPIKeyRequired()
}

// HasAPIKey returns true if the client has an API key.
func (c *Client) HasAPIKey() bool {
	return c.GoogleClient.HasAPIKey()
}

// NewClient creates a new Google AI Studio client (kept for backward compatibility).
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{
		GoogleClient: baseclient.NewGoogleClient(provider, "https://generativelanguage.googleapis.com/v1beta/models"),
	}
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	c.GoogleClient = baseclient.NewGoogleClient(provider, "https://generativelanguage.googleapis.com/v1beta/models")
}

// ListModels uses the base Google implementation.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	return c.GoogleClient.ListModels(ctx)
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
func (c *Client) ConvertToModel(m baseclient.GoogleModelData) catalogs.Model {
	model := c.GoogleClient.ConvertToModel(m)

	// Google AI Studio specific customizations can be added here
	// For now, we use the base Google implementation as-is

	return model
}
