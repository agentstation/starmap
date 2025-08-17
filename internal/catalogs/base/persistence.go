package base

import "github.com/agentstation/starmap/pkg/catalogs"

// PersistenceProvider defines the interface for catalog persistence operations
type PersistenceProvider interface {
	// Save persists a catalog to storage
	Save(catalog catalogs.Catalog) error

	// Load loads a catalog from storage
	Load() (catalogs.Catalog, error)

	// SaveModel persists a single model to storage
	SaveModel(model catalogs.Model) error

	// LoadModel loads a single model from storage
	LoadModel(id string) (catalogs.Model, error)

	// DeleteModel removes a model from storage
	DeleteModel(id string) error
}

// ModelPersister handles model-specific persistence operations
type ModelPersister interface {
	// SaveModels saves multiple models to storage
	SaveModels(models []catalogs.Model) error

	// LoadModels loads all models from storage
	LoadModels() ([]catalogs.Model, error)

	// SaveProviderModels saves models for a specific provider
	SaveProviderModels(providerID catalogs.ProviderID, models []catalogs.Model) error

	// LoadProviderModels loads all models for a specific provider
	LoadProviderModels(providerID catalogs.ProviderID) ([]catalogs.Model, error)

	// CleanProviderModels removes all models for a specific provider
	CleanProviderModels(providerID catalogs.ProviderID) error
}

// ConfigPersister handles configuration-level persistence
type ConfigPersister interface {
	// SaveProviders saves providers configuration
	SaveProviders(providers map[catalogs.ProviderID]*catalogs.Provider) error

	// LoadProviders loads providers configuration
	LoadProviders() (map[catalogs.ProviderID]*catalogs.Provider, error)

	// SaveAuthors saves authors configuration
	SaveAuthors(authors map[catalogs.AuthorID]*catalogs.Author) error

	// LoadAuthors loads authors configuration
	LoadAuthors() (map[catalogs.AuthorID]*catalogs.Author, error)

	// SaveEndpoints saves endpoints configuration
	SaveEndpoints(endpoints map[string]*catalogs.Endpoint) error

	// LoadEndpoints loads endpoints configuration
	LoadEndpoints() (map[string]*catalogs.Endpoint, error)
}

// TransactionPersister provides transactional persistence operations
type TransactionPersister interface {
	// BeginTransaction starts a new transaction
	BeginTransaction() (Transaction, error)
}

// Transaction represents a persistence transaction
type Transaction interface {
	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error

	// SaveModel saves a model within the transaction
	SaveModel(model catalogs.Model) error

	// DeleteModel deletes a model within the transaction
	DeleteModel(id string) error
}

// CacheProvider defines caching capabilities for catalog operations
type CacheProvider interface {
	// Get retrieves an item from cache
	Get(key string) (interface{}, bool)

	// Set stores an item in cache
	Set(key string, value interface{}) error

	// Delete removes an item from cache
	Delete(key string) error

	// Clear clears all cache entries
	Clear() error

	// Size returns the number of items in cache
	Size() int
}

// ValidatingPersister adds validation capabilities to persistence
type ValidatingPersister interface {
	PersistenceProvider

	// ValidateModel validates a model before persistence
	ValidateModel(model catalogs.Model) error

	// ValidateProvider validates a provider before persistence
	ValidateProvider(provider catalogs.Provider) error

	// ValidateAuthor validates an author before persistence
	ValidateAuthor(author catalogs.Author) error
}

// EventHandler defines callback hooks for persistence events
type EventHandler interface {
	// OnModelSaved is called after a model is successfully saved
	OnModelSaved(model catalogs.Model) error

	// OnModelLoaded is called after a model is successfully loaded
	OnModelLoaded(model catalogs.Model) error

	// OnModelDeleted is called after a model is successfully deleted
	OnModelDeleted(id string) error

	// OnCatalogSaved is called after a catalog is successfully saved
	OnCatalogSaved(catalog catalogs.Catalog) error

	// OnCatalogLoaded is called after a catalog is successfully loaded
	OnCatalogLoaded(catalog catalogs.Catalog) error
}
