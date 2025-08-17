package base

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// BaseCatalog provides common catalog functionality that can be embedded
// in specific catalog implementations (embedded, files, etc.)
type BaseCatalog struct {
	providers    *catalogs.Providers
	authors      *catalogs.Authors
	models       *catalogs.Models
	endpoints    *catalogs.Endpoints
	persister    PersistenceProvider
	cache        CacheProvider
	eventHandler EventHandler
}

// NewBaseCatalog creates a new base catalog with initialized collections
func NewBaseCatalog() BaseCatalog {
	return BaseCatalog{
		providers: catalogs.NewProviders(),
		authors:   catalogs.NewAuthors(),
		models:    catalogs.NewModels(),
		endpoints: catalogs.NewEndpoints(),
	}
}

// NewBaseCatalogWithPersistence creates a new base catalog with persistence support
func NewBaseCatalogWithPersistence(persister PersistenceProvider, cache CacheProvider, eventHandler EventHandler) BaseCatalog {
	return BaseCatalog{
		providers:    catalogs.NewProviders(),
		authors:      catalogs.NewAuthors(),
		models:       catalogs.NewModels(),
		endpoints:    catalogs.NewEndpoints(),
		persister:    persister,
		cache:        cache,
		eventHandler: eventHandler,
	}
}

// Providers returns the providers collection
func (c *BaseCatalog) Providers() *catalogs.Providers {
	return c.providers
}

// Authors returns the authors collection
func (c *BaseCatalog) Authors() *catalogs.Authors {
	return c.authors
}

// Models returns the models collection
func (c *BaseCatalog) Models() *catalogs.Models {
	return c.models
}

// Endpoints returns the endpoints collection
func (c *BaseCatalog) Endpoints() *catalogs.Endpoints {
	return c.endpoints
}

// Provider returns the provider with the given ID
func (c *BaseCatalog) Provider(id catalogs.ProviderID) (catalogs.Provider, error) {
	provider, ok := c.providers.Get(id)
	if !ok {
		return catalogs.Provider{}, fmt.Errorf("provider with ID %s not found", id)
	}
	return *provider, nil
}

// Author returns the author with the given ID
func (c *BaseCatalog) Author(id catalogs.AuthorID) (catalogs.Author, error) {
	author, ok := c.authors.Get(id)
	if !ok {
		return catalogs.Author{}, fmt.Errorf("author with ID %s not found", id)
	}
	return *author, nil
}

// Model returns the model with the given ID
func (c *BaseCatalog) Model(id string) (catalogs.Model, error) {
	model, ok := c.models.Get(id)
	if !ok {
		return catalogs.Model{}, fmt.Errorf("model with ID %s not found", id)
	}
	return *model, nil
}

// Endpoint returns the endpoint with the given ID
func (c *BaseCatalog) Endpoint(id string) (catalogs.Endpoint, error) {
	endpoint, ok := c.endpoints.Get(id)
	if !ok {
		return catalogs.Endpoint{}, fmt.Errorf("endpoint with ID %s not found", id)
	}
	return *endpoint, nil
}

// UpdateProvider updates the provider with the given ID
func (c *BaseCatalog) UpdateProvider(provider catalogs.Provider) error {
	return c.providers.Set(provider.ID, &provider)
}

// UpdateAuthor updates the author with the given ID
func (c *BaseCatalog) UpdateAuthor(author catalogs.Author) error {
	return c.authors.Set(author.ID, &author)
}

// UpdateModel updates the model with the given ID
func (c *BaseCatalog) UpdateModel(model catalogs.Model) error {
	if err := c.models.Set(model.ID, &model); err != nil {
		return err
	}

	// If persistence is configured, save the updated model
	if c.persister != nil {
		if err := c.persister.SaveModel(model); err != nil {
			return fmt.Errorf("failed to persist updated model: %w", err)
		}
	}

	// Trigger event handler if configured
	if c.eventHandler != nil {
		if err := c.eventHandler.OnModelSaved(model); err != nil {
			return fmt.Errorf("event handler error: %w", err)
		}
	}

	// Update cache if configured
	if c.cache != nil {
		c.cache.Set(fmt.Sprintf("model:%s", model.ID), model)
	}

	return nil
}

// UpdateEndpoint updates the endpoint with the given ID
func (c *BaseCatalog) UpdateEndpoint(endpoint catalogs.Endpoint) error {
	return c.endpoints.Set(endpoint.ID, &endpoint)
}

// DeleteProvider deletes the provider with the given ID
func (c *BaseCatalog) DeleteProvider(id catalogs.ProviderID) error {
	return c.providers.Delete(id)
}

// DeleteAuthor deletes the author with the given ID
func (c *BaseCatalog) DeleteAuthor(id catalogs.AuthorID) error {
	return c.authors.Delete(id)
}

// DeleteModel deletes the model with the given ID
func (c *BaseCatalog) DeleteModel(id string) error {
	if err := c.models.Delete(id); err != nil {
		return err
	}

	// If persistence is configured, delete the model from storage
	if c.persister != nil {
		if err := c.persister.DeleteModel(id); err != nil {
			return fmt.Errorf("failed to delete model from storage: %w", err)
		}
	}

	// Trigger event handler if configured
	if c.eventHandler != nil {
		if err := c.eventHandler.OnModelDeleted(id); err != nil {
			return fmt.Errorf("event handler error: %w", err)
		}
	}

	// Remove from cache if configured
	if c.cache != nil {
		c.cache.Delete(fmt.Sprintf("model:%s", id))
	}

	return nil
}

// DeleteEndpoint deletes the endpoint with the given ID
func (c *BaseCatalog) DeleteEndpoint(id string) error {
	return c.endpoints.Delete(id)
}

// AddProvider adds a provider to the catalog
func (c *BaseCatalog) AddProvider(provider catalogs.Provider) error {
	return c.providers.Add(&provider)
}

// AddAuthor adds an author to the catalog
func (c *BaseCatalog) AddAuthor(author catalogs.Author) error {
	return c.authors.Add(&author)
}

// AddModel adds a model to the catalog
func (c *BaseCatalog) AddModel(model catalogs.Model) error {
	if err := c.models.Add(&model); err != nil {
		return err
	}

	// If persistence is configured, save the model
	if c.persister != nil {
		if err := c.persister.SaveModel(model); err != nil {
			// Rollback the in-memory addition
			c.models.Delete(model.ID)
			return fmt.Errorf("failed to persist model: %w", err)
		}
	}

	// Trigger event handler if configured
	if c.eventHandler != nil {
		if err := c.eventHandler.OnModelSaved(model); err != nil {
			return fmt.Errorf("event handler error: %w", err)
		}
	}

	// Update cache if configured
	if c.cache != nil {
		c.cache.Set(fmt.Sprintf("model:%s", model.ID), model)
	}

	return nil
}

// AddEndpoint adds an endpoint to the catalog
func (c *BaseCatalog) AddEndpoint(endpoint catalogs.Endpoint) error {
	return c.endpoints.Add(&endpoint)
}

// Sync syncs the catalog with the given source
func (c *BaseCatalog) Sync(source catalogs.Catalog) error {
	// Sync all providers using batch upsert for efficiency
	if providersMap := source.Providers().Map(); len(providersMap) > 0 {
		if err := c.providers.SetBatch(providersMap); err != nil {
			return fmt.Errorf("failed to sync providers: %w", err)
		}
	}

	// Sync all authors using batch upsert for efficiency
	if authorsMap := source.Authors().Map(); len(authorsMap) > 0 {
		if err := c.authors.SetBatch(authorsMap); err != nil {
			return fmt.Errorf("failed to sync authors: %w", err)
		}
	}

	// Sync all models using batch upsert for efficiency
	if modelsMap := source.Models().Map(); len(modelsMap) > 0 {
		if err := c.models.SetBatch(modelsMap); err != nil {
			return fmt.Errorf("failed to sync models: %w", err)
		}
	}

	// Sync all endpoints using batch upsert for efficiency
	if endpointsMap := source.Endpoints().Map(); len(endpointsMap) > 0 {
		if err := c.endpoints.SetBatch(endpointsMap); err != nil {
			return fmt.Errorf("failed to sync endpoints: %w", err)
		}
	}

	return nil
}

// Copy creates a copy of the catalog using memory catalog
func (c *BaseCatalog) Copy() (catalogs.Catalog, error) {
	return c.CopyWith(func() catalogs.Catalog {
		return &BaseCatalog{
			providers: catalogs.NewProviders(),
			authors:   catalogs.NewAuthors(),
			models:    catalogs.NewModels(),
			endpoints: catalogs.NewEndpoints(),
		}
	})
}

// CopyWith copies the catalog using the provided constructor function
func (c *BaseCatalog) CopyWith(newCatalog func() catalogs.Catalog) (catalogs.Catalog, error) {
	// Create a new empty catalog using the constructor
	catalog := newCatalog()

	// Deep copy all providers
	if providersMap := c.providers.Map(); len(providersMap) > 0 {
		copiedProviders := make(map[catalogs.ProviderID]*catalogs.Provider, len(providersMap))
		for id, provider := range providersMap {
			// Create a new Provider instance (deep copy the struct)
			providerCopy := *provider
			copiedProviders[id] = &providerCopy
		}
		if err := catalog.Providers().SetBatch(copiedProviders); err != nil {
			return nil, fmt.Errorf("failed to copy providers: %w", err)
		}
	}

	// Deep copy all authors
	if authorsMap := c.authors.Map(); len(authorsMap) > 0 {
		copiedAuthors := make(map[catalogs.AuthorID]*catalogs.Author, len(authorsMap))
		for id, author := range authorsMap {
			// Create a new Author instance (deep copy the struct)
			authorCopy := *author
			copiedAuthors[id] = &authorCopy
		}
		if err := catalog.Authors().SetBatch(copiedAuthors); err != nil {
			return nil, fmt.Errorf("failed to copy authors: %w", err)
		}
	}

	// Deep copy all models
	if modelsMap := c.models.Map(); len(modelsMap) > 0 {
		copiedModels := make(map[string]*catalogs.Model, len(modelsMap))
		for id, model := range modelsMap {
			// Create a new Model instance (deep copy the struct)
			modelCopy := *model
			copiedModels[id] = &modelCopy
		}
		if err := catalog.Models().SetBatch(copiedModels); err != nil {
			return nil, fmt.Errorf("failed to copy models: %w", err)
		}
	}

	// Deep copy all endpoints
	if endpointsMap := c.endpoints.Map(); len(endpointsMap) > 0 {
		copiedEndpoints := make(map[string]*catalogs.Endpoint, len(endpointsMap))
		for id, endpoint := range endpointsMap {
			// Create a new Endpoint instance (deep copy the struct)
			endpointCopy := *endpoint
			copiedEndpoints[id] = &endpointCopy
		}
		if err := catalog.Endpoints().SetBatch(copiedEndpoints); err != nil {
			return nil, fmt.Errorf("failed to copy endpoints: %w", err)
		}
	}

	return catalog, nil
}

// SetPersister sets the persistence provider for the catalog
func (c *BaseCatalog) SetPersister(persister PersistenceProvider) {
	c.persister = persister
}

// GetPersister returns the current persistence provider
func (c *BaseCatalog) GetPersister() PersistenceProvider {
	return c.persister
}

// SetCache sets the cache provider for the catalog
func (c *BaseCatalog) SetCache(cache CacheProvider) {
	c.cache = cache
}

// GetCache returns the current cache provider
func (c *BaseCatalog) GetCache() CacheProvider {
	return c.cache
}

// SetEventHandler sets the event handler for the catalog
func (c *BaseCatalog) SetEventHandler(handler EventHandler) {
	c.eventHandler = handler
}

// GetEventHandler returns the current event handler
func (c *BaseCatalog) GetEventHandler() EventHandler {
	return c.eventHandler
}

// Save persists the entire catalog using the configured persister
func (c *BaseCatalog) Save() error {
	if c.persister == nil {
		return fmt.Errorf("no persistence provider configured")
	}

	if err := c.persister.Save(c); err != nil {
		return fmt.Errorf("failed to save catalog: %w", err)
	}

	// Trigger event handler if configured
	if c.eventHandler != nil {
		if err := c.eventHandler.OnCatalogSaved(c); err != nil {
			return fmt.Errorf("event handler error: %w", err)
		}
	}

	return nil
}

// Load loads the catalog from the configured persister
func (c *BaseCatalog) Load() error {
	if c.persister == nil {
		return fmt.Errorf("no persistence provider configured")
	}

	catalog, err := c.persister.Load()
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	// Replace current collections with loaded data
	c.providers = catalog.Providers()
	c.authors = catalog.Authors()
	c.models = catalog.Models()
	c.endpoints = catalog.Endpoints()

	// Trigger event handler if configured
	if c.eventHandler != nil {
		if err := c.eventHandler.OnCatalogLoaded(c); err != nil {
			return fmt.Errorf("event handler error: %w", err)
		}
	}

	return nil
}

// LoadWithCache loads a model using cache-first strategy
func (c *BaseCatalog) LoadWithCache(id string) (catalogs.Model, error) {
	// Try cache first
	if c.cache != nil {
		if cached, found := c.cache.Get(fmt.Sprintf("model:%s", id)); found {
			if model, ok := cached.(catalogs.Model); ok {
				return model, nil
			}
		}
	}

	// Fall back to regular model lookup
	model, err := c.Model(id)
	if err != nil {
		return catalogs.Model{}, err
	}

	// Update cache
	if c.cache != nil {
		c.cache.Set(fmt.Sprintf("model:%s", id), model)
	}

	return model, nil
}
