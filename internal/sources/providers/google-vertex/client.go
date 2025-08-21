package googlevertex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/agentstation/starmap/internal/sources/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/utc"
	"google.golang.org/genai"
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

	// Create GenAI client configured for Vertex AI
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  c.projectID,
		Location: c.location,
		// Use Application Default Credentials automatically
	})
	if err != nil {
		return nil, fmt.Errorf("creating genai client: %w", err)
	}

	var models []catalogs.Model

	// Get all base models with pagination
	baseModels, err := c.getAllModels(ctx, client, true)
	if err != nil {
		fmt.Printf("Note: Could not list base models: %v\n", err)
	} else {
		models = append(models, baseModels...)
	}

	// Get all tuned/custom models with pagination
	tunedModels, err := c.getAllModels(ctx, client, false)
	if err != nil {
		fmt.Printf("Note: Could not list tuned models: %v\n", err)
	} else {
		models = append(models, tunedModels...)
	}

	// Add models from REST API that might not be returned by GenAI SDK
	// This includes Model Garden (MaaS) models and publisher models
	restModels, err := c.getModelsFromRESTAPI(ctx)
	if err != nil {
		fmt.Printf("Note: Could not fetch additional models from REST API: %v\n", err)
	} else {
		// Merge with existing models, avoiding duplicates
		models = c.mergeModels(models, restModels)
	}

	return models, nil
}

// RestAPIModel represents a model from the Vertex AI REST API
type RestAPIModel struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	// Add other fields as needed
}

// RestAPIResponse represents the response from the Vertex AI REST API
type RestAPIResponse struct {
	Models           []RestAPIModel         `json:"models"`
	PublisherModels  []PublisherModel      `json:"publisherModels"`
	NextPageToken    string                `json:"nextPageToken"`
}

// PublisherModel represents a publisher model from the Vertex AI REST API
type PublisherModel struct {
	Name               string `json:"name"`
	VersionID          string `json:"versionId"`
	OpenSourceCategory string `json:"openSourceCategory"`
	LaunchStage        string `json:"launchStage"`
}

// getModelsFromRESTAPI fetches models using the Vertex AI REST API
// This can retrieve models not available through the GenAI SDK, like Model Garden models
func (c *Client) getModelsFromRESTAPI(ctx context.Context) ([]catalogs.Model, error) {
	if c.projectID == "" || c.location == "" {
		return nil, fmt.Errorf("project ID or location not configured")
	}

	// Get access token for authentication
	accessToken, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	var allModels []catalogs.Model

	// Fetch publisher models (Model Garden models)
	publisherModels, err := c.fetchPublisherModels(ctx, accessToken)
	if err == nil {
		allModels = append(allModels, publisherModels...)
	}

	// Fetch regular models as well (in case GenAI SDK missed some)
	regularModels, err := c.fetchRegularModels(ctx, accessToken)
	if err == nil {
		allModels = append(allModels, regularModels...)
	}

	return allModels, nil
}

// fetchPublisherModels fetches models from publishers (like Anthropic) via REST API
func (c *Client) fetchPublisherModels(ctx context.Context, accessToken string) ([]catalogs.Model, error) {
	var allModels []catalogs.Model

	// Fetch models from different publishers using the correct v1beta1 endpoint
	publishers := []string{"google", "anthropic", "meta"}
	
	for _, publisher := range publishers {
		url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1beta1/publishers/%s/models", c.location, publisher)
		publisherModels, err := c.fetchModelsFromURL(ctx, url, accessToken)
		if err == nil {
			allModels = append(allModels, publisherModels...)
		}
	}

	return allModels, nil
}

// fetchRegularModels fetches regular models via REST API
func (c *Client) fetchRegularModels(ctx context.Context, accessToken string) ([]catalogs.Model, error) {
	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/models",
		c.location, c.projectID, c.location)

	return c.fetchModelsFromURL(ctx, url, accessToken)
}

// fetchModelsFromURL fetches models from a specific REST API URL
func (c *Client) fetchModelsFromURL(ctx context.Context, url, accessToken string) ([]catalogs.Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-user-project", c.projectID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("REST API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp RestAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	var models []catalogs.Model
	
	// Handle regular models
	for _, restModel := range apiResp.Models {
		model := c.convertRestModelToStarmap(restModel)
		models = append(models, model)
	}
	
	// Handle publisher models (Model Garden models)
	for _, publisherModel := range apiResp.PublisherModels {
		model := c.convertPublisherModelToStarmap(publisherModel)
		models = append(models, model)
	}

	return models, nil
}

// convertRestModelToStarmap converts a REST API model to a starmap model
func (c *Client) convertRestModelToStarmap(restModel RestAPIModel) catalogs.Model {
	// Extract model ID from the full name (e.g., "publishers/anthropic/models/claude-opus-4-1")
	modelID := c.ExtractModelID(restModel.Name)

	model := catalogs.Model{
		ID:          modelID,
		Name:        restModel.DisplayName,
		Description: restModel.Description,
		CreatedAt:   utc.Now(),
		UpdatedAt:   utc.Now(),
	}

	// Set default features for models found via REST API
	model.Features = &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
		Temperature: true,
		TopP:        true,
		MaxTokens:   true,
		Streaming:   true,
	}

	// Enhanced feature detection for specific model types
	modelIDLower := strings.ToLower(modelID)
	if strings.Contains(modelIDLower, "claude") {
		// Claude models typically support multimodal and advanced features
		model.Features.Modalities.Input = append(model.Features.Modalities.Input, catalogs.ModelModalityImage)
		model.Features.ToolCalls = true
		model.Features.Tools = true
		model.Features.ToolChoice = true
		model.Features.Reasoning = true

		// Set typical Claude limits
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: 200000,
			OutputTokens:  4096,
		}
	}

	return model
}

// convertPublisherModelToStarmap converts a publisher model to a starmap model
func (c *Client) convertPublisherModelToStarmap(publisherModel PublisherModel) catalogs.Model {
	// Extract model ID from the full name (e.g., "publishers/anthropic/models/claude-opus-4-1")
	modelID := c.ExtractModelID(publisherModel.Name)
	
	// Add version to model ID if available
	if publisherModel.VersionID != "" {
		modelID = fmt.Sprintf("%s@%s", modelID, publisherModel.VersionID)
	}

	model := catalogs.Model{
		ID:        modelID,
		Name:      c.generateModelName(modelID),
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}

	// Set features based on model type
	model.Features = &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
		Temperature: true,
		TopP:        true,
		MaxTokens:   true,
		Streaming:   true,
	}

	// Enhanced feature detection for specific model types
	modelIDLower := strings.ToLower(modelID)
	if strings.Contains(modelIDLower, "claude") {
		// Claude models support multimodal and advanced features
		model.Features.Modalities.Input = append(model.Features.Modalities.Input, catalogs.ModelModalityImage)
		model.Features.ToolCalls = true
		model.Features.Tools = true
		model.Features.ToolChoice = true
		model.Features.Reasoning = true

		// Set typical Claude limits
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: 200000,
			OutputTokens:  4096,
		}
		
		// Set metadata for open source category
		if publisherModel.OpenSourceCategory != "" {
			model.Metadata = &catalogs.ModelMetadata{
				OpenWeights: publisherModel.OpenSourceCategory == "OPEN_SOURCE",
			}
		}
	}

	return model
}

// generateModelName creates a human-readable model name from model ID
func (c *Client) generateModelName(modelID string) string {
	// Remove version suffix for name generation
	baseID := modelID
	if idx := strings.Index(modelID, "@"); idx != -1 {
		baseID = modelID[:idx]
	}
	
	// Convert to title case and clean up
	name := strings.ReplaceAll(baseID, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	
	// Convert to title case
	words := strings.Fields(strings.ToLower(name))
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	
	return strings.Join(words, " ")
}

// getAccessToken gets a Google Cloud access token for API authentication
func (c *Client) getAccessToken() (string, error) {
	cmd := exec.Command("gcloud", "auth", "print-access-token")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting access token via gcloud: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// mergeModels merges two slices of models, avoiding duplicates by ID
func (c *Client) mergeModels(existing, new []catalogs.Model) []catalogs.Model {
	existingIDs := make(map[string]bool)
	for _, model := range existing {
		existingIDs[model.ID] = true
	}

	var merged []catalogs.Model
	merged = append(merged, existing...)

	for _, model := range new {
		if !existingIDs[model.ID] {
			merged = append(merged, model)
		}
	}

	return merged
}

// getAllModels fetches all models with pagination support
func (c *Client) getAllModels(ctx context.Context, client *genai.Client, queryBase bool) ([]catalogs.Model, error) {
	var allModels []catalogs.Model
	pageToken := ""
	
	for {
		config := &genai.ListModelsConfig{
			QueryBase: genai.Ptr(queryBase),
			PageSize:  100, // Get more models per request
		}
		
		if pageToken != "" {
			config.PageToken = pageToken
		}
		
		response, err := client.Models.List(ctx, config)
		if err != nil {
			return nil, err
		}
		
		// Process models in this page
		for _, model := range response.Items {
			// Try to get detailed model information
			detailedModel, err := c.getDetailedModel(ctx, client, model.Name)
			if err != nil {
				// Use basic model data as fallback
				starmapModel := c.convertGenAIModelToStarmap(model)
				allModels = append(allModels, starmapModel)
			} else {
				starmapModel := c.convertGenAIModelToStarmap(detailedModel)
				allModels = append(allModels, starmapModel)
			}
		}
		
		// Check if there are more pages
		if response.NextPageToken == "" {
			break
		}
		pageToken = response.NextPageToken
	}
	
	return allModels, nil
}

// getDetailedModel fetches detailed information for a specific model
func (c *Client) getDetailedModel(ctx context.Context, client *genai.Client, modelName string) (*genai.Model, error) {
	// Use the Models.Get() method to fetch detailed model information
	config := &genai.GetModelConfig{}
	return client.Models.Get(ctx, modelName, config)
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

// convertGenAIModelToStarmap converts a GenAI model to a starmap model.
func (c *Client) convertGenAIModelToStarmap(genaiModel *genai.Model) catalogs.Model {
	// Extract model ID from full name
	modelID := c.ExtractModelID(genaiModel.Name)

	// Debug: Compare GenAI vs REST API for specific model
	if modelID == "gemini-1.5-flash-002" {
		fmt.Printf("=== GENAI SDK vs REST API COMPARISON ===\n")
		fmt.Printf("GenAI SDK Model Fields:\n")
		fmt.Printf("  Name: %q\n", genaiModel.Name)
		fmt.Printf("  DisplayName: %q\n", genaiModel.DisplayName)
		fmt.Printf("  Description: %q\n", genaiModel.Description)
		fmt.Printf("  Version: %q\n", genaiModel.Version)
		fmt.Printf("  InputTokenLimit: %d\n", genaiModel.InputTokenLimit)
		fmt.Printf("  OutputTokenLimit: %d\n", genaiModel.OutputTokenLimit)
		fmt.Printf("  SupportedActions: %v\n", genaiModel.SupportedActions)
		fmt.Printf("  Labels: %v\n", genaiModel.Labels)
		fmt.Printf("============================================\n")
	}



	// Create basic starmap model with fallbacks for empty fields
	displayName := genaiModel.DisplayName
	if displayName == "" {
		displayName = modelID // Fallback to ID if no display name
	}
	
	description := genaiModel.Description
	if description == "" {
		description = fmt.Sprintf("Google Vertex AI model: %s", modelID)
	}
	
	model := catalogs.Model{
		ID:          modelID,
		Name:        displayName,
		Description: description,
		CreatedAt:   utc.Now(), // GenAI models don't have creation timestamps
		UpdatedAt:   utc.Now(),
	}

	// Initialize features based on model capabilities
	model.Features = &catalogs.ModelFeatures{
		// Default modalities - all models support text
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
	}

	// Map supported actions to features
	for _, action := range genaiModel.SupportedActions {
		switch action {
		case "generateContent":
			model.Features.Temperature = true
			model.Features.TopP = true
			model.Features.MaxTokens = true
			model.Features.Streaming = true
		case "countTokens":
			// Token counting capability
		case "embedContent":
			// Embedding capability - different from chat
		}
	}

	// Enhanced feature detection based on model ID and capabilities
	modelIDLower := strings.ToLower(modelID)
	if strings.Contains(modelIDLower, "gemini") {
		// Gemini models typically support multimodal input
		if !strings.Contains(modelIDLower, "embedding") {
			model.Features.Modalities.Input = append(model.Features.Modalities.Input, catalogs.ModelModalityImage)
			model.Features.ToolCalls = true
			model.Features.Tools = true
			model.Features.ToolChoice = true
		}
	}

	// Set limits if available
	if genaiModel.InputTokenLimit > 0 || genaiModel.OutputTokenLimit > 0 {
		model.Limits = &catalogs.ModelLimits{}
		
		// Set context window to input limit if available
		if genaiModel.InputTokenLimit > 0 {
			model.Limits.ContextWindow = int64(genaiModel.InputTokenLimit)
		}
		
		// Set output tokens limit if available
		if genaiModel.OutputTokenLimit > 0 {
			model.Limits.OutputTokens = int64(genaiModel.OutputTokenLimit)
		}
	}

	// Set metadata if available
	if model.Metadata == nil {
		model.Metadata = &catalogs.ModelMetadata{}
	}

	// Try to extract version information
	if genaiModel.Version != "" {
		model.Metadata.ReleaseDate = utc.Now() // Use current time as placeholder
	}

	return model
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