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
	defaultOutputDir = "internal/embedded/catalog/providers"
)

// Shared state for models.dev repository
var (
	sharedMu     sync.Mutex
	sharedAPI    *ModelsDevAPI
	sharedClient *Client
	sharedDir    string
)

func init() {
	// Register a single instance
	registry.Register(&ModelsDevGitSource{
		priority: 100,
		name:     "models.dev (git)",
	})
}

// ModelsDevGitSource wraps models.dev git client as a Source
type ModelsDevGitSource struct {
	api      *ModelsDevAPI
	client   *Client
	priority int
	name     string
	mu       sync.RWMutex
}

// Type returns the source type
func (s *ModelsDevGitSource) Type() sources.Type {
	return sources.ModelsDevGit
}

// Configure prepares the source with runtime configuration
func (s *ModelsDevGitSource) Configure(config sources.SourceConfig) error {
	if config.SyncOptions != nil && config.SyncOptions.DisableModelsDevGit {
		return nil
	}

	outputDir := defaultOutputDir
	if config.SyncOptions != nil && config.SyncOptions.OutputDir != "" {
		outputDir = config.SyncOptions.OutputDir
	}

	// Use shared state to avoid multiple clones
	sharedMu.Lock()
	defer sharedMu.Unlock()

	if sharedAPI != nil && sharedDir == outputDir {
		// Reuse existing setup
		s.mu.Lock()
		s.api = sharedAPI
		s.client = sharedClient
		s.mu.Unlock()
		return nil
	}

	// Initialize models.dev
	client := NewClient(outputDir)
	if err := client.EnsureRepository(); err != nil {
		return err
	}
	if err := client.BuildAPI(); err != nil {
		return err
	}
	api, err := ParseAPI(client.GetAPIPath())
	if err != nil {
		return err
	}

	// Update shared state
	sharedAPI = api
	sharedClient = client
	sharedDir = outputDir

	// Set instance state
	s.mu.Lock()
	s.api = api
	s.client = client
	s.mu.Unlock()

	return nil
}

// Reset clears any configuration
func (s *ModelsDevGitSource) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.api = nil
	s.client = nil
}

// Clone creates a copy of this source for concurrent use
func (s *ModelsDevGitSource) Clone() sources.Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &ModelsDevGitSource{
		api:      s.api,
		client:   s.client,
		priority: s.priority,
		name:     s.name,
	}
}

// Name returns a human-readable name for this source
func (s *ModelsDevGitSource) Name() string {
	return s.name
}

// Priority returns the priority of this source
func (s *ModelsDevGitSource) Priority() int {
	return s.priority
}

// FetchModels fetches models from models.dev for a specific provider
func (s *ModelsDevGitSource) FetchModels(ctx context.Context, providerID catalogs.ProviderID) ([]catalogs.Model, error) {
	if s.api == nil {
		return nil, fmt.Errorf("models.dev API not available")
	}

	// Get models from models.dev for this provider
	modelsDevProvider, exists := s.api.GetProvider(providerID)
	if !exists {
		// Not an error - provider might not be in models.dev yet
		return nil, nil
	}

	var starmapModels []catalogs.Model
	for _, modelsDevModel := range modelsDevProvider.Models {
		// Convert models.dev model to starmap model
		starmapModel, err := modelsDevModel.ToStarmapModel()
		if err != nil {
			// Log the error but continue with other models
			continue
		}
		starmapModels = append(starmapModels, *starmapModel)
	}

	return starmapModels, nil
}

// FetchProvider fetches provider information from models.dev
func (s *ModelsDevGitSource) FetchProvider(ctx context.Context, providerID catalogs.ProviderID) (*catalogs.Provider, error) {
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
func (s *ModelsDevGitSource) GetFieldAuthorities() []sources.FieldAuthority {
	return sources.FilterAuthoritiesBySource(sources.DefaultModelFieldAuthorities, sources.ModelsDevGit)
}

// IsAvailable returns true if models.dev data is available
func (s *ModelsDevGitSource) IsAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.api != nil
}

// SetPriority updates the priority of this source
func (s *ModelsDevGitSource) SetPriority(priority int) {
	s.priority = priority
}

// CopyProviderLogos copies provider logos from models.dev repository
func (s *ModelsDevGitSource) CopyProviderLogos(providerIDs []catalogs.ProviderID) error {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil
	}

	sharedMu.Lock()
	defer sharedMu.Unlock()
	return CopyProviderLogos(client, sharedDir, providerIDs)
}

// Cleanup removes the models.dev repository
func (s *ModelsDevGitSource) Cleanup() error {
	sharedMu.Lock()
	defer sharedMu.Unlock()

	if sharedClient != nil {
		err := sharedClient.Cleanup()
		sharedClient = nil
		sharedAPI = nil
		sharedDir = ""
		return err
	}
	return nil
}

// convertModelsDevProvider converts a models.dev provider to a starmap provider
func (s *ModelsDevGitSource) convertModelsDevProvider(modelsDevProvider *ModelsDevProvider, providerID catalogs.ProviderID) (*catalogs.Provider, error) {
	provider := &catalogs.Provider{
		ID: providerID,
	}

	// Extract basic information from models.dev provider
	// Note: This is a simplified conversion - actual implementation would depend
	// on the models.dev provider structure
	if modelsDevProvider.Name != "" {
		provider.Name = modelsDevProvider.Name
	}

	// Add any policy information if available in models.dev
	// This would need to be implemented based on the actual models.dev structure

	return provider, nil
}
