package catalogs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

// Catalog is a collection of models, providers, authors, and endpoints.
type Catalog interface {
	// Lists all providers, authors, models, and endpoints
	Providers() *Providers
	Authors() *Authors
	Models() *Models
	Endpoints() *Endpoints

	// Gets a provider, author, model, or endpoint by id
	Provider(id ProviderID) (Provider, error)
	Author(id AuthorID) (Author, error)
	Model(id string) (Model, error)
	Endpoint(id string) (Endpoint, error)

	// Sets a provider, author, model, or endpoint (upsert semantics)
	SetProvider(provider Provider) error
	SetAuthor(author Author) error
	SetModel(model Model) error
	SetEndpoint(endpoint Endpoint) error

	// Deletes a provider, author, model, or endpoint by id
	DeleteProvider(id ProviderID) error
	DeleteAuthor(id AuthorID) error
	DeleteModel(id string) error
	DeleteEndpoint(id string) error

	// Replace this catalog's contents with another catalog
	ReplaceWith(source Catalog) error

	// Merge another catalog into this one
	// By default uses the source catalog's suggested merge strategy
	// Can be overridden with WithStrategy option
	MergeWith(source Catalog, opts ...MergeOption) error

	// Return a copy of the catalog
	Copy() (Catalog, error)

	// MergeStrategy returns the default merge strategy for this catalog
	MergeStrategy() MergeStrategy

	// SetMergeStrategy sets the default merge strategy for this catalog
	SetMergeStrategy(strategy MergeStrategy)
}

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
			return nil, fmt.Errorf("loading catalog: %w", err)
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
		return Provider{}, fmt.Errorf("provider with ID %s not found", id)
	}
	return *provider, nil
}

// Author returns an author by ID
func (c *catalog) Author(id AuthorID) (Author, error) {
	author, ok := c.authors.Get(id)
	if !ok {
		return Author{}, fmt.Errorf("author with ID %s not found", id)
	}
	return *author, nil
}

// Model returns a model by ID
func (c *catalog) Model(id string) (Model, error) {
	model, ok := c.models.Get(id)
	if !ok {
		return Model{}, fmt.Errorf("model with ID %s not found", id)
	}
	return *model, nil
}

// Endpoint returns an endpoint by ID
func (c *catalog) Endpoint(id string) (Endpoint, error) {
	endpoint, ok := c.endpoints.Get(id)
	if !ok {
		return Endpoint{}, fmt.Errorf("endpoint with ID %s not found", id)
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
func (c *catalog) ReplaceWith(source Catalog) error {
	// Clear existing data
	c.providers.Clear()
	c.authors.Clear()
	c.models.Clear()
	c.endpoints.Clear()

	// Copy all data from source
	for _, provider := range source.Providers().List() {
		if err := c.SetProvider(*provider); err != nil {
			return fmt.Errorf("setting provider %s: %w", provider.ID, err)
		}
	}

	for _, author := range source.Authors().List() {
		if err := c.SetAuthor(*author); err != nil {
			return fmt.Errorf("setting author %s: %w", author.ID, err)
		}
	}

	for _, model := range source.Models().List() {
		if err := c.SetModel(*model); err != nil {
			return fmt.Errorf("setting model %s: %w", model.ID, err)
		}
	}

	for _, endpoint := range source.Endpoints().List() {
		if err := c.SetEndpoint(*endpoint); err != nil {
			return fmt.Errorf("setting endpoint %s: %w", endpoint.ID, err)
		}
	}

	return nil
}

// MergeWith merges another catalog into this one
func (c *catalog) MergeWith(source Catalog, opts ...MergeOption) error {
	// Get the merge strategy (use source's suggested strategy by default)
	mergeOpts := &MergeOptions{Strategy: source.MergeStrategy()}
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
					return fmt.Errorf("setting merged model %s: %w", model.ID, err)
				}
			} else if model.Pricing != nil || model.Limits != nil {
				// New model with substantial data
				if err := c.SetModel(*model); err != nil {
					return fmt.Errorf("setting new model %s: %w", model.ID, err)
				}
			}
		}

	case MergeAppendOnly:
		// Only add new items
		for _, model := range source.Models().List() {
			if _, err := c.Model(model.ID); err != nil {
				if err := c.SetModel(*model); err != nil {
					return fmt.Errorf("appending model %s: %w", model.ID, err)
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
			return fmt.Errorf("parsing providers.yaml: %w", err)
		}
		for _, p := range providers {
			c.SetProvider(p)
		}
	}

	// Load authors.yaml
	if data, err := fs.ReadFile(c.options.readFS, "authors.yaml"); err == nil {
		var authors []Author
		if err := yaml.Unmarshal(data, &authors); err != nil {
			return fmt.Errorf("parsing authors.yaml: %w", err)
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
			c.SetModel(model)
		}
		return nil
	})
	if err != nil {
		// Check if it's just that providers directory doesn't exist
		if !os.IsNotExist(err) {
			return fmt.Errorf("walking providers directory: %w", err)
		}
	}

	return nil
}

// Save saves the catalog to the configured filesystem
func (c *catalog) Save() error {
	if c.options.writePath == "" {
		return fmt.Errorf("no write path configured for saving")
	}

	return c.saveTo(c.options.writePath)
}

// Write saves the catalog to disk
// If a path is provided, saves to that location
// Otherwise uses configured write path
func (c *catalog) Write(paths ...string) error {
	if len(paths) > 1 {
		return fmt.Errorf("Write accepts at most one path argument")
	}

	if len(paths) == 1 && paths[0] != "" {
		// Save to specific path
		return c.saveTo(paths[0])
	}

	if c.options.writePath == "" {
		return fmt.Errorf("memory catalog requires explicit path for Write")
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
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		return os.WriteFile(fullPath, data, 0644)
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
			return fmt.Errorf("writing providers.yaml: %w", err)
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
			return fmt.Errorf("marshaling authors: %w", err)
		}
		if err := writeFile("authors.yaml", data); err != nil {
			return fmt.Errorf("writing authors.yaml: %w", err)
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
			return fmt.Errorf("marshaling model %s: %w", model.ID, err)
		}
		if err := writeFile(modelPath, data); err != nil {
			return fmt.Errorf("writing model %s: %w", model.ID, err)
		}
	}

	return nil
}

