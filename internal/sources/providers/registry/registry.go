package registry

import (
	"fmt"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
)

var (
	mu      sync.RWMutex
	clients = make(map[catalogs.ProviderID]catalogs.Client)
)

func init() {
	// Inject the client getter function to break circular dependency
	catalogs.SetClientGetter(GetClientForProvider)
}

// RegisterClient registers a client instance for a provider ID.
// This is called by provider packages in their init() functions.
func RegisterClient(id catalogs.ProviderID, client catalogs.Client) {
	mu.Lock()
	defer mu.Unlock()
	clients[id] = client
}

// GetClientForProvider returns a client configured with the provider's loaded API key.
func GetClientForProvider(provider *catalogs.Provider) (catalogs.Client, error) {
	mu.RLock()
	client, exists := clients[provider.ID]
	mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no client registered for provider: %s", provider.ID)
	}

	// Configure client with provider (which contains API key value)
	if configurable, ok := client.(interface {
		Configure(*catalogs.Provider)
	}); ok {
		configurable.Configure(provider)
	}

	return client, nil
}

// HasClient checks if a provider ID has a registered client.
func HasClient(id catalogs.ProviderID) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, exists := clients[id]
	return exists
}

// ListSupportedProviders returns all provider IDs that have registered clients.
func ListSupportedProviders() []catalogs.ProviderID {
	mu.RLock()
	defer mu.RUnlock()

	ids := make([]catalogs.ProviderID, 0, len(clients))
	for id := range clients {
		ids = append(ids, id)
	}
	return ids
}

// GetRegisteredClient returns the raw client instance for a provider ID.
// This is mainly for testing or advanced use cases.
func GetRegisteredClient(id catalogs.ProviderID) (catalogs.Client, bool) {
	mu.RLock()
	defer mu.RUnlock()
	client, exists := clients[id]
	return client, exists
}
