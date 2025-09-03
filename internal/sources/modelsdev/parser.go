package modelsdev

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/utc"
)

// ModelsDevAPI represents the structure of models.dev api.json
type ModelsDevAPI map[string]ModelsDevProvider

// ModelsDevProvider represents a provider in models.dev
type ModelsDevProvider struct {
	ID     string                    `json:"id"`
	Env    []string                  `json:"env"`
	NPM    string                    `json:"npm"`
	API    *string                   `json:"api,omitempty"`
	Name   string                    `json:"name"`
	Doc    string                    `json:"doc"`
	Models map[string]ModelsDevModel `json:"models"`
}

// ModelsDevModel represents a model in models.dev
type ModelsDevModel struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Attachment  bool                `json:"attachment"`
	Reasoning   bool                `json:"reasoning"`
	Temperature bool                `json:"temperature"`
	ToolCall    bool                `json:"tool_call"`
	Knowledge   *string             `json:"knowledge,omitempty"`
	ReleaseDate string              `json:"release_date"`
	LastUpdated string              `json:"last_updated"`
	Modalities  ModelsDevModalities `json:"modalities"`
	OpenWeights bool                `json:"open_weights"`
	Cost        *ModelsDevCost      `json:"cost,omitempty"`
	Limit       ModelsDevLimit      `json:"limit"`
}

// ModelsDevModalities represents input/output modalities
type ModelsDevModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// ModelsDevCost represents pricing information
type ModelsDevCost struct {
	Input      *float64 `json:"input,omitempty"`
	Output     *float64 `json:"output,omitempty"`
	Cache      *float64 `json:"cache,omitempty"`       // Legacy cache field
	CacheRead  *float64 `json:"cache_read,omitempty"`  // Cache read costs
	CacheWrite *float64 `json:"cache_write,omitempty"` // Cache write costs
}

// ModelsDevLimit represents model limits
type ModelsDevLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

// ParseAPI parses the api.json file and returns a ModelsDevAPI
func ParseAPI(apiPath string) (*ModelsDevAPI, error) {
	data, err := os.ReadFile(apiPath)
	if err != nil {
		return nil, errors.WrapIO("read", apiPath, err)
	}

	var api ModelsDevAPI
	if err := json.Unmarshal(data, &api); err != nil {
		return nil, errors.WrapParse("json", "api.json", err)
	}

	return &api, nil
}

// ToStarmapProvider converts a ModelsDevProvider to a starmap.Provider
func (p *ModelsDevProvider) ToStarmapProvider() (*catalogs.Provider, error) {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderID(p.ID),
		Name: p.Name,
	}

	// Convert models
	if len(p.Models) > 0 {
		provider.Models = make(map[string]catalogs.Model)
		for modelID, model := range p.Models {
			starmapModel, err := model.ToStarmapModel()
			if err != nil {
				return nil, errors.WrapResource("convert", "model", modelID, err)
			}
			provider.Models[modelID] = *starmapModel
		}
	}

	return provider, nil
}

// ToStarmapModel converts a ModelsDevModel to a starmap.Model
func (m *ModelsDevModel) ToStarmapModel() (*catalogs.Model, error) {
	model := &catalogs.Model{
		ID:   m.ID,
		Name: m.Name,
	}

	// Set metadata including OpenWeights flag
	model.Metadata = &catalogs.ModelMetadata{
		OpenWeights: m.OpenWeights,
	}

	// Convert modalities
	if len(m.Modalities.Input) > 0 || len(m.Modalities.Output) > 0 {
		model.Features = &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  convertModalities(m.Modalities.Input),
				Output: convertModalities(m.Modalities.Output),
			},
		}

		// Set feature flags based on models.dev data
		model.Features.Temperature = m.Temperature
		model.Features.ToolCalls = m.ToolCall
		model.Features.Reasoning = m.Reasoning
		model.Features.Attachments = m.Attachment
	}

	// Convert limits
	if m.Limit.Context > 0 || m.Limit.Output > 0 {
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: int64(m.Limit.Context),
			OutputTokens:  int64(m.Limit.Output),
		}
	}

	// Convert pricing
	if m.Cost != nil {
		model.Pricing = &catalogs.ModelPricing{
			Currency: "USD", // models.dev uses USD
		}

		// Initialize token pricing
		tokenPricing := &catalogs.ModelTokenPricing{}

		if m.Cost.Input != nil {
			tokenPricing.Input = &catalogs.ModelTokenCost{
				Per1M: *m.Cost.Input,
			}
		}
		if m.Cost.Output != nil {
			tokenPricing.Output = &catalogs.ModelTokenCost{
				Per1M: *m.Cost.Output,
			}
		}
		// Handle cache costs (prefer specific cache_read/cache_write over legacy cache field)
		if m.Cost.CacheRead != nil || m.Cost.CacheWrite != nil || m.Cost.Cache != nil {
			cacheCost := &catalogs.ModelTokenCachePricing{}

			if m.Cost.CacheRead != nil {
				cacheCost.Read = &catalogs.ModelTokenCost{
					Per1M: *m.Cost.CacheRead,
				}
			}
			if m.Cost.CacheWrite != nil {
				cacheCost.Write = &catalogs.ModelTokenCost{
					Per1M: *m.Cost.CacheWrite,
				}
			}
			// Legacy fallback: if no specific cache_read/cache_write, use cache for write
			if m.Cost.Cache != nil && cacheCost.Read == nil && cacheCost.Write == nil {
				cacheCost.Write = &catalogs.ModelTokenCost{
					Per1M: *m.Cost.Cache,
				}
			}

			tokenPricing.Cache = cacheCost
		}

		model.Pricing.Tokens = tokenPricing
	}

	// Parse dates
	if m.ReleaseDate != "" {
		if releaseDate, err := parseDate(m.ReleaseDate); err == nil {
			if model.Metadata == nil {
				model.Metadata = &catalogs.ModelMetadata{}
			}
			model.Metadata.ReleaseDate = utc.Time{Time: *releaseDate}
		}
	}

	if m.LastUpdated != "" {
		if lastUpdated, err := parseDate(m.LastUpdated); err == nil {
			model.UpdatedAt = utc.Time{Time: *lastUpdated}
		}
	}

	// Parse knowledge cutoff
	if m.Knowledge != nil && *m.Knowledge != "" {
		if knowledgeDate, err := parseDate(*m.Knowledge); err == nil {
			if model.Metadata == nil {
				model.Metadata = &catalogs.ModelMetadata{}
			}
			knowledgeCutoff := utc.Time{Time: *knowledgeDate}
			model.Metadata.KnowledgeCutoff = &knowledgeCutoff
		}
	}

	return model, nil
}

// convertModalities converts string modalities to starmap.ModelModality
func convertModalities(modalities []string) []catalogs.ModelModality {
	var result []catalogs.ModelModality
	for _, modality := range modalities {
		switch strings.ToLower(modality) {
		case "text":
			result = append(result, catalogs.ModelModalityText)
		case "image":
			result = append(result, catalogs.ModelModalityImage)
		case "audio":
			result = append(result, catalogs.ModelModalityAudio)
		case "video":
			result = append(result, catalogs.ModelModalityVideo)
		}
	}
	return result
}

// parseDate parses various date formats used in models.dev
func parseDate(dateStr string) (*time.Time, error) {
	// Try different date formats
	formats := []string{
		"2006-01-02",           // YYYY-MM-DD
		"2006-01",              // YYYY-MM
		"2006",                 // YYYY
		time.RFC3339,           // ISO 8601 with timezone
		"2006-01-02T15:04:05Z", // ISO 8601 UTC
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return &t, nil
		}
	}

	// If all parsing fails, try to extract year from the string
	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil && year > 1900 && year < 3000 {
			t := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
			return &t, nil
		}
	}

	return nil, errors.WrapParse("date", dateStr, errors.New("unsupported format"))
}

// GetProvider returns a specific provider from the API data
func (api *ModelsDevAPI) GetProvider(providerID catalogs.ProviderID) (*ModelsDevProvider, bool) {
	provider, exists := (*api)[string(providerID)]
	return &provider, exists
}

// GetModel returns a specific model from a provider
func (p *ModelsDevProvider) Model(modelID string) (*ModelsDevModel, bool) {
	model, exists := p.Models[modelID]
	return &model, exists
}
