package modelsdev

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Catalog implements starmap.Catalog interface for models.dev data
type Catalog struct {
	api       *ModelsDevAPI
	providers *catalogs.Providers
	authors   *catalogs.Authors
	models    *catalogs.Models
	endpoints *catalogs.Endpoints
}

// NewCatalog creates a new models.dev catalog from parsed API data
func NewCatalog(api *ModelsDevAPI) (*Catalog, error) {
	if api == nil {
		return nil, fmt.Errorf("api data is nil")
	}

	catalog := &Catalog{
		api:       api,
		providers: catalogs.NewProviders(),
		authors:   catalogs.NewAuthors(),
		models:    catalogs.NewModels(),
		endpoints: catalogs.NewEndpoints(),
	}

	// Convert models.dev data to starmap structures
	if err := catalog.loadFromAPI(); err != nil {
		return nil, fmt.Errorf("loading from API: %w", err)
	}

	return catalog, nil
}

// Load implements starmap.Catalog
func (c *Catalog) Load() error {
	// Already loaded in NewCatalog
	return nil
}

// Save implements starmap.Catalog
func (c *Catalog) Save() error {
	// models.dev catalog is read-only
	return fmt.Errorf("models.dev catalog is read-only")
}

// Providers implements starmap.Catalog
func (c *Catalog) Providers() *catalogs.Providers {
	return c.providers
}

// Authors implements starmap.Catalog
func (c *Catalog) Authors() *catalogs.Authors {
	return c.authors
}

// Models implements starmap.Catalog
func (c *Catalog) Models() *catalogs.Models {
	return c.models
}

// Endpoints implements starmap.Catalog
func (c *Catalog) Endpoints() *catalogs.Endpoints {
	return c.endpoints
}

// Provider implements starmap.Catalog
func (c *Catalog) Provider(id catalogs.ProviderID) (*catalogs.Provider, error) {
	provider, exists := c.providers.Get(id)
	if !exists {
		return nil, fmt.Errorf("provider %s not found", id)
	}
	return provider, nil
}

// Author implements starmap.Catalog
func (c *Catalog) Author(id catalogs.AuthorID) (*catalogs.Author, error) {
	author, exists := c.authors.Get(id)
	if !exists {
		return nil, fmt.Errorf("author %s not found", id)
	}
	return author, nil
}

// Model implements starmap.Catalog
func (c *Catalog) Model(id string) (*catalogs.Model, error) {
	model, exists := c.models.Get(id)
	if !exists {
		return nil, fmt.Errorf("model %s not found", id)
	}
	return model, nil
}

// Endpoint implements starmap.Catalog
func (c *Catalog) Endpoint(id string) (*catalogs.Endpoint, error) {
	endpoint, exists := c.endpoints.Get(id)
	if !exists {
		return nil, fmt.Errorf("endpoint %s not found", id)
	}
	return endpoint, nil
}

// AddProvider implements starmap.Catalog
func (c *Catalog) AddProvider(provider catalogs.Provider) error {
	return c.providers.Add(&provider)
}

// AddAuthor implements starmap.Catalog
func (c *Catalog) AddAuthor(author catalogs.Author) error {
	return c.authors.Add(&author)
}

// AddModel implements starmap.Catalog
func (c *Catalog) AddModel(model catalogs.Model) error {
	return c.models.Add(&model)
}

// AddEndpoint implements starmap.Catalog
func (c *Catalog) AddEndpoint(endpoint catalogs.Endpoint) error {
	return c.endpoints.Add(&endpoint)
}

// UpdateProvider implements starmap.Catalog
func (c *Catalog) UpdateProvider(provider catalogs.Provider) error {
	return c.providers.Set(provider.ID, &provider)
}

// UpdateAuthor implements starmap.Catalog
func (c *Catalog) UpdateAuthor(author catalogs.Author) error {
	return c.authors.Set(author.ID, &author)
}

// UpdateModel implements starmap.Catalog
func (c *Catalog) UpdateModel(model catalogs.Model) error {
	return c.models.Set(model.ID, &model)
}

// UpdateEndpoint implements starmap.Catalog
func (c *Catalog) UpdateEndpoint(endpoint catalogs.Endpoint) error {
	return c.endpoints.Set(endpoint.ID, &endpoint)
}

// RemoveProvider implements starmap.Catalog
func (c *Catalog) RemoveProvider(id catalogs.ProviderID) error {
	return c.providers.Delete(id)
}

// RemoveAuthor implements starmap.Catalog
func (c *Catalog) RemoveAuthor(id catalogs.AuthorID) error {
	return c.authors.Delete(id)
}

// RemoveModel implements starmap.Catalog
func (c *Catalog) RemoveModel(id string) error {
	return c.models.Delete(id)
}

// RemoveEndpoint implements starmap.Catalog
func (c *Catalog) RemoveEndpoint(id string) error {
	return c.endpoints.Delete(id)
}

// loadFromAPI converts models.dev API data to starmap structures
func (c *Catalog) loadFromAPI() error {
	for providerID, modelsDevProvider := range *c.api {
		// Convert provider
		provider, err := modelsDevProvider.ToStarmapProvider()
		if err != nil {
			return fmt.Errorf("converting provider %s: %w", providerID, err)
		}

		// Add provider to catalog
		if err := c.AddProvider(*provider); err != nil {
			return fmt.Errorf("adding provider %s: %w", providerID, err)
		}

		// Add models from this provider
		for modelID, modelsDevModel := range modelsDevProvider.Models {
			model, err := modelsDevModel.ToStarmapModel()
			if err != nil {
				return fmt.Errorf("converting model %s: %w", modelID, err)
			}

			// Add model to catalog
			if err := c.AddModel(*model); err != nil {
				return fmt.Errorf("adding model %s: %w", modelID, err)
			}
		}
	}

	return nil
}
