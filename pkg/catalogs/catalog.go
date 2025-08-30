// Package catalogs provides the core catalog system for managing AI model metadata.
// It offers multiple implementations (embedded, file-based, memory) and supports
// CRUD operations, merging strategies, and persistence.
//
// The catalog system is designed to be thread-safe and extensible, with support
// for providers, models, authors, and endpoints. Each catalog implementation
// can be configured with different storage backends while maintaining a consistent
// interface.
//
// Example usage:
//
//	// Create an embedded catalog (production use)
//	catalog, err := New(WithEmbedded())
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access models
//	models := catalog.Models()
//	for _, model := range models.List() {
//	    fmt.Printf("Model: %s\n", model.ID)
//	}
//
//	// Create a file-based catalog (development use)
//	catalog, err := New(WithFiles("./catalog"))
//	if err != nil {
//	    log.Fatal(err)
//	}
package catalogs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/goccy/go-yaml"
)

// MergeStrategy defines how catalogs should be merged
type MergeStrategy int

const (
	// MergeEnrichEmpty intelligently merges, preserving existing non-empty values
	MergeEnrichEmpty MergeStrategy = iota
	// MergeReplaceAll completely replaces the target catalog with the source
	MergeReplaceAll
	// MergeAppendOnly only adds new items, skips existing ones
	MergeAppendOnly
)

// Note: The Catalog interface is defined in interfaces.go following
// the Interface Segregation Principle. It combines Reader, Writer, 
// Merger, and Copier interfaces for complete catalog functionality.

// Compile-time interface checks to ensure proper implementation
var (
	_ Catalog = (*catalog)(nil)
	_ Reader  = (*catalog)(nil)
	_ Writer  = (*catalog)(nil)
	_ Merger  = (*catalog)(nil)
	_ Copier  = (*catalog)(nil)
)

// catalog is the single concrete implementation of the Catalog interface
// It can work as:
// - Memory catalog (readFS == nil)
// - Embedded catalog (readFS is embed.FS)
// - Files catalog (readFS is os.DirFS)
// - Custom catalog (readFS is any fs.FS implementation)
type catalog struct {
	options   *catalogOptions
	providers *Providers
	authors   *Authors
	models    *Models
	endpoints *Endpoints
}

// New creates a new catalog with the given options
// No options = memory catalog
// WithEmbedded() = embedded catalog with auto-load
// WithFiles(path) = files catalog with auto-load
func New(opts ...Option) (Catalog, error) {
	options := defaultCatalogOptions()

	for _, opt := range opts {
		opt(options)
	}

	c := &catalog{
		providers: NewProviders(),
		authors:   NewAuthors(),
		models:    NewModels(),
		endpoints: NewEndpoints(),
		options:   options,
	}

	// Auto-load if configured and has filesystem
	if c.options.readFS != nil {
		if err := c.Load(); err != nil {
			return nil, errors.WrapResource("load", "catalog", "", err)
		}
	}

	return c, nil
}

// Providers returns the providers collection
func (c *catalog) Providers() *Providers {
	return c.providers
}

// Authors returns the authors collection
func (c *catalog) Authors() *Authors {
	return c.authors
}

// Models returns the models collection
func (c *catalog) Models() *Models {
	return c.models
}

// Endpoints returns the endpoints collection
func (c *catalog) Endpoints() *Endpoints {
	return c.endpoints
}

// Provider returns a provider by ID
func (c *catalog) Provider(id ProviderID) (Provider, error) {
	provider, ok := c.providers.Get(id)
	if !ok {
		return Provider{}, &errors.NotFoundError{
			Resource: "provider",
			ID:       string(id),
		}
	}
	return *provider, nil
}

// Author returns an author by ID
func (c *catalog) Author(id AuthorID) (Author, error) {
	author, ok := c.authors.Get(id)
	if !ok {
		return Author{}, &errors.NotFoundError{
			Resource: "author",
			ID:       string(id),
		}
	}
	return *author, nil
}

// Model returns a model by ID
func (c *catalog) Model(id string) (Model, error) {
	model, ok := c.models.Get(id)
	if !ok {
		return Model{}, &errors.NotFoundError{
			Resource: "model",
			ID:       id,
		}
	}
	return *model, nil
}

// Endpoint returns an endpoint by ID
func (c *catalog) Endpoint(id string) (Endpoint, error) {
	endpoint, ok := c.endpoints.Get(id)
	if !ok {
		return Endpoint{}, &errors.NotFoundError{
			Resource: "endpoint",
			ID:       id,
		}
	}
	return *endpoint, nil
}

// SetProvider sets a provider (upsert)
func (c *catalog) SetProvider(provider Provider) error {
	return c.providers.Set(provider.ID, &provider)
}

// SetAuthor sets an author (upsert)
func (c *catalog) SetAuthor(author Author) error {
	return c.authors.Set(author.ID, &author)
}

// SetModel sets a model (upsert)
func (c *catalog) SetModel(model Model) error {
	return c.models.Set(model.ID, &model)
}

// SetEndpoint sets an endpoint (upsert)
func (c *catalog) SetEndpoint(endpoint Endpoint) error {
	return c.endpoints.Set(endpoint.ID, &endpoint)
}

// DeleteProvider deletes a provider
func (c *catalog) DeleteProvider(id ProviderID) error {
	return c.providers.Delete(id)
}

// DeleteAuthor deletes an author
func (c *catalog) DeleteAuthor(id AuthorID) error {
	return c.authors.Delete(id)
}

// DeleteModel deletes a model
func (c *catalog) DeleteModel(id string) error {
	return c.models.Delete(id)
}

// DeleteEndpoint deletes an endpoint
func (c *catalog) DeleteEndpoint(id string) error {
	return c.endpoints.Delete(id)
}

// ReplaceWith replaces this catalog's contents with another
func (c *catalog) ReplaceWith(source Reader) error {
	// Clear existing data
	c.providers.Clear()
	c.authors.Clear()
	c.models.Clear()
	c.endpoints.Clear()

	// Copy all data from source
	for _, provider := range source.Providers().List() {
		if err := c.SetProvider(*provider); err != nil {
			return errors.WrapResource("set", "provider", string(provider.ID), err)
		}
	}

	for _, author := range source.Authors().List() {
		if err := c.SetAuthor(*author); err != nil {
			return errors.WrapResource("set", "author", string(author.ID), err)
		}
	}

	for _, model := range source.Models().List() {
		if err := c.SetModel(*model); err != nil {
			return errors.WrapResource("set", "model", model.ID, err)
		}
	}

	for _, endpoint := range source.Endpoints().List() {
		if err := c.SetEndpoint(*endpoint); err != nil {
			return errors.WrapResource("set", "endpoint", endpoint.ID, err)
		}
	}

	return nil
}

// MergeWith merges another catalog into this one
func (c *catalog) MergeWith(source Reader, opts ...MergeOption) error {
	// Parse merge options (defaults to MergeEnrichEmpty if not specified)
	mergeOpts := &MergeOptions{Strategy: MergeEnrichEmpty}
	for _, opt := range opts {
		opt(mergeOpts)
	}
	strategy := mergeOpts.Strategy

	switch strategy {
	case MergeReplaceAll:
		return c.ReplaceWith(source)

	case MergeEnrichEmpty:
		// Smart merge - only enrich existing models
		for _, model := range source.Models().List() {
			if existing, err := c.Model(model.ID); err == nil {
				// Model exists, merge it
				merged := MergeModels(existing, *model)
				if err := c.SetModel(merged); err != nil {
					return errors.WrapResource("set", "merged model", model.ID, err)
				}
			} else if model.Pricing != nil || model.Limits != nil {
				// New model with substantial data
				if err := c.SetModel(*model); err != nil {
					return errors.WrapResource("set", "new model", model.ID, err)
				}
			}
		}

	case MergeAppendOnly:
		// Only add new items
		for _, model := range source.Models().List() {
			if _, err := c.Model(model.ID); err != nil {
				if err := c.SetModel(*model); err != nil {
					return errors.WrapResource("append", "model", model.ID, err)
				}
			}
		}
	}

	return nil
}

// Copy creates a deep copy of the catalog
func (c *catalog) Copy() (Catalog, error) {
	// Create a new catalog with the same configuration
	newCat := &catalog{
		providers: NewProviders(),
		authors:   NewAuthors(),
		models:    NewModels(),
		endpoints: NewEndpoints(),
		options:   c.options,
	}

	// Copy all data
	return newCat, newCat.ReplaceWith(c)
}


// MergeStrategy returns the default merge strategy
func (c *catalog) MergeStrategy() MergeStrategy {
	return c.options.mergeStrategy
}

// SetMergeStrategy sets the default merge strategy
func (c *catalog) SetMergeStrategy(strategy MergeStrategy) {
	c.options.mergeStrategy = strategy
}

// Load loads the catalog from the configured filesystem
func (c *catalog) Load() error {
	if c.options.readFS == nil {
		// Memory catalog - nothing to load
		return nil
	}

	// Load providers.yaml
	if data, err := fs.ReadFile(c.options.readFS, "providers.yaml"); err == nil {
		var providers []Provider
		if err := yaml.Unmarshal(data, &providers); err != nil {
			return errors.WrapParse("yaml", "providers.yaml", err)
		}
		for _, p := range providers {
			c.SetProvider(p)
		}
	}

	// Load authors.yaml
	if data, err := fs.ReadFile(c.options.readFS, "authors.yaml"); err == nil {
		var authors []Author
		if err := yaml.Unmarshal(data, &authors); err != nil {
			return errors.WrapParse("yaml", "authors.yaml", err)
		}
		for _, a := range authors {
			c.SetAuthor(a)
		}
	}

	// Load model files from providers/
	err := fs.WalkDir(c.options.readFS, "providers", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// If providers directory doesn't exist, that's okay
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		// Read the file
		data, err := fs.ReadFile(c.options.readFS, path)
		if err != nil {
			return nil // Skip files we can't read
		}

		var model Model
		if err := yaml.Unmarshal(data, &model); err == nil {
			// Add model to the general models collection
			c.SetModel(model)
			
			// Also associate the model with its provider
			// Extract provider ID from path: providers/[provider-id]/[model].yaml
			pathParts := strings.Split(path, "/")
			if len(pathParts) >= 3 && pathParts[0] == "providers" {
				providerID := ProviderID(pathParts[1])
				
				// Get the provider and add this model to its Models map
				if provider, err := c.Provider(providerID); err == nil {
					// Initialize Models map if nil
					if provider.Models == nil {
						provider.Models = make(map[string]Model)
					}
					// Add the model to the provider's Models map
					provider.Models[model.ID] = model
					// Update the provider in the catalog
					c.SetProvider(provider)
				}
			}
		}
		return nil
	})
	if err != nil {
		// Check if it's just that providers directory doesn't exist
		if !os.IsNotExist(err) {
			return errors.WrapIO("walk", "providers directory", err)
		}
	}

	return nil
}

// Save saves the catalog to the configured filesystem
func (c *catalog) Save() error {
	if c.options.writePath == "" {
		return &errors.ConfigError{
			Component: "catalog",
			Message:   "no write path configured for saving",
		}
	}

	return c.saveTo(c.options.writePath)
}

// SaveTo saves the catalog to a specific path
func (c *catalog) SaveTo(path string) error {
	if path == "" {
		return &errors.ValidationError{
			Field:   "path",
			Message: "cannot be empty",
		}
	}
	return c.saveTo(path)
}

// Write saves the catalog to disk
// If a path is provided, saves to that location
// Otherwise uses configured write path
func (c *catalog) Write(paths ...string) error {
	if len(paths) > 1 {
		return &errors.ValidationError{
			Field:   "paths",
			Message: "Write accepts at most one path argument",
		}
	}

	if len(paths) == 1 && paths[0] != "" {
		// Save to specific path
		return c.saveTo(paths[0])
	}

	if c.options.writePath == "" {
		return &errors.ConfigError{
			Component: "catalog",
			Message:   "memory catalog requires explicit path for Write",
		}
	}

	// Use configured write path
	return c.Save()
}

// saveTo saves the catalog to the specified path
func (c *catalog) saveTo(basePath string) error {
	// Helper function to write a file
	writeFile := func(path string, data []byte) error {
		fullPath := filepath.Join(basePath, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
			return errors.WrapIO("create", dir, err)
		}
		return os.WriteFile(fullPath, data, constants.FilePermissions)
	}

	// Save providers.yaml
	providers := c.providers.List()
	if len(providers) > 0 {
		// Clear runtime fields
		cleaned := make([]*Provider, 0, len(providers))
		for _, p := range providers {
			pCopy := *p
			pCopy.Models = nil
			pCopy.EnvVarValues = nil
			cleaned = append(cleaned, &pCopy)
		}

		// Use FormatYAML if available
		yamlData := c.providers.FormatYAML()
		if err := writeFile("providers.yaml", []byte(yamlData)); err != nil {
			return errors.WrapIO("write", "providers.yaml", err)
		}
	}

	// Save authors.yaml
	authors := c.authors.List()
	if len(authors) > 0 {
		cleaned := make([]*Author, 0, len(authors))
		for _, a := range authors {
			aCopy := *a
			aCopy.Models = nil
			cleaned = append(cleaned, &aCopy)
		}

		data, err := yaml.Marshal(cleaned)
		if err != nil {
			return errors.WrapParse("yaml", "authors", err)
		}
		if err := writeFile("authors.yaml", data); err != nil {
			return errors.WrapIO("write", "authors.yaml", err)
		}
	}

	// Save model files to providers/<provider>/<model>.yaml
	for _, model := range c.models.List() {
		// Determine path based on model ID structure
		modelPath := filepath.Join("providers", model.ID+".yaml")
		if strings.Contains(model.ID, "/") {
			// Hierarchical ID like "meta-llama/llama-3"
			modelPath = filepath.Join("providers", model.ID+".yaml")
		}

		data, err := yaml.Marshal(model)
		if err != nil {
			return errors.WrapParse("yaml", "model "+model.ID, err)
		}
		if err := writeFile(modelPath, data); err != nil {
			return errors.WrapIO("write", "model "+model.ID, err)
		}
	}

	return nil
}

