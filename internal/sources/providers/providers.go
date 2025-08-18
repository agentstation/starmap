package providers

import (
	"sync"

	"github.com/agentstation/starmap/internal/sources/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"

	// Import all provider implementations for side-effect registration
	_ "github.com/agentstation/starmap/internal/sources/providers/anthropic"
	_ "github.com/agentstation/starmap/internal/sources/providers/cerebras"
	_ "github.com/agentstation/starmap/internal/sources/providers/deepseek"
	_ "github.com/agentstation/starmap/internal/sources/providers/google-ai-studio"
	_ "github.com/agentstation/starmap/internal/sources/providers/google-vertex"
	_ "github.com/agentstation/starmap/internal/sources/providers/groq"
	_ "github.com/agentstation/starmap/internal/sources/providers/openai"
)

var (
	initOnce sync.Once
	initDone bool
	initMu   sync.RWMutex
)

func init() {
	// Maintain backward compatibility by calling Init() automatically
	Init()
}

// Init explicitly initializes all provider clients.
// This function is safe to call multiple times and is automatically called on package import.
// For advanced use cases, you can call this function explicitly to control when providers are loaded.
func Init() {
	initOnce.Do(func() {
		// Provider registration happens automatically via import side-effects
		// This function serves as an explicit initialization point for future extensibility
		initMu.Lock()
		initDone = true
		initMu.Unlock()
	})
}

// IsInitialized returns true if the providers have been initialized.
func IsInitialized() bool {
	initMu.RLock()
	defer initMu.RUnlock()
	return initDone
}

// InitWithOptions provides future extensibility for dynamic provider loading.
// Currently behaves the same as Init() but provides a foundation for future enhancements.
type InitOptions struct {
	// Future options could include:
	// ProviderFilters []string
	// DynamicLoading  bool
	// CustomProviders map[string]Client
}

func InitWithOptions(opts InitOptions) {
	// For now, just call Init()
	// Future implementations could selectively load providers based on options
	Init()
}


// GetClient returns a configured client for the given provider.
// This is a convenience wrapper around the registry that auto-registers
// all provider implementations.
func GetClient(provider *catalogs.Provider) (catalogs.Client, error) {
	return registry.GetClientForProvider(provider)
}

// HasClient checks if a provider ID has a registered client.
func HasClient(id catalogs.ProviderID) bool {
	return registry.HasClient(id)
}

// ListSupportedProviders returns all provider IDs that have registered clients.
func ListSupportedProviders() []catalogs.ProviderID {
	return registry.ListSupportedProviders()
}

// GetRegisteredClient returns the raw client instance for a provider ID.
// This is mainly for testing or advanced use cases.
func GetRegisteredClient(id catalogs.ProviderID) (catalogs.Client, bool) {
	return registry.GetRegisteredClient(id)
}

// RegisterClient registers a client instance for a provider ID.
// This is typically called by provider packages in their init() functions.
// Exposed for testing purposes.
func RegisterClient(id catalogs.ProviderID, client catalogs.Client) {
	registry.RegisterClient(id, client)
}