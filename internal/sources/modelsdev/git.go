package modelsdev

import (
	"context"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/utc"
)

const (
	defaultOutputDir = "internal/embedded/catalog/providers"
)

// Package-level state for expensive git operations
var (
	gitOnce sync.Once
	gitAPI  *ModelsDevAPI
	gitErr  error
	gitDir  string
)

// No init() - sources are created explicitly

// GitSource enhances models with models.dev data
type GitSource struct {
	providers *catalogs.Providers
	catalog   catalogs.Catalog
}

// NewGitSource creates a new models.dev git source
func NewGitSource() *GitSource {
	return &GitSource{}
}

// Type returns the type of this source
func (s *GitSource) Type() sources.Type {
	return sources.ModelsDevGit
}

// ensureGitRepo initializes models.dev data once using sync.Once
func ensureGitRepo(outputDir string) (*ModelsDevAPI, error) {
	gitOnce.Do(func() {
		if outputDir == "" {
			outputDir = defaultOutputDir
		}
		gitDir = outputDir

		client := NewClient(outputDir)
		if err := client.EnsureRepository(); err != nil {
			gitErr = err
			return
		}
		if err := client.BuildAPI(); err != nil {
			gitErr = err
			return
		}
		gitAPI, gitErr = ParseAPI(client.GetAPIPath())
	})

	// If the directory changed, we need a new sync.Once but that's rare
	// For now, just use what we have
	return gitAPI, gitErr
}

// Setup initializes the source with dependencies
func (s *GitSource) Setup(providers *catalogs.Providers) error {
	s.providers = providers
	return nil
}

// Fetch creates a catalog with models that have pricing/limits data from models.dev
func (s *GitSource) Fetch(ctx context.Context, opts ...sources.Option) error {
	// Apply options (not currently used by GitSource, but kept for consistency)
	_ = sources.ApplyOptions(opts...)

	// Create a new catalog to build into
	var err error
	s.catalog, err = catalogs.New()
	if err != nil {
		return errors.WrapResource("create", "memory catalog", "", err)
	}

	// Set the default merge strategy for models.dev catalog (enhances with pricing/limits)
	s.catalog.SetMergeStrategy(catalogs.MergeEnrichEmpty)

	// Note: Source disabling should be handled at orchestration level

	// We'll return only models with pricing/limits data
	// The merge strategy will handle combining with existing models

	// Note: Output directory is now handled by catalog Save() method
	outputDir := defaultOutputDir

	// Initialize models.dev data once
	api, err := ensureGitRepo(outputDir)
	if err != nil {
		return errors.WrapResource("initialize", "models.dev", "", err)
	}

	// Add providers with their models that have pricing/limits data from models.dev
	added := 0
	for _, mdProvider := range *api {
		// Convert provider ID from models.dev format
		providerID := catalogs.ProviderID(mdProvider.ID)

		// Get or create provider in catalog
		provider, err := s.catalog.Provider(providerID)
		if err != nil {
			// Provider doesn't exist, create a minimal one
			provider = catalogs.Provider{
				ID:   providerID,
				Name: mdProvider.ID, // Use ID as name for now
			}
		}

		// Initialize models map if needed
		if provider.Models == nil {
			provider.Models = make(map[string]catalogs.Model)
		}

		// Add models with pricing/limits data
		for _, mdModel := range mdProvider.Models {
			// Only include models that have pricing or limits data
			if (mdModel.Cost != nil && (mdModel.Cost.Input != nil || mdModel.Cost.Output != nil)) ||
				mdModel.Limit.Context > 0 || mdModel.Limit.Output > 0 {
				// Convert to starmap model with pricing/limits
				model := s.convertToStarmapModel(mdModel)
				provider.Models[model.ID] = model
				added++
			}
		}

		// Update provider in catalog if we added any models
		if len(provider.Models) > 0 {
			if err := s.catalog.SetProvider(provider); err != nil {
				return errors.WrapResource("set", "provider", string(provider.ID), err)
			}
		}
	}

	logging.Info().
		Int("model_count", added).
		Msg("Found models with pricing/limits from models.dev Git")
	return nil
}

// Catalog returns the catalog of this source
func (s *GitSource) Catalog() catalogs.Catalog {
	return s.catalog
}

// Cleanup releases any resources
func (s *GitSource) Cleanup() error {
	// GitSource doesn't hold persistent resources
	return nil
}

// convertToStarmapModel converts a models.dev model to starmap model with pricing/limits/metadata
func (s *GitSource) convertToStarmapModel(mdModel ModelsDevModel) catalogs.Model {
	model := catalogs.Model{
		ID:   mdModel.ID,
		Name: mdModel.Name,
	}

	// Add metadata if available
	if mdModel.ReleaseDate != "" || (mdModel.Knowledge != nil && *mdModel.Knowledge != "") {
		model.Metadata = &catalogs.ModelMetadata{}

		// Parse release date
		if mdModel.ReleaseDate != "" {
			if releaseDate, err := parseDate(mdModel.ReleaseDate); err == nil {
				model.Metadata.ReleaseDate = utc.Time{Time: *releaseDate}
			}
		}

		// Parse knowledge cutoff
		if mdModel.Knowledge != nil && *mdModel.Knowledge != "" {
			if knowledgeDate, err := parseDate(*mdModel.Knowledge); err == nil {
				knowledgeCutoff := utc.Time{Time: *knowledgeDate}
				model.Metadata.KnowledgeCutoff = &knowledgeCutoff
			}
		}

		// Set open weights flag
		model.Metadata.OpenWeights = mdModel.OpenWeights
	}

	// Add pricing if available
	if mdModel.Cost != nil && (mdModel.Cost.Input != nil || mdModel.Cost.Output != nil) {
		model.Pricing = &catalogs.ModelPricing{
			Currency: "USD", // models.dev uses USD
			Tokens:   &catalogs.ModelTokenPricing{},
		}

		// Map input cost (models.dev uses cost per 1M tokens)
		if mdModel.Cost.Input != nil && *mdModel.Cost.Input > 0 {
			model.Pricing.Tokens.Input = &catalogs.ModelTokenCost{
				Per1M: *mdModel.Cost.Input,
			}
		}

		// Map output cost
		if mdModel.Cost.Output != nil && *mdModel.Cost.Output > 0 {
			model.Pricing.Tokens.Output = &catalogs.ModelTokenCost{
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
