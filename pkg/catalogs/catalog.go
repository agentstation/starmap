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
	"strings"

	"github.com/goccy/go-yaml"

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

// NewLocal creates a new local catalog.
func NewLocal(path string) (Catalog, error) {
	if path != "" {
		return NewFromPath(path)
	}
	return NewEmbedded()
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

// Load loads the catalog from the configured filesystem.
func (cat *catalog) Load() error {
	if cat.options.readFS == nil {
		return nil // Memory catalog - nothing to load
	}

	// Load providers.yaml
	if err := cat.loadProvidersYAML(); err != nil {
		return err
	}

	// Load authors.yaml
	if err := cat.loadAuthorsYAML(); err != nil {
		return err
	}

	// Load model files from providers/
	if err := cat.loadModelFiles(); err != nil {
		return err
	}

	// Post-process: Populate author.Models from model.Authors field
	if err := cat.populateAuthorModels(); err != nil {
		return err
	}

	return nil
}

// loadProvidersYAML loads providers from providers.yaml file.
func (cat *catalog) loadProvidersYAML() error {
	data, err := fs.ReadFile(cat.options.readFS, "providers.yaml")
	if err != nil {
		return nil // File doesn't exist is okay
	}

	var providers []Provider
	if err := yaml.Unmarshal(data, &providers); err != nil {
		return errors.WrapParse("yaml", "providers.yaml", err)
	}

	for _, p := range providers {
		_ = cat.SetProvider(p)
	}
	return nil
}

// loadAuthorsYAML loads authors from authors.yaml file.
func (cat *catalog) loadAuthorsYAML() error {
	data, err := fs.ReadFile(cat.options.readFS, "authors.yaml")
	if err != nil {
		return nil // File doesn't exist is okay
	}

	var authors []Author
	if err := yaml.Unmarshal(data, &authors); err != nil {
		return errors.WrapParse("yaml", "authors.yaml", err)
	}

	for _, a := range authors {
		_ = cat.SetAuthor(a)
	}
	return nil
}

// loadProviderModel loads a model into a provider's Models map.
func (cat *catalog) loadProviderModel(pathParts []string, model *Model) error {
	if len(pathParts) < 4 || pathParts[0] != "providers" || pathParts[2] != "models" {
		return nil // Not a provider model path
	}

	providerID := ProviderID(pathParts[1])
	provider, err := cat.Provider(providerID)
	if err != nil {
		return nil // Provider doesn't exist, skip
	}

	if provider.Models == nil {
		provider.Models = make(map[string]*Model)
	}
	provider.Models[model.ID] = model
	return cat.SetProvider(provider)
}

// loadAuthorModel loads a model into an author's Models map.
func (cat *catalog) loadAuthorModel(pathParts []string, model *Model) error {
	if len(pathParts) < 4 || pathParts[0] != "authors" || pathParts[2] != "models" {
		return nil // Not an author model path
	}

	authorID := AuthorID(pathParts[1])
	author, err := cat.Author(authorID)
	if err != nil {
		return nil // Author doesn't exist, skip
	}

	if author.Models == nil {
		author.Models = make(map[string]*Model)
	}
	author.Models[model.ID] = model
	return cat.SetAuthor(author)
}

// loadModelFile parses and loads a model file.
func (cat *catalog) loadModelFile(path string, data []byte) error {
	var model Model
	if err := yaml.Unmarshal(data, &model); err != nil {
		return nil // Skip invalid YAML
	}

	pathParts := strings.Split(path, "/")

	// Handle providers/[provider-id]/models/[model].yaml
	if err := cat.loadProviderModel(pathParts, &model); err != nil {
		return err
	}

	// Handle authors/[author-id]/models/[model].yaml
	if err := cat.loadAuthorModel(pathParts, &model); err != nil {
		return err
	}

	return nil
}

// loadModelFiles walks the providers directory and loads all model files.
func (cat *catalog) loadModelFiles() error {
	err := fs.WalkDir(cat.options.readFS, "providers", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := fs.ReadFile(cat.options.readFS, path)
		if err != nil {
			return nil // Skip files we can't read
		}

		return cat.loadModelFile(path, data)
	})

	if err != nil && !os.IsNotExist(err) {
		return errors.WrapIO("walk", "providers directory", err)
	}
	return nil
}

// addModelToAuthor adds a model to an author's Models map.
func (cat *catalog) addModelToAuthor(authorID AuthorID, model *Model) error {
	author, err := cat.Author(authorID)
	if err != nil {
		return err
	}

	if author.Models == nil {
		author.Models = make(map[string]*Model)
	}
	author.Models[model.ID] = model
	return cat.SetAuthor(author)
}

// populateAuthorModels populates author.Models from model.Authors field.
// This ensures bidirectional relationship between models and authors.
func (cat *catalog) populateAuthorModels() error {
	providers := cat.providers.List()

	for _, provider := range providers {
		if provider.Models == nil {
			continue
		}

		for _, model := range provider.Models {
			if len(model.Authors) == 0 {
				continue
			}

			for _, modelAuthor := range model.Authors {
				if err := cat.addModelToAuthor(modelAuthor.ID, model); err != nil {
					// Continue on error - not fatal
					continue
				}
			}
		}
	}
	return nil
}
