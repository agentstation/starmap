package providers

import (
	"context"
	"errors"
	"sync"

	"github.com/agentstation/starmap/internal/sources/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// No init() - no singleton registration
// Sources are created explicitly

// Source fetches models from all provider APIs concurrently
type Source struct {
	providers *catalogs.Providers // Provider configs injected during Setup
}

// New creates a new provider API source
func New() *Source {
	return &Source{}
}

// Name returns the name of this source
func (s *Source) Name() sources.SourceName {
	return sources.ProviderAPI
}

// providerModels holds models fetched from a specific provider
type providerModels struct {
	providerID catalogs.ProviderID
	models     []catalogs.Model
	err        error
}

// Setup initializes the source with provider configurations
func (s *Source) Setup(providers *catalogs.Providers) error {
	s.providers = providers
	return nil
}

// Fetch creates a new catalog with models fetched from all provider APIs concurrently
func (s *Source) Fetch(ctx context.Context, opts ...sources.SourceOption) (catalogs.Catalog, error) {
	// Apply options
	options := sources.ApplyOptions(opts...)

	// Create a new catalog to build into
	catalog, err := catalogs.New()
	if err != nil {
		return nil, pkgerrors.WrapResource("create", "memory catalog", "", err)
	}

	// Set the default merge strategy for provider catalog (fresh API data)
	catalog.SetMergeStrategy(catalogs.MergeReplaceAll)

	// Note: Source disabling should be handled at orchestration level

	// Check if we have provider configs
	if s.providers == nil {
		// Can't fetch without provider configs
		return catalog, nil
	}

	// Determine which providers to sync
	var providerIDs []catalogs.ProviderID
	if options.ProviderID != nil {
		providerIDs = []catalogs.ProviderID{*options.ProviderID}
	} else {
		providerIDs = registry.List()
	}

	// Get provider configs from injected providers
	var providerConfigs []*catalogs.Provider
	for _, id := range providerIDs {
		if p, found := s.providers.Get(id); found {
			providerConfigs = append(providerConfigs, p)
		}
	}

	if len(providerConfigs) == 0 {
		return catalog, nil // No providers to sync
	}

	// Add provider configurations to the catalog first
	for _, provider := range providerConfigs {
		if err := catalog.SetProvider(*provider); err != nil {
			logging.FromContext(ctx).Warn().
				Err(err).
				Str("provider_id", string(provider.ID)).
				Msg("Failed to add provider to catalog")
		}
	}

	logger := logging.FromContext(ctx)
	logger.Info().
		Int("provider_count", len(providerConfigs)).
		Msg("Syncing providers concurrently")

	// Sync all providers CONCURRENTLY
	var wg sync.WaitGroup
	resultChan := make(chan providerModels, len(providerConfigs))

	for _, provider := range providerConfigs {
		wg.Add(1)
		go func(p *catalogs.Provider) {
			defer wg.Done()

			result := providerModels{providerID: p.ID}

			// Load credentials
			p.LoadAPIKey()
			p.LoadEnvVars()

			// Check if provider has required credentials
			logger := logging.WithProvider(ctx, string(p.ID))
			if p.IsAPIKeyRequired() && !p.HasAPIKey() {
				logging.Ctx(logger).Debug().
					Str("provider_id", string(p.ID)).
					Msg("Skipping provider - no API key")
				return
			}
			if missingVars := p.MissingEnvVars(); len(missingVars) > 0 {
				logging.Ctx(logger).Debug().
					Str("provider_id", string(p.ID)).
					Strs("missing_env_vars", missingVars).
					Msg("Skipping provider - missing environment variables")
				return
			}

			// Create NEW client instance with dedicated HTTP client
			client, err := registry.Get(p)
			if err != nil {
				logging.Ctx(logger).Debug().
					Err(err).
					Str("provider_id", string(p.ID)).
					Msg("Skipping provider - client creation failed")
				result.err = &pkgerrors.SyncError{
				Provider: string(p.ID),
				Err:      err,
			}
				resultChan <- result
				return
			}

			// Fetch models from API
			models, err := client.ListModels(ctx)
			if err != nil {
				result.err = &pkgerrors.SyncError{
				Provider: string(p.ID),
				Err:      err,
			}
				resultChan <- result
				return
			}

			result.models = models
			resultChan <- result

			logging.Ctx(logger).Info().
				Str("provider_id", string(p.ID)).
				Int("model_count", len(models)).
				Msg("Fetched models")
		}(provider)
	}

	wg.Wait()
	close(resultChan)

	// Process results and update catalog
	var errs []error
	for result := range resultChan {
		if result.err != nil {
			errs = append(errs, result.err)
			continue
		}

		// Get the provider from catalog
		provider, err := catalog.Provider(result.providerID)
		if err != nil {
			logger.Warn().
				Err(err).
				Str("provider_id", string(result.providerID)).
				Msg("Failed to get provider from catalog")
			continue
		}

		// Initialize Models map if nil
		if provider.Models == nil {
			provider.Models = make(map[string]catalogs.Model)
		}

		// Add models to catalog and associate with provider
		for _, model := range result.models {
			// Create copy to avoid modifying original
			modelCopy := model
			if err := catalog.SetModel(modelCopy); err != nil {
				logger.Warn().
					Err(err).
					Str("model_id", modelCopy.ID).
					Msg("Failed to set model")
				continue
			}

			// Associate model with provider
			provider.Models[modelCopy.ID] = modelCopy
		}

		// Update the provider in the catalog with its models
		if err := catalog.SetProvider(provider); err != nil {
			logger.Warn().
				Err(err).
				Str("provider_id", string(result.providerID)).
				Msg("Failed to update provider with models")
		}

		// Note: Saving is now handled by the catalog's Save() method
		// Sources should only create catalogs, not persist them
	}

	if len(errs) > 0 {
		return catalog, errors.Join(errs...)
	}

	return catalog, nil
}

// Cleanup releases any resources
func (s *Source) Cleanup() error {
	// ProvidersSource doesn't hold persistent resources
	return nil
}
