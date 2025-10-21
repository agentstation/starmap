// Package google provides a unified, dynamic client for Google AI APIs (AI Studio and Vertex AI).
// This package provides configuration-driven behavior based on provider YAML configuration.
package google

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"github.com/agentstation/utc"
	"google.golang.org/genai"

	"github.com/agentstation/starmap/internal/auth/adc"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Client implements the catalogs.Client interface with dynamic configuration
// for both Google AI Studio and Vertex AI.
type Client struct {
	provider *catalogs.Provider

	// Authentication
	credentials *auth.Credentials // Centralized credentials management

	// Vertex AI specific fields (lazy-loaded)
	projectID string
	location  string

	// GenAI client - reused across calls when possible
	genaiClient *genai.Client

	mu sync.RWMutex
}

// NewClient creates a new dynamic Google client that works for both AI Studio and Vertex AI.
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{
		provider: provider,
	}
}

// Configure sets the provider for this client.
func (c *Client) Configure(provider *catalogs.Provider) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.provider = provider

	// Reset cached clients and credentials
	c.genaiClient = nil
	c.credentials = nil
	c.projectID = ""
	c.location = ""
}

// Close releases any resources held by the client.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.genaiClient != nil {
		// GenAI client doesn't have a Close method, but we clear the reference
		c.genaiClient = nil
	}

	// Clear credentials to force re-initialization if needed
	c.credentials = nil

	return nil
}

// IsAPIKeyRequired returns true if the client requires an API key.
func (c *Client) IsAPIKeyRequired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider.IsAPIKeyRequired()
}

// HasAPIKey returns true if the client has an API key.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider.HasAPIKey()
}

// initCredentials initializes or returns cached credentials for Google Cloud authentication.
func (c *Client) initCredentials(ctx context.Context) (*auth.Credentials, error) {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.credentials != nil {
		return c.credentials, nil
	}

	// Detect credentials with aggressive timeout (2 seconds max)
	// DetectDefault doesn't accept context, so we run it in a goroutine
	type result struct {
		creds *auth.Credentials
		err   error
	}

	resultChan := make(chan result, 1)
	go func() {
		creds, err := credentials.DetectDefault(&credentials.DetectOptions{
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/generative-language",
			},
		})
		resultChan <- result{creds: creds, err: err}
	}()

	// Wait for result or timeout (2 seconds - realistic time is under 100ms)
	timeout := time.After(2 * time.Second)
	select {
	case res := <-resultChan:
		if res.err != nil {
			return nil, &errors.ConfigError{
				Component: string(c.provider.ID),
				Message:   "no valid credentials found - configure Application Default Credentials or set GOOGLE_CLOUD_PROJECT",
			}
		}
		c.credentials = res.creds
		return res.creds, nil

	case <-timeout:
		return nil, &errors.ConfigError{
			Component: string(c.provider.ID),
			Message:   "credential detection timed out (2s) - likely not configured or network issue",
		}

	case <-ctx.Done():
		return nil, &errors.ConfigError{
			Component: string(c.provider.ID),
			Message:   "credential detection cancelled",
		}
	}

}

// ListModels retrieves all available models using the appropriate Google API.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return nil, &errors.ValidationError{
			Field:   "provider",
			Message: "provider not configured",
		}
	}

	// Determine which backend to use based on provider configuration
	useVertex := c.shouldUseVertexBackend()

	if useVertex {
		return c.listModelsVertex(ctx)
	}

	// Check if AI Studio is configured
	if !provider.HasAPIKey() {
		return nil, &errors.ConfigError{
			Component: string(provider.ID),
			Message:   "no valid configuration found - set GOOGLE_API_KEY for AI Studio or GOOGLE_CLOUD_PROJECT for Vertex AI",
		}
	}

	return c.listModelsAIStudio(ctx)
}

// shouldUseVertexBackend determines if we should use Vertex AI backend.
func (c *Client) shouldUseVertexBackend() bool {
	// Check endpoint type first
	if c.provider.Catalog != nil && c.provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		return true
	}

	// Check for Vertex-specific configuration
	if c.provider.EnvVar("GOOGLE_VERTEX_PROJECT") != "" ||
		c.provider.EnvVar("GOOGLE_CLOUD_PROJECT") != "" {
		return true
	}

	// If we have an API key, prefer AI Studio
	if c.provider.HasAPIKey() {
		return false
	}

	// Don't default to Vertex - it requires explicit configuration
	// Without project ID or API key, we can't use either backend
	return false
}

// getOrCreateGenAIClient gets or creates a GenAI client for the appropriate backend.
func (c *Client) getOrCreateGenAIClient(ctx context.Context, forVertex bool) (*genai.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Return existing client if available and matches backend
	if c.genaiClient != nil {
		return c.genaiClient, nil
	}

	var config *genai.ClientConfig

	if forVertex {
		// Ensure we have project and location
		if c.projectID == "" {
			c.projectID = c.getProjectID(ctx)
		}
		if c.location == "" {
			c.location = c.getLocation(ctx)
		}

		if c.projectID == "" {
			return nil, &errors.ConfigError{
				Component: "google-vertex",
				Message:   "project ID not configured - set GOOGLE_CLOUD_PROJECT or configure ADC with project",
			}
		}

		config = &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  c.projectID,
			Location: c.location,
		}

		// Check if API key is available for Vertex AI (optional)
		if apiKey, err := c.provider.APIKeyValue(); err == nil && apiKey != "" {
			// Use API key for Vertex AI if available
			config.APIKey = apiKey
		} else {
			// Fall back to Application Default Credentials
			creds, err := c.initCredentials(ctx)
			if err != nil {
				return nil, err
			}
			config.Credentials = creds
		}
	} else {
		// AI Studio configuration with API key
		apiKey, err := c.provider.APIKeyValue()
		if err != nil || apiKey == "" {
			return nil, &errors.AuthenticationError{
				Provider: "google-ai-studio",
				Method:   "api-key",
				Message:  "API key required for Google AI Studio",
				Err:      err,
			}
		}

		config = &genai.ClientConfig{
			Backend: genai.BackendGeminiAPI,
			APIKey:  apiKey,
		}
	}

	client, err := genai.NewClient(ctx, config)
	if err != nil {
		return nil, err
	}

	c.genaiClient = client
	return client, nil
}

// listModelsAIStudio fetches models using Google AI Studio API via GenAI SDK.
func (c *Client) listModelsAIStudio(ctx context.Context) ([]catalogs.Model, error) {
	// Use GenAI SDK only
	client, err := c.getOrCreateGenAIClient(ctx, false)
	if err != nil {
		return nil, err
	}

	return c.listModelsViaGenAI(ctx, client)
}

// checkVertexPrerequisites performs pre-flight checks for Vertex AI.
// This uses the same logic as `starmap providers auth test` to detect ADC configuration
// locally without making network calls.
func (c *Client) checkVertexPrerequisites() error {
	// Check ADC status using the same logic as `starmap providers auth test`
	details := adc.BuildDetails()

	switch details.State {
	case adc.StateMissing:
		return &errors.ConfigError{
			Component: "google-vertex",
			Message:   "Application Default Credentials not configured - run 'gcloud auth application-default login'",
		}
	case adc.StateInvalid:
		return &errors.ConfigError{
			Component: "google-vertex",
			Message:   "Application Default Credentials invalid - check 'gcloud auth application-default login'",
		}
	case adc.StateConfigured:
		// ADC is configured, now check if project is set
		if os.Getenv("GOOGLE_VERTEX_PROJECT") == "" && os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
			return &errors.ConfigError{
				Component: "google-vertex",
				Message:   "No project configured - set GOOGLE_VERTEX_PROJECT or GOOGLE_CLOUD_PROJECT environment variable",
			}
		}
		// All checks passed
		return nil
	default:
		return &errors.ConfigError{
			Component: "google-vertex",
			Message:   "Unknown ADC state",
		}
	}
}

// listModelsVertex fetches models using Vertex AI API.
func (c *Client) listModelsVertex(ctx context.Context) ([]catalogs.Model, error) {
	// Pre-flight check: Verify ADC is available before attempting network calls
	// This is the same check used by `starmap providers auth test`
	if err := c.checkVertexPrerequisites(); err != nil {
		return nil, err
	}

	// Create a strict timeout for Vertex operations (5 seconds)
	// Realistic response time is under 1 second; 5 seconds is generous
	vertexCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Detect project/location
	if c.projectID == "" {
		c.projectID = c.getProjectID(vertexCtx)
	}
	if c.location == "" {
		c.location = c.getLocation(vertexCtx)
	}

	if c.projectID == "" {
		return nil, &errors.ConfigError{
			Component: "google-vertex",
			Message:   "project ID not configured - set GOOGLE_CLOUD_PROJECT env var or run 'gcloud auth application-default set-quota-project YOUR_PROJECT'",
		}
	}

	// Use GenAI SDK only
	client, err := c.getOrCreateGenAIClient(vertexCtx, true)
	if err != nil {
		return nil, err
	}

	// Get models from GenAI SDK with timeout protection
	type result struct {
		models []catalogs.Model
		err    error
	}
	resultChan := make(chan result, 1)

	go func() {
		models, err := c.listModelsViaGenAI(vertexCtx, client)
		resultChan <- result{models: models, err: err}
	}()

	// Wait for result or timeout
	select {
	case res := <-resultChan:
		if res.err != nil {
			return nil, res.err
		}

		// Add Model Garden models from pre-defined list
		modelGardenModels := c.getModelGardenModels()
		models := c.mergeModels(res.models, modelGardenModels)
		return models, nil

	case <-vertexCtx.Done():
		return nil, &errors.APIError{
			Provider:   "google-vertex",
			Endpoint:   "models",
			StatusCode: 0,
			Message:    "request timed out after 5 seconds - vertex AI may not be properly configured or network is slow",
			Err:        vertexCtx.Err(),
		}
	}
}

// listModelsViaGenAI uses the GenAI SDK to list models (works for both backends).
func (c *Client) listModelsViaGenAI(ctx context.Context, client *genai.Client) ([]catalogs.Model, error) {
	var models []catalogs.Model

	// Get all base models with pagination
	baseModels, err := c.getAllModelsGenAI(ctx, client, true)
	if err != nil {
		fmt.Printf("Note: Could not list base models: %v\n", err)
	} else {
		for _, model := range baseModels {
			models = append(models, *model)
		}
	}

	// Get all tuned/custom models with pagination
	tunedModels, err := c.getAllModelsGenAI(ctx, client, false)
	if err != nil {
		fmt.Printf("Note: Could not list tuned models: %v\n", err)
	} else {
		for _, model := range tunedModels {
			models = append(models, *model)
		}
	}

	if len(models) == 0 && err != nil {
		return nil, err // Return error if we got no models at all
	}

	return models, nil
}

// extractModelID extracts the model ID from the full name.
func (c *Client) extractModelID(name string) string {
	// Handle different formats:
	// - AI Studio: models/gemini-pro
	// - Vertex: projects/PROJECT/locations/LOCATION/models/MODEL_ID
	// - Publisher: publishers/anthropic/models/claude-opus-4-1

	if strings.Contains(name, "/models/") {
		parts := strings.Split(name, "/models/")
		if len(parts) > 1 {
			return parts[1]
		}
	}

	// Fallback to last segment
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// inferFeatures infers model features based on the model ID and supported methods.
func (c *Client) inferFeatures(modelID string, supportedMethods []string) *catalogs.ModelFeatures {
	features := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
		Temperature: true,
		TopP:        true,
		MaxTokens:   true,
		Stop:        true,
		Streaming:   true,
	}

	// Apply provider-specific feature rules if configured
	if c.provider.Catalog != nil && c.provider.Catalog.Endpoint.FeatureRules != nil {
		for _, rule := range c.provider.Catalog.Endpoint.FeatureRules {
			c.applyFeatureRule(features, modelID, rule)
		}
		return features
	}

	// Default feature detection
	modelLower := strings.ToLower(modelID)

	// Gemini models
	if strings.Contains(modelLower, "gemini") {
		features.Tools = true
		features.ToolChoice = true
		features.ToolCalls = true
		features.StructuredOutputs = true
		features.FormatResponse = true

		if strings.Contains(modelLower, "vision") ||
			strings.Contains(modelLower, "gemini-1.5") ||
			strings.Contains(modelLower, "gemini-2") {
			features.Modalities.Input = append(features.Modalities.Input, catalogs.ModelModalityImage)
		}
	}

	// Claude models (via Vertex)
	if strings.Contains(modelLower, "claude") {
		features.Modalities.Input = append(features.Modalities.Input, catalogs.ModelModalityImage)
		features.ToolCalls = true
		features.Tools = true
		features.ToolChoice = true
		features.Reasoning = true
	}

	// Llama models
	if strings.Contains(modelLower, "llama") {
		features.ToolCalls = true
		features.Tools = true
		features.Reasoning = true
	}

	// Mistral models
	if strings.Contains(modelLower, "mistral") {
		features.ToolCalls = true
		features.Tools = true
	}

	// Check supported generation methods
	for _, method := range supportedMethods {
		switch strings.ToLower(method) {
		case "generatecontent":
			// Standard generation
		case "streamgeneratecontent":
			features.Streaming = true
		case "counttokens":
			// Token counting capability
		case "embedcontent":
			// Embedding models have different output
			features.Modalities.Output = []catalogs.ModelModality{}
		}
	}

	return features
}

// applyFeatureRule applies a configured feature rule.
func (c *Client) applyFeatureRule(features *catalogs.ModelFeatures, modelID string, rule catalogs.FeatureRule) {
	fieldValue := modelID
	if rule.Field != "id" {
		return
	}

	fieldLower := strings.ToLower(fieldValue)
	matches := false
	for _, contains := range rule.Contains {
		if strings.Contains(fieldLower, strings.ToLower(contains)) {
			matches = true
			break
		}
	}

	if !matches {
		return
	}

	switch rule.Feature {
	case "tools":
		features.Tools = rule.Value
	case "tool_choice":
		features.ToolChoice = rule.Value
	case "tool_calls":
		features.ToolCalls = rule.Value
	case "structured_outputs":
		features.StructuredOutputs = rule.Value
	case "reasoning":
		features.Reasoning = rule.Value
	case "top_k":
		features.TopK = rule.Value
	case "format_response":
		features.FormatResponse = rule.Value
	}
}

// getAllModelsGenAI fetches all models with pagination support using GenAI SDK.
func (c *Client) getAllModelsGenAI(ctx context.Context, client *genai.Client, queryBase bool) ([]*catalogs.Model, error) {
	var allModels []*catalogs.Model
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
				starmapModel := c.convertGenAIModel(model)
				allModels = append(allModels, starmapModel)
			} else {
				starmapModel := c.convertGenAIModel(detailedModel)
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

// getDetailedModel fetches detailed information for a specific model.
func (c *Client) getDetailedModel(ctx context.Context, client *genai.Client, modelName string) (*genai.Model, error) {
	config := &genai.GetModelConfig{}
	return client.Models.Get(ctx, modelName, config)
}

// convertGenAIModel converts a GenAI model to a starmap model.
func (c *Client) convertGenAIModel(genaiModel *genai.Model) *catalogs.Model {
	modelID := c.extractModelID(genaiModel.Name)

	displayName := genaiModel.DisplayName
	if displayName == "" {
		displayName = modelID
	}

	description := genaiModel.Description
	if description == "" {
		description = fmt.Sprintf("Google model: %s", modelID)
	}

	model := &catalogs.Model{
		ID:          modelID,
		Name:        displayName,
		Description: description,
		CreatedAt:   utc.Now(),
		UpdatedAt:   utc.Now(),
	}

	// Extract author from publisher info
	if strings.Contains(genaiModel.Name, "/publishers/") {
		parts := strings.Split(genaiModel.Name, "/publishers/")
		if len(parts) > 1 {
			publisherParts := strings.Split(parts[1], "/")
			if len(publisherParts) > 0 {
				authorID := c.normalizePublisherToAuthorID(publisherParts[0])
				model.Authors = []catalogs.Author{
					{ID: authorID, Name: string(authorID)},
				}
			}
		}
	} else if strings.Contains(strings.ToLower(modelID), "jamba") {
		// Special case for Jamba models
		model.Authors = []catalogs.Author{
			{ID: catalogs.AuthorIDAI21, Name: string(catalogs.AuthorIDAI21)},
		}
	} else {
		model.Authors = []catalogs.Author{
			{ID: catalogs.AuthorIDGoogle, Name: "Google"},
		}
	}

	// Initialize features based on model capabilities
	model.Features = &catalogs.ModelFeatures{
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
			model.Features.Modalities.Output = []catalogs.ModelModality{}
		}
	}

	// Enhanced feature detection
	modelIDLower := strings.ToLower(modelID)
	if strings.Contains(modelIDLower, "gemini") {
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

		if genaiModel.InputTokenLimit > 0 {
			model.Limits.ContextWindow = int64(genaiModel.InputTokenLimit)
		}

		if genaiModel.OutputTokenLimit > 0 {
			model.Limits.OutputTokens = int64(genaiModel.OutputTokenLimit)
		}
	}

	// Metadata.ReleaseDate will be provided by models.dev during reconciliation
	// (models.dev is authoritative for metadata per authority hierarchy)

	return model
}

// getModelGardenModels returns pre-defined Model Garden models based on configured authors.
func (c *Client) getModelGardenModels() []*catalogs.Model {
	var models []*catalogs.Model

	// Only include Model Garden models if authors are configured
	authors := c.provider.Catalog.Authors
	if len(authors) == 0 {
		return models
	}

	// Pre-defined Model Garden models for common publishers
	for _, author := range authors {
		switch author {
		case catalogs.AuthorIDAnthropic:
			// Anthropic Claude models
			models = append(models, c.createModelGardenModel("claude-3-5-sonnet@20241022", "Claude 3.5 Sonnet", author))
			models = append(models, c.createModelGardenModel("claude-3-5-haiku@20241022", "Claude 3.5 Haiku", author))
			models = append(models, c.createModelGardenModel("claude-3-opus@20240229", "Claude 3 Opus", author))

		case catalogs.AuthorIDMeta:
			// Meta Llama models
			models = append(models, c.createModelGardenModel("llama-3-2-90b-vision-instruct-maas", "Llama 3.2 90B Vision Instruct", author))
			models = append(models, c.createModelGardenModel("llama-3-1-405b-instruct-maas", "Llama 3.1 405B Instruct", author))
			models = append(models, c.createModelGardenModel("llama-3-1-70b-instruct-maas", "Llama 3.1 70B Instruct", author))

		case catalogs.AuthorIDMistralAI:
			// Mistral models
			models = append(models, c.createModelGardenModel("mistral-large@2407", "Mistral Large", author))
			models = append(models, c.createModelGardenModel("mistral-nemo@2407", "Mistral Nemo", author))

		case catalogs.AuthorIDAI21:
			// AI21 Jamba models
			models = append(models, c.createModelGardenModel("jamba-1-5-large@001", "Jamba 1.5 Large", author))
			models = append(models, c.createModelGardenModel("jamba-1-5-mini@001", "Jamba 1.5 Mini", author))

		case "deepseek-ai":
			// DeepSeek models
			models = append(models, c.createModelGardenModel("deepseek-r1-distill-qwen-32b@001", "DeepSeek R1 Distill Qwen 32B", catalogs.AuthorIDDeepSeek))
			models = append(models, c.createModelGardenModel("deepseek-r1-distill-llama-70b@001", "DeepSeek R1 Distill Llama 70B", catalogs.AuthorIDDeepSeek))

		case catalogs.AuthorIDQwen:
			// Qwen models
			models = append(models, c.createModelGardenModel("qwen2-5-coder-32b-instruct@001", "Qwen 2.5 Coder 32B Instruct", author))

		case catalogs.AuthorIDOpenAI:
			// OpenAI models via Vertex
			models = append(models, c.createModelGardenModel("gpt-4o-2024-08-06@001", "GPT-4o", author))
		}
	}

	return models
}

// createModelGardenModel creates a standardized Model Garden model.
func (c *Client) createModelGardenModel(modelID, displayName string, authorID catalogs.AuthorID) *catalogs.Model {
	model := &catalogs.Model{
		ID:          modelID,
		Name:        displayName,
		Description: fmt.Sprintf("%s model available through Vertex AI Model Garden", displayName),
		Authors:     []catalogs.Author{{ID: authorID, Name: string(authorID)}},
		CreatedAt:   utc.Now(),
		UpdatedAt:   utc.Now(),
	}

	// Set features based on model ID
	model.Features = c.inferFeatures(modelID, nil)

	// Set limits based on author/model type
	modelLower := strings.ToLower(modelID)
	switch authorID {
	case catalogs.AuthorIDAnthropic:
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: 200000,
			OutputTokens:  4096,
		}
	case catalogs.AuthorIDMeta:
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: 128000,
			OutputTokens:  4096,
		}
	case catalogs.AuthorIDMistralAI:
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: 128000,
			OutputTokens:  4096,
		}
	case catalogs.AuthorIDAI21:
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: 256000,
			OutputTokens:  4096,
		}
	case catalogs.AuthorIDDeepSeek:
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: 64000,
			OutputTokens:  4096,
		}
	case catalogs.AuthorIDQwen:
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: 32000,
			OutputTokens:  4096,
		}
	case catalogs.AuthorIDOpenAI:
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: 128000,
			OutputTokens:  4096,
		}
	}

	// Metadata will be provided by models.dev during reconciliation
	// (models.dev is authoritative for metadata per authority hierarchy)

	// Special handling for vision models
	if strings.Contains(modelLower, "vision") {
		model.Features.Modalities.Input = append(model.Features.Modalities.Input, catalogs.ModelModalityImage)
	}

	return model
}

// mergeModels merges existing models with additional models, avoiding duplicates.
func (c *Client) mergeModels(existing []catalogs.Model, additional []*catalogs.Model) []catalogs.Model {
	existingIDs := make(map[string]bool)
	for _, model := range existing {
		existingIDs[model.ID] = true
	}

	merged := append([]catalogs.Model{}, existing...)
	for _, model := range additional {
		if !existingIDs[model.ID] {
			merged = append(merged, *model)
		}
	}

	return merged
}

// getProjectID gets the project ID from environment variables or Application Default Credentials.
func (c *Client) getProjectID(ctx context.Context) string {
	// 1. Check environment variables first (highest priority)
	if projectID := c.provider.EnvVar("GOOGLE_CLOUD_PROJECT"); projectID != "" {
		return projectID
	}
	if projectID := c.provider.EnvVar("GOOGLE_VERTEX_PROJECT"); projectID != "" {
		return projectID
	}

	// 2. Get from credentials (no gcloud fallback)
	creds, err := c.initCredentials(ctx)
	if err == nil {
		// Try quota project ID first (for billing)
		if projectID, err := creds.QuotaProjectID(ctx); err == nil && projectID != "" {
			return projectID
		}

		// Fall back to regular project ID
		if projectID, err := creds.ProjectID(ctx); err == nil && projectID != "" {
			return projectID
		}
	}

	return ""
}

// getLocation gets the location from environment variables with sensible defaults.
// Returns empty string if context is cancelled.
func (c *Client) getLocation(ctx context.Context) string {
	// Check if context is already cancelled
	if ctx.Err() != nil {
		return ""
	}

	// Check environment variables
	locations := []string{
		c.provider.EnvVar("GOOGLE_CLOUD_LOCATION"),
		c.provider.EnvVar("GOOGLE_CLOUD_REGION"),
		c.provider.EnvVar("GOOGLE_VERTEX_LOCATION"),
	}

	for _, loc := range locations {
		if loc != "" {
			return loc
		}
	}

	// Default to us-central1 (most commonly available region)
	return "us-central1"
}

// ValidateCredentials validates that the client can authenticate properly.
func (c *Client) ValidateCredentials(ctx context.Context) error {
	if c.shouldUseVertexBackend() {
		// For Vertex, check that we can get credentials and project
		creds, err := c.initCredentials(ctx)
		if err != nil {
			return err
		}

		// Try to get a token to validate credentials work
		_, err = creds.Token(ctx)
		if err != nil {
			return &errors.AuthenticationError{
				Provider: string(c.provider.ID),
				Method:   "oauth2",
				Message:  "credentials validation failed",
				Err:      err,
			}
		}

		// Verify project ID is available
		projectID := c.getProjectID(ctx)
		if projectID == "" {
			return &errors.ConfigError{
				Component: "google-vertex",
				Message:   "no project ID available - set GOOGLE_CLOUD_PROJECT or configure ADC with project",
			}
		}
	} else {
		// For AI Studio, just check API key
		if !c.HasAPIKey() {
			return &errors.AuthenticationError{
				Provider: "google-ai-studio",
				Method:   "api-key",
				Message:  "API key not configured",
			}
		}
	}

	return nil
}

// normalizePublisherToAuthorID maps Google Vertex publisher names to AuthorID.
func (c *Client) normalizePublisherToAuthorID(publisher string) catalogs.AuthorID {
	switch strings.ToLower(publisher) {
	case "google":
		return catalogs.AuthorIDGoogle
	case "meta":
		return catalogs.AuthorIDMeta
	case "deepseek-ai":
		return catalogs.AuthorIDDeepSeek
	case "openai":
		return catalogs.AuthorIDOpenAI
	case "qwen":
		return catalogs.AuthorIDQwen
	case "ai21":
		return catalogs.AuthorIDAI21
	case "anthropic":
		return catalogs.AuthorIDAnthropic
	case "mistralai":
		return catalogs.AuthorIDMistralAI
	default:
		return catalogs.AuthorID(strings.ToLower(publisher))
	}
}
