package modelsdev

import (
	"context"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	defaultHTTPOutputDir = "internal/embedded/catalog/providers"
)

// Package-level state for expensive HTTP operations
var (
	httpOnce sync.Once
	httpAPI  *ModelsDevAPI
	httpErr  error
	httpDir  string
)

// HTTPSource enhances models with models.dev data via HTTP
type HTTPSource struct{}

// NewHTTPSource creates a new models.dev HTTP source
func NewHTTPSource() *HTTPSource {
	return &HTTPSource{}
}

// Name returns the name of this source
func (s *HTTPSource) Name() sources.SourceName {
	return sources.ModelsDevHTTP
}

// ensureHTTPAPI initializes models.dev data once via HTTP
func ensureHTTPAPI(outputDir string) (*ModelsDevAPI, error) {
	httpOnce.Do(func() {
		if outputDir == "" {
			outputDir = defaultHTTPOutputDir
		}
		httpDir = outputDir

		client := NewHTTPClient(outputDir)
		if err := client.EnsureAPI(); err != nil {
			httpErr = err
			return
		}
		httpAPI, httpErr = ParseAPI(client.GetAPIPath())
	})
	return httpAPI, httpErr
}

// Setup initializes the source with dependencies
func (s *HTTPSource) Setup(providers *catalogs.Providers) error {
	// HTTPSource doesn't need provider configs
	return nil
}

// Fetch creates a catalog with models that have pricing/limits data from models.dev
func (s *HTTPSource) Fetch(ctx context.Context, opts ...sources.SourceOption) (catalogs.Catalog, error) {
	// Apply options (not currently used by HTTPSource, but kept for consistency)
	_ = sources.ApplyOptions(opts...)

	// Create a new catalog to build into
	catalog, err := catalogs.New()
	if err != nil {
		return nil, errors.WrapResource("create", "memory catalog", "", err)
	}

	// Set the default merge strategy for models.dev catalog (enhances with pricing/limits)
	catalog.SetMergeStrategy(catalogs.MergeEnrichEmpty)

	// Note: Source disabling should be handled at orchestration level

	// We'll return only models with pricing/limits data
	// The merge strategy will handle combining with existing models

	// Note: Output directory is now handled by catalog Save() method
	outputDir := defaultHTTPOutputDir

	// Initialize models.dev data once
	api, err := ensureHTTPAPI(outputDir)
	if err != nil {
		return nil, errors.WrapResource("initialize", "models.dev HTTP", "", err)
	}

	// Add only models with pricing/limits data from models.dev
	added := 0
	for _, provider := range *api {
		for _, mdModel := range provider.Models {
			// Only include models that have pricing or limits data
			if (mdModel.Cost != nil && (mdModel.Cost.Input != nil || mdModel.Cost.Output != nil)) ||
				mdModel.Limit.Context > 0 || mdModel.Limit.Output > 0 {
				// Convert to starmap model with pricing/limits
				model := s.convertToStarmapModel(mdModel)
				if err := catalog.SetModel(model); err != nil {
					return nil, errors.WrapResource("set", "model", model.ID, err)
				}
				added++
			}
		}
	}

	logging.Info().
		Int("model_count", added).
		Msg("Found models with pricing/limits from models.dev HTTP")
	return catalog, nil
}

// Cleanup releases any resources
func (s *HTTPSource) Cleanup() error {
	// HTTPSource doesn't hold persistent resources
	return nil
}

// convertToStarmapModel converts a models.dev model to starmap model with pricing/limits
func (s *HTTPSource) convertToStarmapModel(mdModel ModelsDevModel) catalogs.Model {
	model := catalogs.Model{
		ID:   mdModel.ID,
		Name: mdModel.Name,
	}

	// Add pricing if available
	if mdModel.Cost != nil && (mdModel.Cost.Input != nil || mdModel.Cost.Output != nil) {
		model.Pricing = &catalogs.ModelPricing{
			Currency: "USD", // models.dev uses USD
			Tokens:   &catalogs.TokenPricing{},
		}

		// Map input cost (models.dev uses cost per 1M tokens)
		if mdModel.Cost.Input != nil && *mdModel.Cost.Input > 0 {
			model.Pricing.Tokens.Input = &catalogs.TokenCost{
				Per1M: *mdModel.Cost.Input,
			}
		}

		// Map output cost
		if mdModel.Cost.Output != nil && *mdModel.Cost.Output > 0 {
			model.Pricing.Tokens.Output = &catalogs.TokenCost{
				Per1M: *mdModel.Cost.Output,
			}
		}
	}

	// Add limits if available
	if mdModel.Limit.Context > 0 || mdModel.Limit.Output > 0 {
		model.Limits = &catalogs.ModelLimits{}

		if mdModel.Limit.Context > 0 {
			model.Limits.ContextWindow = int64(mdModel.Limit.Context)
		}

		if mdModel.Limit.Output > 0 {
			model.Limits.OutputTokens = int64(mdModel.Limit.Output)
		}
	}

	return model
}
