package providers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"

	// Import provider implementations for factory functions
	"github.com/agentstation/starmap/internal/sources/providers/anthropic"
	"github.com/agentstation/starmap/internal/sources/providers/cerebras"
	"github.com/agentstation/starmap/internal/sources/providers/deepseek"
	googleaistudio "github.com/agentstation/starmap/internal/sources/providers/google-ai-studio"
	googlevertex "github.com/agentstation/starmap/internal/sources/providers/google-vertex"
	"github.com/agentstation/starmap/internal/sources/providers/groq"
	"github.com/agentstation/starmap/internal/sources/providers/openai"
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
		return nil, fmt.Errorf("creating memory catalog: %w", err)
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
		providerIDs = listSupportedProviders()
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

	log.Printf("  Syncing %d providers concurrently...", len(providerConfigs))

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
			if p.IsAPIKeyRequired() && !p.HasAPIKey() {
				log.Printf("    %s: skipping (no API key)", p.ID)
				return
			}
			if len(p.GetMissingEnvVars()) > 0 {
				log.Printf("    %s: skipping (missing env vars: %v)", p.ID, p.GetMissingEnvVars())
				return
			}

			// Create NEW client instance with dedicated HTTP client
			client, err := getClient(p)
			if err != nil {
				log.Printf("    %s: skipping (%v)", p.ID, err)
				result.err = fmt.Errorf("%s: %w", p.ID, err)
				resultChan <- result
				return
			}

			// Fetch models from API
			models, err := client.ListModels(ctx)
			if err != nil {
				result.err = fmt.Errorf("%s: %w", p.ID, err)
				resultChan <- result
				return
			}

			result.models = models
			resultChan <- result

			log.Printf("    %s: fetched %d models", p.ID, len(models))
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

		// Add models to catalog with proper provider context
		for _, model := range result.models {
			// Create copy to avoid modifying original
			modelCopy := model
			if err := catalog.SetModel(modelCopy); err != nil {
				log.Printf("    Warning: failed to set model %s: %v", modelCopy.ID, err)
			}
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

// getClient creates a NEW client instance for the given provider.
// Each call returns a fresh client with its own HTTP client to avoid race conditions.
// This is now an internal implementation detail.
func getClient(provider *catalogs.Provider) (catalogs.Client, error) {
	// Create NEW client instance with dedicated HTTP client
	switch provider.ID {
	case catalogs.ProviderIDOpenAI:
		return openai.NewClient(provider), nil
	case catalogs.ProviderIDAnthropic:
		return anthropic.NewClient(provider), nil
	case catalogs.ProviderIDGroq:
		return groq.NewClient(provider), nil
	case catalogs.ProviderIDCerebras:
		return cerebras.NewClient(provider), nil
	case catalogs.ProviderIDDeepSeek:
		return deepseek.NewClient(provider), nil
	case catalogs.ProviderIDGoogleAIStudio:
		return googleaistudio.NewClient(provider), nil
	case catalogs.ProviderIDGoogleVertex:
		return googlevertex.NewClient(provider), nil // Uses env vars, not API key
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider.ID)
	}
}

// listSupportedProviders returns all provider IDs that have client implementations.
// This is now based on the switch statement in getClient, not a registry.
func listSupportedProviders() []catalogs.ProviderID {
	return []catalogs.ProviderID{
		catalogs.ProviderIDOpenAI,
		catalogs.ProviderIDAnthropic,
		catalogs.ProviderIDGroq,
		catalogs.ProviderIDCerebras,
		catalogs.ProviderIDDeepSeek,
		catalogs.ProviderIDGoogleAIStudio,
		catalogs.ProviderIDGoogleVertex,
	}
}

// Public API functions for backward compatibility and external use

// GetClient creates a NEW client instance for the given provider.
// Exposed for testing and advanced use cases.
func GetClient(provider *catalogs.Provider) (catalogs.Client, error) {
	return getClient(provider)
}

// HasClient checks if a provider ID has a client implementation.
func HasClient(id catalogs.ProviderID) bool {
	for _, supported := range listSupportedProviders() {
		if supported == id {
			return true
		}
	}
	return false
}

// ListSupportedProviders returns all provider IDs that have client implementations.
func ListSupportedProviders() []catalogs.ProviderID {
	return listSupportedProviders()
}
