package google

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentstation/utc"
	"google.golang.org/genai"

	"github.com/agentstation/starmap/pkg/catalogs"
)

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
		features.SetSupport(catalogs.ModelFeatureTools, rule.Value)
	case "tool_choice":
		features.SetSupport(catalogs.ModelFeatureToolChoice, rule.Value)
	case "tool_calls":
		features.SetSupport(catalogs.ModelFeatureToolCalls, rule.Value)
	case "structured_outputs":
		features.SetSupport(catalogs.ModelFeatureStructuredOutputs, rule.Value)
	case "reasoning":
		features.SetSupport(catalogs.ModelFeatureReasoning, rule.Value)
	case "top_k":
		features.SetSupport(catalogs.ModelFeatureTopK, rule.Value)
	case "format_response":
		features.SetSupport(catalogs.ModelFeatureFormatResponse, rule.Value)
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
	generation := &catalogs.ModelGeneration{}
	if rawModel.Temperature != nil {
		model.Features.Temperature = true
	}
	if rawModel.Temperature != nil && rawModel.MaxTemperature != nil {
		generation.Temperature = &catalogs.FloatRange{
			Min: 0, Max: *rawModel.MaxTemperature, Default: *rawModel.Temperature,
		}
	}
	if rawModel.TopP != nil {
		model.Features.TopP = true
		generation.TopP = &catalogs.FloatRange{Min: 0, Max: 1, Default: *rawModel.TopP}
	}
	if rawModel.TopK != nil {
		// The API reports the default but no maximum. Preserve support without
		// manufacturing a complete range that would fail closed on reload.
		model.Features.TopK = true
	}
	if generation.Temperature != nil || generation.TopP != nil {
		model.Generation = generation
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
