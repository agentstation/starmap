// Package google provides a unified, dynamic client for Google AI APIs (AI Studio and Vertex AI).
// This package provides configuration-driven behavior based on provider YAML configuration.
package google

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"cloud.google.com/go/auth"
	"github.com/agentstation/utc"
	"google.golang.org/genai"

	"github.com/agentstation/starmap/internal/acquisition"
	starmapauth "github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

type aiStudioModelsResponse struct {
	Models        []aiStudioModel                  `json:"models"`
	NextPageToken string                           `json:"nextPageToken,omitempty"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
}

func (r *aiStudioModelsResponse) UnmarshalJSON(data []byte) error {
	type responseAlias aiStudioModelsResponse
	var decoded responseAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "$")
	if err != nil {
		return err
	}
	*r = aiStudioModelsResponse(decoded)
	r.UnknownFields = unknown
	return nil
}

type aiStudioModel struct {
	Name                       string                           `json:"name"`
	DisplayName                string                           `json:"displayName"`
	Description                string                           `json:"description"`
	Version                    string                           `json:"version,omitempty"`
	InputTokenLimit            int32                            `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           int32                            `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string                         `json:"supportedGenerationMethods,omitempty"`
	Temperature                *float64                         `json:"temperature,omitempty"`
	MaxTemperature             *float64                         `json:"maxTemperature,omitempty"`
	TopP                       *float64                         `json:"topP,omitempty"`
	TopK                       *int32                           `json:"topK,omitempty"`
	Thinking                   *bool                            `json:"thinking,omitempty"`
	UnknownFields              []sourcepayload.UnknownJSONField `json:"-"`
}

func (m *aiStudioModel) UnmarshalJSON(data []byte) error {
	type modelAlias aiStudioModel
	var decoded modelAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "models[]")
	if err != nil {
		return err
	}
	*m = aiStudioModel(decoded)
	m.UnknownFields = unknown
	return nil
}

// Client implements the catalogs.Client interface with dynamic configuration
// for both Google AI Studio and Vertex AI.
type Client struct {
	provider *catalogs.Provider
	source   catalogs.ProviderSource
	endpoint string
	auth     starmapauth.ResolvedAuth

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
func NewClient(source acquisition.Source) *Client {
	provider := source.Provider()
	projectID, _ := source.Binding("project")
	location, _ := source.Binding("location")
	return &Client{
		provider:  &provider,
		source:    source.Config(),
		endpoint:  source.EndpointURL(),
		auth:      source.Auth(),
		projectID: projectID,
		location:  location,
	}
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

	session, ok := c.auth.CloudSession().(interface{ Credentials() *auth.Credentials })
	if !ok || session.Credentials() == nil {
		return nil, &errors.AuthenticationError{Provider: string(c.provider.ID), Method: "cloud_chain", Message: "resolved Google cloud session is unavailable"}
	}
	c.credentials = session.Credentials()
	return c.credentials, nil
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
	if _, found := c.auth.APIKey(); !found {
		return nil, &errors.ConfigError{
			Component: string(provider.ID),
			Message:   "configured source authentication is unavailable",
		}
	}

	return c.listModelsAIStudio(ctx)
}

// shouldUseVertexBackend determines if we should use Vertex AI backend.
func (c *Client) shouldUseVertexBackend() bool {
	// Check endpoint type first
	if c.source.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		return true
	}
	return false
}

// getOrCreateGenAIClient gets or creates a GenAI client for the appropriate backend.
func (c *Client) getOrCreateGenAIClient(ctx context.Context, forVertex bool) (*genai.Client, error) {
	c.mu.RLock()
	if c.genaiClient != nil {
		client := c.genaiClient
		c.mu.RUnlock()
		return client, nil
	}
	projectID := c.projectID
	location := c.location
	c.mu.RUnlock()

	var config *genai.ClientConfig

	if forVertex {
		// Ensure we have project and location
		if projectID == "" {
			projectID = c.getProjectID(ctx)
		}
		if location == "" {
			location = c.getLocation(ctx)
		}

		if projectID == "" {
			return nil, &errors.ConfigError{
				Component: string(catalogs.ProviderIDGoogleVertex),
				Message:   "project binding is unavailable from configured inputs and the cloud profile",
			}
		}

		config = &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  projectID,
			Location: location,
		}

		// Check if API key is available for Vertex AI (optional)
		if apiKey, found := c.auth.APIKey(); found {
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
		apiKey, found := c.auth.APIKey()
		if !found {
			return nil, &errors.AuthenticationError{
				Provider: "google-ai-studio",
				Method:   "api-key",
				Message:  "API key required for Google AI Studio",
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

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.genaiClient != nil {
		return c.genaiClient, nil
	}
	c.projectID = projectID
	c.location = location
	c.genaiClient = client
	return client, nil
}

// listModelsAIStudio fetches models using Google AI Studio API via GenAI SDK.
func (c *Client) listModelsAIStudio(ctx context.Context) ([]catalogs.Model, error) {
	if models, err := c.listModelsAIStudioREST(ctx); err == nil {
		if len(models) > 0 {
			return models, nil
		}
	} else {
		var parseErr *errors.ParseError
		if stderrors.As(err, &parseErr) {
			return nil, err
		}
	}

	// Use GenAI SDK only
	client, err := c.getOrCreateGenAIClient(ctx, false)
	if err != nil {
		return nil, err
	}

	return c.listModelsViaGenAI(ctx, client)
}

func (c *Client) listModelsAIStudioREST(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	endpoint := c.endpoint
	resolvedAuth := c.auth
	c.mu.RUnlock()
	if provider == nil || endpoint == "" {
		return nil, &errors.ValidationError{
			Field:   "catalog.endpoint.url",
			Message: "Google AI Studio REST endpoint not configured",
		}
	}

	httpClient := transport.New(resolvedAuth)
	pageToken := ""
	models := make([]catalogs.Model, 0)
	for {
		requestURL, err := googleListURL(endpoint, pageToken)
		if err != nil {
			return nil, err
		}
		resp, err := httpClient.Get(ctx, requestURL)
		if err != nil {
			return nil, err
		}
		var result aiStudioModelsResponse
		if err := transport.DecodeResponse(resp, &result); err != nil {
			return nil, err
		}
		if result.Models == nil {
			return nil, errors.NewParseError("json", "google AI Studio response", "required models array is missing or null", nil)
		}
		for _, rawModel := range result.Models {
			rawModel.UnknownFields = append(rawModel.UnknownFields, result.UnknownFields...)
			models = append(models, *c.convertAIStudioModel(rawModel))
		}
		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}
	return models, nil
}

func googleListURL(endpoint, pageToken string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.WrapParse("url", endpoint, err)
	}
	query := parsed.Query()
	if query.Get("pageSize") == "" {
		query.Set("pageSize", "100")
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// listModelsVertex fetches models using Vertex AI API.
func (c *Client) listModelsVertex(ctx context.Context) ([]catalogs.Model, error) {
	// Bound the complete paginated operation while respecting any shorter caller
	// deadline. Vertex model listings can span multiple requests, so a per-call
	// latency assumption is not an appropriate operation deadline.
	vertexCtx, cancel := context.WithTimeout(ctx, constants.ProviderFetchTimeout)
	defer cancel()

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
			Provider:   string(catalogs.ProviderIDGoogleVertex),
			Endpoint:   "models",
			StatusCode: 0,
			Message:    fmt.Sprintf("request timed out after %s", constants.ProviderFetchTimeout),
			Err:        vertexCtx.Err(),
		}
	}
}

// listModelsViaGenAI uses the GenAI SDK to list models (works for both backends).
func (c *Client) listModelsViaGenAI(ctx context.Context, client *genai.Client) ([]catalogs.Model, error) {
	var models []catalogs.Model
	providerID := "google"
	c.mu.RLock()
	if c.provider != nil {
		providerID = string(c.provider.ID)
	}
	c.mu.RUnlock()
	logger := logging.FromContext(logging.WithProvider(ctx, providerID))

	// Get all base models with pagination
	baseModels, err := c.getAllModelsGenAI(ctx, client, true)
	if err != nil {
		logger.Warn().Err(err).Str("model_scope", "base").Msg("Could not list Google models")
	} else {
		for _, model := range baseModels {
			models = append(models, *model)
		}
	}

	// Get all tuned/custom models with pagination
	tunedModels, err := c.getAllModelsGenAI(ctx, client, false)
	if err != nil {
		logger.Warn().Err(err).Str("model_scope", "tuned").Msg("Could not list Google models")
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
	if c.provider.Catalog != nil && c.provider.Catalog.Sources[0].Endpoint.FeatureRules != nil {
		for _, rule := range c.provider.Catalog.Sources[0].Endpoint.FeatureRules {
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
		case "streamGenerateContent":
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
			model.Limits.InputTokens = int64(genaiModel.InputTokenLimit)
		}

		if genaiModel.OutputTokenLimit > 0 {
			model.Limits.OutputTokens = int64(genaiModel.OutputTokenLimit)
		}
	}

	// Metadata.ReleaseDate will be provided by models.dev during reconciliation
	// (models.dev is authoritative for metadata per authority hierarchy)
	c.applyProviderExtensions(model, genaiModel)

	return model
}

func (c *Client) convertAIStudioModel(rawModel aiStudioModel) *catalogs.Model {
	model := c.convertGenAIModel(&genai.Model{
		Name:             rawModel.Name,
		DisplayName:      rawModel.DisplayName,
		Description:      rawModel.Description,
		Version:          rawModel.Version,
		InputTokenLimit:  rawModel.InputTokenLimit,
		OutputTokenLimit: rawModel.OutputTokenLimit,
		SupportedActions: rawModel.SupportedGenerationMethods,
	})
	if rawModel.Temperature != nil || rawModel.MaxTemperature != nil || rawModel.TopP != nil || rawModel.TopK != nil {
		model.Generation = &catalogs.ModelGeneration{}
		if rawModel.Temperature != nil || rawModel.MaxTemperature != nil {
			model.Generation.Temperature = &catalogs.FloatRange{}
			if rawModel.Temperature != nil {
				model.Generation.Temperature.Default = *rawModel.Temperature
				model.Features.Temperature = true
			}
			if rawModel.MaxTemperature != nil {
				model.Generation.Temperature.Max = *rawModel.MaxTemperature
			}
		}
		if rawModel.TopP != nil {
			model.Generation.TopP = &catalogs.FloatRange{Default: *rawModel.TopP}
			model.Features.TopP = true
		}
		if rawModel.TopK != nil {
			model.Generation.TopK = &catalogs.IntRange{Default: int(*rawModel.TopK)}
			model.Features.TopK = true
		}
	}
	if rawModel.Thinking != nil {
		model.Features.Reasoning = *rawModel.Thinking
	}
	if len(rawModel.SupportedGenerationMethods) > 0 || rawModel.Thinking != nil {
		source := c.extensionSource()
		if model.Extensions == nil {
			model.Extensions = catalogs.SourceExtensions{}
		}
		extension := model.Extensions[source]
		if extension.Fields == nil {
			extension.Fields = make(map[string]any)
		}
		if len(rawModel.SupportedGenerationMethods) > 0 {
			methods := make([]any, 0, len(rawModel.SupportedGenerationMethods))
			for _, method := range rawModel.SupportedGenerationMethods {
				methods = append(methods, method)
			}
			extension.Fields["supported_generation_methods"] = methods
		}
		if rawModel.Thinking != nil {
			extension.Fields["thinking"] = *rawModel.Thinking
		}
		model.Extensions[source] = extension
	}
	if len(rawModel.UnknownFields) > 0 {
		source := c.extensionSource()
		if model.Extensions == nil {
			model.Extensions = catalogs.SourceExtensions{}
		}
		extension := model.Extensions[source]
		if extension.Fields == nil {
			extension.Fields = make(map[string]any)
		}
		extension.Fields["unknown_fields"] = rawModel.UnknownFields
		model.Extensions[source] = extension
	}
	return model
}

func (c *Client) applyProviderExtensions(model *catalogs.Model, genaiModel *genai.Model) {
	fields := make(map[string]any)
	if genaiModel.Version != "" {
		fields["version"] = genaiModel.Version
	}
	if genaiModel.DefaultCheckpointID != "" {
		fields["default_checkpoint_id"] = genaiModel.DefaultCheckpointID
	}
	if len(genaiModel.Labels) > 0 {
		labels := make(map[string]any, len(genaiModel.Labels))
		for key, value := range genaiModel.Labels {
			labels[key] = value
		}
		fields["labels"] = labels
	}
	if len(genaiModel.SupportedActions) > 0 {
		actions := make([]any, 0, len(genaiModel.SupportedActions))
		for _, action := range genaiModel.SupportedActions {
			actions = append(actions, action)
		}
		fields["supported_actions"] = actions
	}
	if len(fields) == 0 {
		return
	}
	source := c.extensionSource()
	model.Extensions = catalogs.SourceExtensions{
		source: {Fields: fields},
	}
}

func (c *Client) extensionSource() string {
	if c.provider != nil && c.provider.ID != "" {
		return c.provider.ID.String()
	}
	return catalogs.ProviderIDGoogleAIStudio.String()
}

// getModelGardenModels returns pre-defined Model Garden models based on configured authors.
func (c *Client) getModelGardenModels() []*catalogs.Model {
	var models []*catalogs.Model

	// Only include Model Garden models if authors are configured
	authors := c.provider.Catalog.Sources[0].Authors
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
	if c.projectID != "" {
		return c.projectID
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

	if c.location != "" {
		return c.location
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
				Component: string(catalogs.ProviderIDGoogleVertex),
				Message:   "project binding is unavailable from configured inputs and the cloud profile",
			}
		}
	} else {
		// For AI Studio, just check API key
		if _, found := c.auth.APIKey(); !found {
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
