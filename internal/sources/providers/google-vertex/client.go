package googlevertex

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/agentstation/starmap/internal/sources/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/utc"
)

func init() {
	// Register this provider client in the registry
	registry.RegisterClient(catalogs.ProviderIDGoogleVertex, &Client{})
}

// Client implements the catalogs.Client interface for Google Vertex AI.
type Client struct {
	provider  *catalogs.Provider
	projectID string
	location  string
}

// NewClient creates a new Google Vertex AI client (kept for backward compatibility).
func NewClient(apiKey string, provider *catalogs.Provider) *Client {
	client := &Client{provider: provider}
	client.Configure(provider)
	return client
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	c.provider = provider
	c.projectID = getProjectID(provider)
	c.location = getLocation(provider)
}

// ListModels retrieves all available models from Google Vertex AI.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	if c.projectID == "" {
		return nil, fmt.Errorf("project ID not configured - set GOOGLE_VERTEX_PROJECT or run 'gcloud config set project YOUR_PROJECT'")
	}

	// Get generative AI models (Gemini, Claude) - this is what Vertex AI primarily offers
	return c.listGenerativeModels(ctx)
}


// listGenerativeModels returns known generative AI models available on Vertex AI
func (c *Client) listGenerativeModels(ctx context.Context) ([]catalogs.Model, error) {
	var models []catalogs.Model

	// Known Gemini models available on Vertex AI
	geminiModels := []string{
		"gemini-2.0-flash-001",
		"gemini-2.0-flash-lite-001",
		"gemini-2.5-pro-001",
		"gemini-2.5-flash-001",
		"gemini-1.5-pro-001",
		"gemini-1.5-flash-001",
	}

	for _, modelID := range geminiModels {
		models = append(models, catalogs.Model{
			ID:          modelID,
			Name:        strings.Replace(modelID, "-", " ", -1),
			Description: fmt.Sprintf("Google Gemini model %s available through Vertex AI", modelID),
			CreatedAt:   utc.Now(),
			UpdatedAt:   utc.Now(),
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
				ToolCalls:   true,
				Tools:       true,
				ToolChoice:  true,
				Temperature: true,
				TopP:        true,
				MaxTokens:   true,
			},
		})
	}

	// Known Claude models available on Vertex AI Model Garden
	claudeModels := []string{
		"claude-3-5-sonnet-v2@20241022",
		"claude-3-5-haiku@20241022", 
		"claude-3-opus@20240229",
		"claude-3-sonnet@20240229",
		"claude-3-haiku@20240307",
	}

	for _, modelID := range claudeModels {
		cleanID := strings.Split(modelID, "@")[0] // Remove version suffix for display
		hasVision := strings.Contains(modelID, "sonnet") || strings.Contains(modelID, "opus")
		inputModalities := []catalogs.ModelModality{catalogs.ModelModalityText}
		if hasVision {
			inputModalities = append(inputModalities, catalogs.ModelModalityImage)
		}
		
		models = append(models, catalogs.Model{
			ID:          modelID,
			Name:        strings.Replace(cleanID, "-", " ", -1),
			Description: fmt.Sprintf("Anthropic Claude model %s available through Vertex AI Model Garden", cleanID),
			CreatedAt:   utc.Now(),
			UpdatedAt:   utc.Now(),
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  inputModalities,
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
				ToolCalls:   true,
				Tools:       true,
				ToolChoice:  true,
				Temperature: true,
				TopP:        true,
				MaxTokens:   true,
			},
		})
	}

	return models, nil
}

// GetModel retrieves a specific model by its ID.
func (c *Client) GetModel(ctx context.Context, modelID string) (*catalogs.Model, error) {
	// Try to find the model in our list
	models, err := c.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	for _, model := range models {
		if model.ID == modelID {
			return &model, nil
		}
	}

	return nil, fmt.Errorf("model %s not found", modelID)
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


// getProjectID gets the project ID from environment variable or gcloud config
func getProjectID(provider *catalogs.Provider) string {
	// Try environment variable first
	if projectID := provider.GetEnvVar("GOOGLE_VERTEX_PROJECT"); projectID != "" {
		return projectID
	}

	// Try gcloud config
	if projectID := getGcloudConfig("project"); projectID != "" {
		return projectID
	}

	// Return empty string - will cause error with helpful message
	return ""
}

// getLocation gets the location from environment variable or gcloud config
func getLocation(provider *catalogs.Provider) string {
	// Try environment variable first
	if location := provider.GetEnvVar("GOOGLE_VERTEX_LOCATION"); location != "" {
		return location
	}

	// Try gcloud config for region or zone
	if region := getGcloudConfig("compute/region"); region != "" {
		return region
	}

	if zone := getGcloudConfig("compute/zone"); zone != "" {
		// Extract region from zone (e.g., us-central1-a -> us-central1)
		if idx := strings.LastIndex(zone, "-"); idx > 0 {
			return zone[:idx]
		}
	}

	// Default fallback
	return "us-central1"
}

// getGcloudConfig gets a configuration value from gcloud
func getGcloudConfig(property string) string {
	cmd := exec.Command("gcloud", "config", "get-value", property)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	result := strings.TrimSpace(string(output))
	// gcloud returns "(unset)" when a property is not set
	if result == "(unset)" {
		return ""
	}

	return result
}