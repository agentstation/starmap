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

	"github.com/agentstation/starmap/pkg/errors"
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
	_ Catalog     = (*catalog)(nil)
	_ Reader      = (*catalog)(nil)
	_ Writer      = (*catalog)(nil)
	_ Merger      = (*catalog)(nil)
	_ Copier      = (*catalog)(nil)
	_ Persistence = (*catalog)(nil)
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
// WithEmbedded() = embedded catalog with auto-load
// WithFiles(path) = files catalog with auto-load.
func New(opt Option, opts ...Option) (Catalog, error) {
	cat := &catalog{
		providers: NewProviders(),
		authors:   NewAuthors(),
		endpoints: NewEndpoints(),
		options:   catalogDefaults().apply(append([]Option{opt}, opts...)...),
	}

	// Auto-load if configured and has filesystem
	if cat.options.readFS != nil {
		if err := cat.Load(); err != nil {
			return nil, errors.WrapResource("load", "catalog", "", err)
		}
	}

	return cat, nil
}

// NewEmbedded creates a catalog backed by embedded files.
// This is the recommended catalog for production use as it includes
// all model data compiled into the binary.
func NewEmbedded() (Catalog, error) {
	return New(WithEmbedded())
}

// NewFromPath creates a catalog backed by files on disk.
// This is useful for development when you want to edit catalog files
// without recompiling the binary.
//
// Example:
//
//	catalog, err := NewFromPath("./internal/embedded/catalog")
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewFromPath(path string) (Catalog, error) {
	// Verify path exists
	if _, err := os.Stat(path); err != nil {
		return nil, errors.WrapIO("stat", path, err)
	}
	return New(WithPath(path))
}

// NewLocal creates a catalog by merging embedded catalog with local file.
// - Always loads embedded catalog (latest provider configs)
// - Merges with file catalog if path provided and file exists
// - Returns embedded-only if file doesn't exist or path is empty
//
// This ensures that the catalog always has the latest provider configurations
// from the embedded catalog, while preserving saved model data from files.
func NewLocal(path string) (Catalog, error) {
	// Always start with embedded (latest provider configs)
	embedded, err := NewEmbedded()
	if err != nil {
		return nil, errors.WrapResource("load", "embedded catalog", "", err)
	}

	// If no path specified, return embedded only
	if path == "" {
		return embedded, nil
	}

	// Try to load file catalog
	fileCatalog, err := NewFromPath(path)
	if err != nil {
		// File doesn't exist or is corrupt - that's OK on first run
		// Return embedded only
		return embedded, nil
	}

	// Merge file into embedded (preserves embedded configs, adds file models)
	if merger, ok := embedded.(Merger); ok {
		if err := merger.MergeWith(fileCatalog); err != nil {
			return nil, errors.WrapResource("merge", "catalogs", "", err)
		}
	}

	return embedded, nil
}

// NewEmpty creates an in-memory empty catalog.
// This is useful for testing or temporary catalogs that don't
// need persistence.
//
// Example:
//
//	catalog := NewEmpty()
//	provider := Provider{ID: "openai", Models: map[string]Model{}}
//	catalog.SetProvider(provider)
func NewEmpty() Catalog {
	return &catalog{
		providers: NewProviders(),
		authors:   NewAuthors(),
		endpoints: NewEndpoints(),
		options:   catalogDefaults(),
	}
}

// NewFromFS creates a catalog from a custom filesystem implementation.
// This allows for advanced use cases like virtual filesystems or
// custom storage backends.
//
// Example:
//
//	var myFS embed.FS
//	catalog, err := NewFromFS(myFS, "catalog")
func NewFromFS(fsys fs.FS, root string) (Catalog, error) {
	subFS, err := fs.Sub(fsys, root)
	if err != nil {
		return nil, errors.WrapResource("create", "sub filesystem", root, err)
	}
	return New(WithFS(subFS))
}

// Providers returns the providers collection.
func (cat *catalog) Providers() *Providers {
	return cat.providers
}

// Authors returns the authors collection.
func (cat *catalog) Authors() *Authors {
	return cat.authors
}

// Endpoints returns the endpoints collection.
func (cat *catalog) Endpoints() *Endpoints {
	return cat.endpoints
}

// Provider returns a provider by ID.
func (cat *catalog) Provider(id ProviderID) (Provider, error) {
	provider, ok := cat.providers.Get(id)
	if !ok {
		return Provider{}, &errors.NotFoundError{
			Resource: "provider",
			ID:       string(id),
		}
	}
	return *provider, nil
}

// Author returns an author by ID.
func (cat *catalog) Author(id AuthorID) (Author, error) {
	author, ok := cat.authors.Get(id)
	if !ok {
		return Author{}, &errors.NotFoundError{
			Resource: "author",
			ID:       string(id),
		}
	}
	return *author, nil
}

// Endpoint returns an endpoint by ID.
func (cat *catalog) Endpoint(id string) (Endpoint, error) {
	endpoint, ok := cat.endpoints.Get(id)
	if !ok {
		return Endpoint{}, &errors.NotFoundError{
			Resource: "endpoint",
			ID:       id,
		}
	}
	return *endpoint, nil
}

// Models returns all models from all providers and authors.
func (cat *catalog) Models() *Models {
	models := NewModels()

	// Collect models from providers
	for _, provider := range cat.providers.List() {
		if provider.Models != nil {
			for _, model := range provider.Models {
				_ = models.Set(model.ID, model) // Ignore error - models from providers are already validated
			}
		}
	}

	// Collect models from authors (avoiding duplicates)
	modelIDs := make(map[string]bool)
	for _, model := range models.List() {
		modelIDs[model.ID] = true
	}
	for _, author := range cat.authors.List() {
		if author.Models != nil {
			for _, model := range author.Models {
				if !modelIDs[model.ID] {
					_ = models.Set(model.ID, model) // Ignore error - models from authors are already validated
					modelIDs[model.ID] = true
				}
			}
		}
	}

	return models
}

// FindModel searches for a model by ID.
func (cat *catalog) FindModel(id string) (Model, error) {
	// Check each model in the catalog for the given Model ID.
	for _, model := range cat.Models().List() {
		if model.ID == id {
			return model, nil // Return the model if found.
		}
	}

	// If the model is not found, return a not found error.
	return Model{}, &errors.NotFoundError{
		Resource: "model",
		ID:       id,
	}
}

// SetProvider sets a provider (upsert).
func (cat *catalog) SetProvider(provider Provider) error {
	// Deep copy to prevent shared references
	providerCopy := DeepCopyProvider(provider)
	return cat.providers.Set(providerCopy.ID, &providerCopy)
}

// SetAuthor sets an author (upsert).
func (cat *catalog) SetAuthor(author Author) error {
	// Deep copy to prevent shared references
	authorCopy := DeepCopyAuthor(author)
	return cat.authors.Set(authorCopy.ID, &authorCopy)
}

// SetEndpoint sets an endpoint (upsert).
func (cat *catalog) SetEndpoint(endpoint Endpoint) error {
	return cat.endpoints.Set(endpoint.ID, &endpoint)
}

// DeleteProvider deletes a provider.
func (cat *catalog) DeleteProvider(id ProviderID) error {
	return cat.providers.Delete(id)
}

// DeleteAuthor deletes an author.
func (cat *catalog) DeleteAuthor(id AuthorID) error {
	return cat.authors.Delete(id)
}

// DeleteEndpoint deletes an endpoint.
func (cat *catalog) DeleteEndpoint(id string) error {
	return cat.endpoints.Delete(id)
}

// ReplaceWith replaces this catalog's contents with another.
func (cat *catalog) ReplaceWith(source Reader) error {
	// Clear existing data
	cat.providers.Clear()
	cat.authors.Clear()
	cat.endpoints.Clear()

	// Copy all data from source
	for _, provider := range source.Providers().List() {
		// Deep copy the provider including its Models map
		providerCopy := DeepCopyProvider(provider)
		if err := cat.SetProvider(providerCopy); err != nil {
			return errors.WrapResource("set", "provider", string(provider.ID), err)
		}
	}

	for _, author := range source.Authors().List() {
		// Deep copy the author including its Models map
		authorCopy := DeepCopyAuthor(author)
		if err := cat.SetAuthor(authorCopy); err != nil {
			return errors.WrapResource("set", "author", string(author.ID), err)
		}
	}

	for _, endpoint := range source.Endpoints().List() {
		if err := cat.SetEndpoint(endpoint); err != nil {
			return errors.WrapResource("set", "endpoint", endpoint.ID, err)
		}
	}

	return nil
}

// MergeWith merges another catalog into this one.
//
//nolint:gocyclo // Complex merge logic with many fields
func (cat *catalog) MergeWith(source Reader, opts ...MergeOption) error {
	// Parse merge options (defaults to MergeEnrichEmpty if not specified)
	mergeOpts := &MergeOptions{Strategy: MergeEnrichEmpty}
	for _, opt := range opts {
		opt(mergeOpts)
	}
	strategy := mergeOpts.Strategy

	switch strategy {
	case MergeReplaceAll:
		return cat.ReplaceWith(source)

	case MergeEnrichEmpty:
		// Smart merge - merge providers and their models
		for _, sourceProvider := range source.Providers().List() {
			if existingProvider, err := cat.Provider(sourceProvider.ID); err == nil {
				// Provider exists, merge models
				if sourceProvider.Models != nil {
					// Create a new map to avoid concurrent modification
					mergedModels := make(map[string]Model)

					// Copy existing models first
					if existingProvider.Models != nil {
						for k, v := range existingProvider.Models {
							mergedModels[k] = *v
						}
					}

					// Merge source models
					for modelID, sourceModel := range sourceProvider.Models {
						if existingModel, exists := mergedModels[modelID]; exists {
							// Merge the models
							mergedModels[modelID] = MergeModels(existingModel, *sourceModel)
						} else if sourceModel.Pricing != nil || sourceModel.Limits != nil {
							// Add new model with substantial data
							mergedModels[modelID] = *sourceModel
						}
					}

					mergedModelsCopy := make(map[string]*Model, len(mergedModels))
					for k, v := range mergedModels {
						mergedModelsCopy[k] = &v
					}
					// Update provider with new models map
					existingProvider.Models = mergedModelsCopy
				}
				// Update the provider
				if err := cat.SetProvider(existingProvider); err != nil {
					return errors.WrapResource("set", "merged provider", string(existingProvider.ID), err)
				}
			} else {
				// New provider
				if err := cat.SetProvider(sourceProvider); err != nil {
					return errors.WrapResource("set", "new provider", string(sourceProvider.ID), err)
				}
			}
		}

		// Merge authors similarly
		for _, sourceAuthor := range source.Authors().List() {
			if existingAuthor, err := cat.Author(sourceAuthor.ID); err == nil {
				// Author exists, merge models
				if sourceAuthor.Models != nil {
					// Create a new map to avoid concurrent modification
					mergedModels := make(map[string]*Model)

					// Copy existing models first
					if existingAuthor.Models != nil {
						for k, v := range existingAuthor.Models {
							mergedModels[k] = v
						}
					}

					// Merge source models
					for modelID, sourceModel := range sourceAuthor.Models {
						if existingModel, exists := mergedModels[modelID]; exists {
							mergedModel := MergeModels(*existingModel, *sourceModel)
							mergedModels[modelID] = &mergedModel
						} else {
							mergedModels[modelID] = sourceModel
						}
					}

					// Update author with new models map
					existingAuthor.Models = mergedModels
				}
				// Update the author
				if err := cat.SetAuthor(existingAuthor); err != nil {
					return errors.WrapResource("set", "merged author", string(existingAuthor.ID), err)
				}
			} else {
				// New author
				if err := cat.SetAuthor(sourceAuthor); err != nil {
					return errors.WrapResource("set", "new author", string(sourceAuthor.ID), err)
				}
			}
		}

	case MergeAppendOnly:
		// Only add new providers/authors
		for _, provider := range source.Providers().List() {
			if _, err := cat.Provider(provider.ID); err != nil {
				if err := cat.SetProvider(provider); err != nil {
					return errors.WrapResource("append", "provider", string(provider.ID), err)
				}
			}
		}
		for _, author := range source.Authors().List() {
			if _, err := cat.Author(author.ID); err != nil {
				if err := cat.SetAuthor(author); err != nil {
					return errors.WrapResource("append", "author", string(author.ID), err)
				}
			}
		}
	}

	return nil
}

// Copy creates a deep copy of the catalog.
func (cat *catalog) Copy() (Catalog, error) {
	// Create a new catalog with the same configuration
	NewCat := &catalog{
		providers: NewProviders(),
		authors:   NewAuthors(),
		endpoints: NewEndpoints(),
		options:   cat.options,
	}

	// Copy all data
	return NewCat, NewCat.ReplaceWith(cat)
}

// MergeStrategy returns the default merge strategy.
func (cat *catalog) MergeStrategy() MergeStrategy {
	return cat.options.mergeStrategy
}

// SetMergeStrategy sets the default merge strategy.
func (cat *catalog) SetMergeStrategy(strategy MergeStrategy) {
	cat.options.mergeStrategy = strategy
}
