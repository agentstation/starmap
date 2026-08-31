package google

import (
	"context"
	"strings"

	"google.golang.org/genai"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func (c *Client) extractModelID(name string) string {
	// Handle different formats:
	// - AI Studio: models/MODEL_ID
	// - Vertex: projects/PROJECT/locations/LOCATION/models/MODEL_ID
	// - Publisher: publishers/PUBLISHER/models/MODEL_ID

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

	model := &catalogs.Model{
		ID:          modelID,
		Name:        displayName,
		Description: genaiModel.Description,
	}

	if authorID, found := c.mappedPublisherAuthor(genaiModel.Name); found {
		model.Authors = []catalogs.Author{
			{ID: authorID, Name: authorID.String()},
		}
	}

	for _, action := range genaiModel.SupportedActions {
		switch action {
		case "streamGenerateContent":
			ensureGoogleModelFeatures(model).Streaming = true
		case "embedContent":
			ensureGoogleModelFeatures(model).Modalities.Output = []catalogs.ModelModality{
				catalogs.ModelModalityEmbedding,
			}
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

	c.applyProviderExtensions(model, genaiModel)

	return model
}

func (c *Client) mappedPublisherAuthor(name string) (catalogs.AuthorID, bool) {
	publisher := publisherFromModelName(name)
	if publisher == "" {
		return catalogs.AuthorIDUnknown, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.provider == nil || c.provider.Catalog == nil ||
		c.provider.Catalog.Endpoint.AuthorMapping == nil ||
		c.provider.Catalog.Endpoint.AuthorMapping.Field != "publisher" {
		return catalogs.AuthorIDUnknown, false
	}
	return c.provider.Catalog.Endpoint.AuthorMapping.Resolve(publisher)
}

func publisherFromModelName(name string) string {
	parts := strings.Split(strings.Trim(name, "/"), "/")
	for index := 0; index+1 < len(parts); index++ {
		if parts[index] == "publishers" {
			return parts[index+1]
		}
	}
	return ""
}

func ensureGoogleModelFeatures(model *catalogs.Model) *catalogs.ModelFeatures {
	if model.Features == nil {
		model.Features = &catalogs.ModelFeatures{}
	}
	return model.Features
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
		ensureGoogleModelFeatures(model).Temperature = true
	}
	if rawModel.Temperature != nil && rawModel.MaxTemperature != nil {
		generation.Temperature = &catalogs.FloatRange{
			Min: 0, Max: *rawModel.MaxTemperature, Default: *rawModel.Temperature,
		}
	}
	if rawModel.TopP != nil {
		ensureGoogleModelFeatures(model).TopP = true
		generation.TopP = &catalogs.FloatRange{Min: 0, Max: 1, Default: *rawModel.TopP}
	}
	if rawModel.TopK != nil {
		// The API reports the default but no maximum. Preserve support without
		// manufacturing a complete range that would fail closed on reload.
		ensureGoogleModelFeatures(model).TopK = true
	}
	if generation.Temperature != nil || generation.TopP != nil {
		model.Generation = generation
	}
	if rawModel.Thinking != nil {
		ensureGoogleModelFeatures(model).Reasoning = *rawModel.Thinking
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
