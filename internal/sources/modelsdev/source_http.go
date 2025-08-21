package modelsdev

import (
	"context"
	"fmt"
	"sync"

	"github.com/agentstation/starmap/internal/sources/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	defaultHTTPOutputDir = "internal/embedded/catalog/providers"
)

// Shared state for models.dev HTTP API
var (
	sharedHTTPMu     sync.Mutex
	sharedHTTPAPI    *ModelsDevAPI
	sharedHTTPClient *HTTPClient
	sharedHTTPDir    string
)

func init() {
	// Register HTTP source instance
	registry.Register(&ModelsDevHTTPSource{
		priority: 110, // Higher priority than git for faster access
		name:     "models.dev (http)",
	})
}

// ModelsDevHTTPSource wraps models.dev HTTP client as a Source
type ModelsDevHTTPSource struct {
	api      *ModelsDevAPI
	client   *HTTPClient
	priority int
	name     string
	mu       sync.RWMutex
}

// Type returns the source type
func (s *ModelsDevHTTPSource) Type() sources.Type {
	return sources.ModelsDevHTTP
}

// Configure prepares the source with runtime configuration
func (s *ModelsDevHTTPSource) Configure(config sources.SourceConfig) error {
	if config.SyncOptions != nil && config.SyncOptions.DisableModelsDevHTTP {
		return nil
	}

	outputDir := defaultHTTPOutputDir
	if config.SyncOptions != nil && config.SyncOptions.OutputDir != "" {
		outputDir = config.SyncOptions.OutputDir
	}

	// Use shared state to avoid multiple downloads
	sharedHTTPMu.Lock()
	defer sharedHTTPMu.Unlock()

	if sharedHTTPAPI != nil && sharedHTTPDir == outputDir {
		// Reuse existing setup
		s.mu.Lock()
		s.api = sharedHTTPAPI
		s.client = sharedHTTPClient
		s.mu.Unlock()
		return nil
	}

	// Initialize HTTP models.dev
	client := NewHTTPClient(outputDir)
	if err := client.EnsureAPI(); err != nil {
		return err
	}
	
	api, err := ParseAPI(client.GetAPIPath())
	if err != nil {
		return err
	}

	// Update shared state
	sharedHTTPAPI = api
	sharedHTTPClient = client
	sharedHTTPDir = outputDir

	// Set instance state
	s.mu.Lock()
	s.api = api
	s.client = client
	s.mu.Unlock()

	return nil
}

// Reset clears any configuration
func (s *ModelsDevHTTPSource) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.api = nil
	s.client = nil
}

// Clone creates a copy of this source for concurrent use
func (s *ModelsDevHTTPSource) Clone() sources.Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &ModelsDevHTTPSource{
		api:      s.api,
		client:   s.client,
		priority: s.priority,
		name:     s.name,
	}
}

// Name returns a human-readable name for this source
func (s *ModelsDevHTTPSource) Name() string {
	return s.name
}

// Priority returns the priority of this source
func (s *ModelsDevHTTPSource) Priority() int {
	return s.priority
}

// FetchModels fetches models from models.dev for a specific provider
func (s *ModelsDevHTTPSource) FetchModels(ctx context.Context, providerID catalogs.ProviderID) ([]catalogs.Model, error) {
	if s.api == nil {
		return nil, fmt.Errorf("models.dev API not available")
	}

	var allStarmapModels []catalogs.Model
	
	// Get provider aliases - some providers have multiple models.dev providers
	providerAliases := s.getProviderAliases(providerID)
	
	for _, alias := range providerAliases {
		// Get models from models.dev for this provider alias
		modelsDevProvider, exists := s.api.GetProvider(alias)
		if !exists {
			continue // Try next alias
		}

		for _, modelsDevModel := range modelsDevProvider.Models {
			// Convert models.dev model to starmap model
			starmapModel, err := modelsDevModel.ToStarmapModel()
			if err != nil {
				// Log the error but continue with other models
				continue
			}
			allStarmapModels = append(allStarmapModels, *starmapModel)
		}
	}

	return allStarmapModels, nil
}

// getProviderAliases returns a list of provider IDs to check in models.dev
// Some providers have models spread across multiple models.dev providers
func (s *ModelsDevHTTPSource) getProviderAliases(providerID catalogs.ProviderID) []catalogs.ProviderID {
	aliases := []catalogs.ProviderID{providerID} // Always include the original
	
	switch providerID {
	case "google-vertex":
		// Google Vertex has models under multiple models.dev providers
		aliases = append(aliases, "google-vertex-anthropic")
	case "openai":
		// OpenAI might have models under other aliases in the future
		// Add additional aliases as needed
	}
	
	return aliases
}

// FetchProvider fetches provider information from models.dev
func (s *ModelsDevHTTPSource) FetchProvider(ctx context.Context, providerID catalogs.ProviderID) (*catalogs.Provider, error) {
	if s.api == nil {
		return nil, fmt.Errorf("models.dev API not available")
	}

	// Get provider from models.dev
	modelsDevProvider, exists := s.api.GetProvider(providerID)
	if !exists {
		// Not an error - provider might not be in models.dev yet
		return nil, nil
	}

	// Convert models.dev provider to starmap provider
	starmapProvider, err := s.convertModelsDevProvider(modelsDevProvider, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert models.dev provider: %w", err)
	}

	return starmapProvider, nil
}

// GetFieldAuthorities returns the field authorities where models.dev is authoritative
func (s *ModelsDevHTTPSource) GetFieldAuthorities() []sources.FieldAuthority {
	return sources.FilterAuthoritiesBySource(sources.DefaultModelFieldAuthorities, sources.ModelsDevHTTP)
}

// IsAvailable returns true if models.dev data is available
func (s *ModelsDevHTTPSource) IsAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.api != nil
}

// SetPriority updates the priority of this source
func (s *ModelsDevHTTPSource) SetPriority(priority int) {
	s.priority = priority
}

// CopyProviderLogos copies provider logos (HTTP source cannot provide logos)
// Provider logos are only available through the Git source which has access to the
// models.dev repository files. To get logos, enable the Git source alongside HTTP.
func (s *ModelsDevHTTPSource) CopyProviderLogos(providerIDs []catalogs.ProviderID) error {
	if len(providerIDs) > 0 {
		fmt.Printf("  ℹ️  Note: HTTP source cannot copy provider logos. Enable git source for logo support.\n")
	}
	return nil
}

// Cleanup removes the models.dev cache
func (s *ModelsDevHTTPSource) Cleanup() error {
	sharedHTTPMu.Lock()
	defer sharedHTTPMu.Unlock()

	if sharedHTTPClient != nil {
		err := sharedHTTPClient.Cleanup()
		sharedHTTPClient = nil
		sharedHTTPAPI = nil
		sharedHTTPDir = ""
		return err
	}
	return nil
}

// convertModelsDevProvider converts a models.dev provider to a starmap provider
func (s *ModelsDevHTTPSource) convertModelsDevProvider(modelsDevProvider *ModelsDevProvider, providerID catalogs.ProviderID) (*catalogs.Provider, error) {
	provider := &catalogs.Provider{
		ID: providerID,
	}

	// Extract basic information from models.dev provider
	if modelsDevProvider.Name != "" {
		provider.Name = modelsDevProvider.Name
	}

	return provider, nil
}