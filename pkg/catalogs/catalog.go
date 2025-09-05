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

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// MergeStrategy defines how catalogs should be merged.
type MergeStrategy int

const (
	// MergeEnrichEmpty intelligently merges, preserving existing non-empty values.
	MergeEnrichEmpty MergeStrategy = iota
	// MergeReplaceAll completely replaces the target catalog with the source.
	MergeReplaceAll
	// MergeAppendOnly only adds new items, skips existing ones.
	MergeAppendOnly
)

// Note: The Catalog interface is defined in interfaces.go following
// the Interface Segregation Principle. It combines Reader, Writer,
// Merger, and Copier interfaces for complete catalog functionality.

// Compile-time interface checks to ensure proper implementation.
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
// - Custom catalog (readFS is any fs.FS implementation).
type catalog struct {
	options   *catalogOptions
	providers *Providers
	authors   *Authors
	endpoints *Endpoints
}

// New creates a new catalog with the given options
// No options = memory catalog
// WithEmbedded() = embedded catalog with auto-load
// WithFiles(path) = files catalog with auto-load.
func New(opts ...Option) (Catalog, error) {
	options := defaultCatalogOptions()

	for _, opt := range opts {
		opt(options)
	}

	c := &catalog{
		providers: NewProviders(),
		authors:   NewAuthors(),
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

// Providers returns the providers collection.
func (c *catalog) Providers() *Providers {
	return c.providers
}

// Authors returns the authors collection.
func (c *catalog) Authors() *Authors {
	return c.authors
}

// Endpoints returns the endpoints collection.
func (c *catalog) Endpoints() *Endpoints {
	return c.endpoints
}

// Provider returns a provider by ID.
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

// Author returns an author by ID.
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

// Endpoint returns an endpoint by ID.
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

// GetAllModels returns all models from all providers and authors.
func (c *catalog) GetAllModels() []Model {
	models := make([]Model, 0)

	// Collect models from providers
	for _, provider := range c.providers.List() {
		if provider.Models != nil {
			for _, model := range provider.Models {
				models = append(models, model)
			}
		}
	}

	// Collect models from authors (avoiding duplicates)
	modelIDs := make(map[string]bool)
	for _, model := range models {
		modelIDs[model.ID] = true
	}

	for _, author := range c.authors.List() {
		if author.Models != nil {
			for _, model := range author.Models {
				if !modelIDs[model.ID] {
					models = append(models, model)
					modelIDs[model.ID] = true
				}
			}
		}
	}

	return models
}

// FindModel searches for a model by ID across all providers and authors.
func (c *catalog) FindModel(id string) (Model, error) {
	// Search in providers first
	for _, provider := range c.providers.List() {
		if provider.Models != nil {
			if model, exists := provider.Models[id]; exists {
				return model, nil
			}
		}
	}

	// Search in authors
	for _, author := range c.authors.List() {
		if author.Models != nil {
			if model, exists := author.Models[id]; exists {
				return model, nil
			}
		}
	}

	return Model{}, &errors.NotFoundError{
		Resource: "model",
		ID:       id,
	}
}

// SetProvider sets a provider (upsert).
func (c *catalog) SetProvider(provider Provider) error {
	// Deep copy the Models map to prevent shared references
	if provider.Models != nil {
		modelsCopy := make(map[string]Model, len(provider.Models))
		for k, v := range provider.Models {
			modelsCopy[k] = v
		}
		provider.Models = modelsCopy
	}
	return c.providers.Set(provider.ID, &provider)
}

// SetAuthor sets an author (upsert).
func (c *catalog) SetAuthor(author Author) error {
	// Deep copy the Models map to prevent shared references
	if author.Models != nil {
		modelsCopy := make(map[string]Model, len(author.Models))
		for k, v := range author.Models {
			modelsCopy[k] = v
		}
		author.Models = modelsCopy
	}
	return c.authors.Set(author.ID, &author)
}

// SetEndpoint sets an endpoint (upsert).
func (c *catalog) SetEndpoint(endpoint Endpoint) error {
	return c.endpoints.Set(endpoint.ID, &endpoint)
}

// DeleteProvider deletes a provider.
func (c *catalog) DeleteProvider(id ProviderID) error {
	return c.providers.Delete(id)
}

// DeleteAuthor deletes an author.
func (c *catalog) DeleteAuthor(id AuthorID) error {
	return c.authors.Delete(id)
}

// DeleteEndpoint deletes an endpoint.
func (c *catalog) DeleteEndpoint(id string) error {
	return c.endpoints.Delete(id)
}

// ReplaceWith replaces this catalog's contents with another.
func (c *catalog) ReplaceWith(source Reader) error {
	// Clear existing data
	c.providers.Clear()
	c.authors.Clear()
	c.endpoints.Clear()

	// Copy all data from source
	for _, provider := range source.Providers().List() {
		// Deep copy the provider including its Models map
		providerCopy := *provider
		if provider.Models != nil {
			providerCopy.Models = make(map[string]Model, len(provider.Models))
			for k, v := range provider.Models {
				providerCopy.Models[k] = v
			}
		}
		if err := c.SetProvider(providerCopy); err != nil {
			return errors.WrapResource("set", "provider", string(provider.ID), err)
		}
	}

	for _, author := range source.Authors().List() {
		// Deep copy the author including its Models map
		authorCopy := *author
		if author.Models != nil {
			authorCopy.Models = make(map[string]Model, len(author.Models))
			for k, v := range author.Models {
				authorCopy.Models[k] = v
			}
		}
		if err := c.SetAuthor(authorCopy); err != nil {
			return errors.WrapResource("set", "author", string(author.ID), err)
		}
	}

	for _, endpoint := range source.Endpoints().List() {
		if err := c.SetEndpoint(*endpoint); err != nil {
			return errors.WrapResource("set", "endpoint", endpoint.ID, err)
		}
	}

	return nil
}

//
//nolint:gocyclo // Complex merge logic with many fields
// MergeWith merges another catalog into this one.
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
		// Smart merge - merge providers and their models
		for _, sourceProvider := range source.Providers().List() {
			if existingProvider, err := c.Provider(sourceProvider.ID); err == nil {
				// Provider exists, merge models
				if sourceProvider.Models != nil {
					// Create a new map to avoid concurrent modification
					mergedModels := make(map[string]Model)

					// Copy existing models first
					if existingProvider.Models != nil {
						for k, v := range existingProvider.Models {
							mergedModels[k] = v
						}
					}

					// Merge source models
					for modelID, sourceModel := range sourceProvider.Models {
						if existingModel, exists := mergedModels[modelID]; exists {
							// Merge the models
							mergedModels[modelID] = MergeModels(existingModel, sourceModel)
						} else if sourceModel.Pricing != nil || sourceModel.Limits != nil {
							// Add new model with substantial data
							mergedModels[modelID] = sourceModel
						}
					}

					// Update provider with new models map
					existingProvider.Models = mergedModels
				}
				// Update the provider
				if err := c.SetProvider(existingProvider); err != nil {
					return errors.WrapResource("set", "merged provider", string(existingProvider.ID), err)
				}
			} else {
				// New provider
				if err := c.SetProvider(*sourceProvider); err != nil {
					return errors.WrapResource("set", "new provider", string(sourceProvider.ID), err)
				}
			}
		}

		// Merge authors similarly
		for _, sourceAuthor := range source.Authors().List() {
			if existingAuthor, err := c.Author(sourceAuthor.ID); err == nil {
				// Author exists, merge models
				if sourceAuthor.Models != nil {
					// Create a new map to avoid concurrent modification
					mergedModels := make(map[string]Model)

					// Copy existing models first
					if existingAuthor.Models != nil {
						for k, v := range existingAuthor.Models {
							mergedModels[k] = v
						}
					}

					// Merge source models
					for modelID, sourceModel := range sourceAuthor.Models {
						if existingModel, exists := mergedModels[modelID]; exists {
							mergedModels[modelID] = MergeModels(existingModel, sourceModel)
						} else {
							mergedModels[modelID] = sourceModel
						}
					}

					// Update author with new models map
					existingAuthor.Models = mergedModels
				}
				// Update the author
				if err := c.SetAuthor(existingAuthor); err != nil {
					return errors.WrapResource("set", "merged author", string(existingAuthor.ID), err)
				}
			} else {
				// New author
				if err := c.SetAuthor(*sourceAuthor); err != nil {
					return errors.WrapResource("set", "new author", string(sourceAuthor.ID), err)
				}
			}
		}

	case MergeAppendOnly:
		// Only add new providers/authors
		for _, provider := range source.Providers().List() {
			if _, err := c.Provider(provider.ID); err != nil {
				if err := c.SetProvider(*provider); err != nil {
					return errors.WrapResource("append", "provider", string(provider.ID), err)
				}
			}
		}
		for _, author := range source.Authors().List() {
			if _, err := c.Author(author.ID); err != nil {
				if err := c.SetAuthor(*author); err != nil {
					return errors.WrapResource("append", "author", string(author.ID), err)
				}
			}
		}
	}

	return nil
}

// Copy creates a deep copy of the catalog.
func (c *catalog) Copy() (Catalog, error) {
	// Create a new catalog with the same configuration
	newCat := &catalog{
		providers: NewProviders(),
		authors:   NewAuthors(),
		endpoints: NewEndpoints(),
		options:   c.options,
	}

	// Copy all data
	return newCat, newCat.ReplaceWith(c)
}

// MergeStrategy returns the default merge strategy.
func (c *catalog) MergeStrategy() MergeStrategy {
	return c.options.mergeStrategy
}

// SetMergeStrategy sets the default merge strategy.
func (c *catalog) SetMergeStrategy(strategy MergeStrategy) {
	c.options.mergeStrategy = strategy
}

// Load loads the catalog from the configured filesystem.
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
			_ = c.SetProvider(p)
		}
	}

	// Load authors.yaml
	if data, err := fs.ReadFile(c.options.readFS, "authors.yaml"); err == nil {
		var authors []Author
		if err := yaml.Unmarshal(data, &authors); err != nil {
			return errors.WrapParse("yaml", "authors.yaml", err)
		}
		for _, a := range authors {
			_ = c.SetAuthor(a)
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
			// Associate the model with its provider or author based on path
			pathParts := strings.Split(path, "/")

			// Handle providers/[provider-id]/models/[model].yaml or providers/[provider-id]/models/[org]/[model].yaml
			if len(pathParts) >= 4 && pathParts[0] == "providers" && pathParts[2] == "models" {
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
					_ = c.SetProvider(provider)
				}
			}

			// Handle authors/[author-id]/models/[model].yaml or authors/[author-id]/models/[org]/[model].yaml
			if len(pathParts) >= 4 && pathParts[0] == "authors" && pathParts[2] == "models" {
				authorID := AuthorID(pathParts[1])

				// Get the author and add this model to its Models map
				if author, err := c.Author(authorID); err == nil {
					// Initialize Models map if nil
					if author.Models == nil {
						author.Models = make(map[string]Model)
					}
					// Add the model to the author's Models map
					author.Models[model.ID] = model
					// Update the author in the catalog
					_ = c.SetAuthor(author)
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

// Save saves the catalog to the configured filesystem.
func (c *catalog) Save() error {
	if c.options.writePath == "" {
		return &errors.ConfigError{
			Component: "catalog",
			Message:   "no write path configured for saving",
		}
	}

	return c.saveTo(c.options.writePath)
}

// SaveTo saves the catalog to a specific path.
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
// Otherwise uses configured write path.
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

// saveTo saves the catalog to the specified path.
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

	// Save model files to providers/<provider>/models/<model>.yaml or providers/<provider>/models/<org>/<model>.yaml
	for _, provider := range c.providers.List() {
		if len(provider.Models) > 0 {
			// Debug: log provider with models
			logging.Debug().
				Str("provider", string(provider.ID)).
				Int("model_count", len(provider.Models)).
				Msg("Saving provider models")

			for _, model := range provider.Models {
				var modelPath string
				if strings.Contains(model.ID, "/") {
					// Hierarchical ID like "meta-llama/llama-3" -> providers/groq/models/meta-llama/llama-3.yaml
					modelPath = filepath.Join("providers", string(provider.ID), "models", model.ID+".yaml")
				} else {
					// Simple ID like "gpt-4" -> providers/openai/models/gpt-4.yaml
					modelPath = filepath.Join("providers", string(provider.ID), "models", model.ID+".yaml")
				}

				// Use FormatYAML for nicely formatted output with comments
				data := []byte(model.FormatYAML())
				if err := writeFile(modelPath, data); err != nil {
					return errors.WrapIO("write", "model "+model.ID, err)
				}
			}
		}
	}

	// Save author models under authors/<author>/models/<model>.yaml
	for _, author := range c.authors.List() {
		if author.Models != nil {
			for _, model := range author.Models {
				var modelPath string
				if strings.Contains(model.ID, "/") {
					// Hierarchical ID -> authors/meta/models/meta-llama/llama-3.yaml
					modelPath = filepath.Join("authors", string(author.ID), "models", model.ID+".yaml")
				} else {
					// Simple ID -> authors/openai/models/gpt-4.yaml
					modelPath = filepath.Join("authors", string(author.ID), "models", model.ID+".yaml")
				}

				// Use FormatYAML for nicely formatted output with comments
				data := []byte(model.FormatYAML())
				if err := writeFile(modelPath, data); err != nil {
					return errors.WrapIO("write", "model "+model.ID, err)
				}
			}
		}
	}

	return nil
}
