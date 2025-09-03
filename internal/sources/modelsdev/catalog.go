package modelsdev

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Catalog implements starmap.Catalog interface for models.dev data
type Catalog struct {
	api       *ModelsDevAPI
	providers *catalogs.Providers
	authors   *catalogs.Authors
	endpoints *catalogs.Endpoints
}

// NewCatalog creates a new models.dev catalog from parsed API data
func NewCatalog(api *ModelsDevAPI) (*Catalog, error) {
	if api == nil {
		return nil, &errors.ValidationError{
			Field:   "api",
			Message: "cannot be nil",
		}
	}

	catalog := &Catalog{
		api:       api,
		providers: catalogs.NewProviders(),
		authors:   catalogs.NewAuthors(),
		endpoints: catalogs.NewEndpoints(),
	}

	// Convert models.dev data to starmap structures
	if err := catalog.loadFromAPI(); err != nil {
		return nil, errors.WrapResource("load", "API data", "", err)
	}

	return catalog, nil
}

// Providers implements starmap.Catalog
func (c *Catalog) Providers() *catalogs.Providers {
	return c.providers
}

// Authors implements starmap.Catalog
func (c *Catalog) Authors() *catalogs.Authors {
	return c.authors
}

// Endpoints implements starmap.Catalog
func (c *Catalog) Endpoints() *catalogs.Endpoints {
	return c.endpoints
}

// Provider implements starmap.Catalog
func (c *Catalog) Provider(id catalogs.ProviderID) (*catalogs.Provider, error) {
	provider, exists := c.providers.Get(id)
	if !exists {
		return nil, &errors.NotFoundError{
			Resource: "provider",
			ID:       string(id),
		}
	}
	return provider, nil
}

// Author implements starmap.Catalog
func (c *Catalog) Author(id catalogs.AuthorID) (*catalogs.Author, error) {
	author, exists := c.authors.Get(id)
	if !exists {
		return nil, &errors.NotFoundError{
			Resource: "author",
			ID:       string(id),
		}
	}
	return author, nil
}

// Endpoint implements starmap.Catalog
func (c *Catalog) Endpoint(id string) (*catalogs.Endpoint, error) {
	endpoint, exists := c.endpoints.Get(id)
	if !exists {
		return nil, &errors.NotFoundError{
			Resource: "endpoint",
			ID:       id,
		}
	}
	return endpoint, nil
}

// AddProvider implements starmap.Catalog
func (c *Catalog) SetProvider(provider catalogs.Provider) error {
	return c.providers.Set(provider.ID, &provider)
}

// AddAuthor implements starmap.Catalog
func (c *Catalog) SetAuthor(author catalogs.Author) error {
	return c.authors.Set(author.ID, &author)
}

// AddEndpoint implements starmap.Catalog
func (c *Catalog) SetEndpoint(endpoint catalogs.Endpoint) error {
	return c.endpoints.Set(endpoint.ID, &endpoint)
}

// DeleteProvider implements starmap.Catalog
func (c *Catalog) DeleteProvider(id catalogs.ProviderID) error {
	return c.providers.Delete(id)
}

// DeleteAuthor implements starmap.Catalog
func (c *Catalog) DeleteAuthor(id catalogs.AuthorID) error {
	return c.authors.Delete(id)
}

// DeleteEndpoint implements starmap.Catalog
func (c *Catalog) DeleteEndpoint(id string) error {
	return c.endpoints.Delete(id)
}

// loadFromAPI converts models.dev API data to starmap structures
func (c *Catalog) loadFromAPI() error {
	for providerID, modelsDevProvider := range *c.api {
		// Convert provider
		provider, err := modelsDevProvider.ToStarmapProvider()
		if err != nil {
			return errors.WrapResource("convert", "provider", providerID, err)
		}

		// Add provider to catalog
		if err := c.SetProvider(*provider); err != nil {
			return errors.WrapResource("add", "provider", providerID, err)
		}

		// Add models from this provider
		for modelID, modelsDevModel := range modelsDevProvider.Models {
			model, err := modelsDevModel.ToStarmapModel()
			if err != nil {
				return errors.WrapResource("convert", "model", modelID, err)
			}

			// Add model to catalog
			if err := c.Providers().SetModel(catalogs.ProviderID(providerID), *model); err != nil {
				return errors.WrapResource("add", "model", modelID, err)
			}
		}
	}

	return nil
}
