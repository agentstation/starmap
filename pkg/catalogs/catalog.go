// Package catalogs defines Starmap's authored-model and provider-serving
// construction records plus its immutable canonical read model. Advanced
// producers use Builder to load or assemble those records, then Build validates
// and derives definitions, provider offerings, and author membership into a
// concrete Catalog. Ordinary consumers retain and share that immutable Catalog.
//
// Example usage:
//
//	// Advanced producers construct a draft, then publish an immutable catalog.
//	builder, err := New(WithFS(os.DirFS("./catalog")))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	catalog, err := builder.Build()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access canonical model definitions
//	for _, model := range catalog.Definitions() {
//	    fmt.Printf("Model: %s\n", model.ID)
//	}
//
//	// Create a file-based draft (development use)
//	builder, err = New(WithFiles("./catalog"))
//	if err != nil {
//	    log.Fatal(err)
//	}
package catalogs

import (
	"io/fs"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

// MergeStrategy defines how to merge catalogs.
type MergeStrategy int

const (
	// MergeEnrichEmpty intelligently merges, preserving existing non-empty values.
	MergeEnrichEmpty MergeStrategy = iota
	// MergeReplaceAll completely replaces the target catalog with the source.
	MergeReplaceAll
	// MergeAppendOnly only adds new items, skips existing ones.
	MergeAppendOnly
)

// Catalog is the immutable concrete product type. Builder is the advanced
// mutation boundary used to construct it.

// Compile-time interface check for the open algorithm input boundary.
var _ Reader = (*Builder)(nil)

// Builder is the advanced mutable catalog construction type. Use it for custom
// update callbacks, source or plugin authors, and persistence pipelines. Ordinary
// consumers should use the immutable *Catalog from *starmap.Client.Catalog.
// It can work as:
// - Memory catalog (readFS == nil)
// - Embedded catalog (readFS is embed.FS)
// - Files catalog (readFS is os.DirFS)
// - Custom catalog (readFS is any fs.FS implementation).
type Builder struct {
	config         *options
	providers      *Providers
	authors        *Authors
	authoredModels *authoredModelStore
	provenance     *Provenance
	loadReport     LoadReport
}

// New creates a new builder with the given options.
// WithFS(fsys) and WithPath(path) load the configured files automatically.
func New(opt Option, opts ...Option) (*Builder, error) {
	cat := &Builder{
		providers:      NewProviders(),
		authors:        NewAuthors(),
		authoredModels: newAuthoredModelStore(),
		provenance:     NewProvenance(),
		config:         defaults().apply(append([]Option{opt}, opts...)...),
	}

	// Auto-load if configured and has filesystem
	if cat.config.readFilesystem() != nil {
		if err := cat.Load(); err != nil {
			return nil, errors.WrapResource("load", "catalog", "", err)
		}
	}

	return cat, nil
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
func NewFromPath(path string) (*Builder, error) {
	// Verify path exists
	if _, err := os.Stat(path); err != nil {
		return nil, errors.WrapIO("stat", path, err)
	}
	return New(WithPath(path))
}

// NewEmpty creates an in-memory empty catalog.
// This is useful for testing or temporary catalogs that do not
// need persistence.
//
// Example:
//
//	catalog := NewEmpty()
//	provider := Provider{ID: "openai", Models: map[string]Model{}}
//	catalog.SetProvider(provider)
func NewEmpty() *Builder {
	return &Builder{
		providers:      NewProviders(),
		authors:        NewAuthors(),
		authoredModels: newAuthoredModelStore(),
		provenance:     NewProvenance(),
		config:         defaults(),
	}
}

// NewBuilderFrom copies source into a new independent builder.
func NewBuilderFrom(source Reader) (*Builder, error) {
	if source == nil {
		return nil, &errors.ValidationError{
			Field:   "source",
			Message: "builder source cannot be nil",
		}
	}
	builder := NewEmpty()
	if err := builder.ReplaceWith(source); err != nil {
		return nil, errors.WrapResource("create", "catalog builder", "", err)
	}
	return builder, nil
}

// NewFromFS creates a catalog from a custom filesystem implementation.
// This allows for advanced use cases like virtual filesystems or
// custom storage backends.
//
// Example:
//
//	var myFS embed.FS
//	catalog, err := NewFromFS(myFS, "catalog")
func NewFromFS(fsys fs.FS, root string) (*Builder, error) {
	subFS, err := fs.Sub(fsys, root)
	if err != nil {
		return nil, errors.WrapResource("create", "sub filesystem", root, err)
	}
	return New(WithFS(subFS))
}

// Providers returns the providers collection.
func (cat *Builder) Providers() ProvidersReader {
	return cat.providers
}

// Authors returns the authors collection.
func (cat *Builder) Authors() AuthorsReader {
	return cat.authors
}

// AuthoredModels returns caller-owned provider-independent construction
// records in canonical author/slug order.
func (cat *Builder) AuthoredModels() []AuthoredModel {
	return cat.authoredModels.list()
}

// Provenance returns the provenance collection.
func (cat *Builder) Provenance() ProvenanceReader {
	return cat.provenance
}

// Provider returns a provider by ID or alias.
// Silently resolves aliases to canonical provider IDs.
func (cat *Builder) Provider(id ProviderID) (Provider, error) {
	provider, ok := cat.providers.Resolve(id)
	if !ok {
		return Provider{}, &errors.NotFoundError{
			Resource: "provider",
			ID:       string(id),
		}
	}
	return DeepCopyProvider(*provider), nil
}

// Author returns an author by ID or alias.
// Silently resolves aliases to canonical author IDs.
func (cat *Builder) Author(id AuthorID) (Author, error) {
	author, ok := cat.authors.Resolve(id)
	if !ok {
		return Author{}, &errors.NotFoundError{
			Resource: "author",
			ID:       string(id),
		}
	}
	return DeepCopyAuthor(*author), nil
}

// ProviderModels returns the models served by a provider or one of its aliases.
func (cat *Builder) ProviderModels(id ProviderID) (ModelsReader, error) {
	provider, ok := cat.providers.Resolve(id)
	if !ok {
		return nil, &errors.NotFoundError{Resource: "provider", ID: string(id)}
	}
	models := NewModels()
	for modelID, model := range provider.Models {
		if model == nil {
			continue
		}
		if err := models.Set(modelID, model); err != nil {
			return nil, errors.WrapResource("index", "provider model", string(provider.ID)+"/"+modelID, err)
		}
	}
	return models, nil
}

// ProviderModel returns one provider-specific model offering without flattening
// equal model IDs from other providers.
func (cat *Builder) ProviderModel(providerID ProviderID, modelID string) (Model, error) {
	provider, ok := cat.providers.Resolve(providerID)
	if !ok {
		return Model{}, &errors.NotFoundError{Resource: "provider", ID: string(providerID)}
	}
	model, ok := provider.Models[modelID]
	if !ok || model == nil {
		return Model{}, &errors.NotFoundError{
			Resource: "provider model",
			ID:       string(provider.ID) + "/" + modelID,
		}
	}
	return DeepCopyModel(*model), nil
}

// SetProvider sets a provider (upsert).
func (cat *Builder) SetProvider(provider Provider) error {
	// Deep copy to prevent shared references
	providerCopy := DeepCopyProvider(provider)
	return cat.providers.Set(providerCopy.ID, &providerCopy)
}

// SetAuthor sets an author (upsert).
func (cat *Builder) SetAuthor(author Author) error {
	// Deep copy to prevent shared references
	authorCopy := DeepCopyAuthor(author)
	return cat.authors.Set(authorCopy.ID, &authorCopy)
}

// SetProviderModel sets a model on a provider atomically.
func (cat *Builder) SetProviderModel(providerID ProviderID, model Model) error {
	if err := validateProviderModelPathID(model.ID); err != nil {
		return errors.WrapResource("validate", "provider model", string(providerID)+"/"+model.ID, err)
	}
	if err := validateModelGeneration(model.Generation); err != nil {
		return errors.WrapResource("validate", "provider model", string(providerID)+"/"+model.ID, err)
	}
	if model.ModelRef != "" {
		if _, _, err := ParseModelDefinitionID(model.ModelRef); err != nil {
			return errors.WrapResource("validate", "provider model reference", string(model.ModelRef), err)
		}
	}
	return cat.providers.SetModel(providerID, model)
}

// SetAuthorModel sets one provider-independent model on its owning author.
func (cat *Builder) SetAuthorModel(authorID AuthorID, model Model) error {
	author, err := cat.Author(authorID)
	if err != nil {
		return err
	}
	for index, modelAuthor := range model.Authors {
		canonical, resolveErr := cat.Author(modelAuthor.ID)
		if resolveErr != nil {
			return errors.WrapResource(
				"resolve",
				"authored model author",
				string(modelAuthor.ID),
				resolveErr,
			)
		}
		model.Authors[index] = Author{ID: canonical.ID, Name: canonical.Name}
	}
	if err := validateAuthoredModel(author.ID, model); err != nil {
		return errors.WrapResource(
			"validate",
			"authored model",
			string(author.ID)+"/"+model.ID,
			err,
		)
	}
	return cat.authoredModels.set(AuthoredModel{AuthorID: author.ID, Model: model})
}

// SetProvenance replaces catalog provenance.
func (cat *Builder) SetProvenance(value provenance.Map) {
	cat.provenance.Set(value)
}

// MergeProvenance appends catalog provenance.
func (cat *Builder) MergeProvenance(value provenance.Map) {
	cat.provenance.Merge(value)
}

// ClearProvenance removes catalog provenance.
func (cat *Builder) ClearProvenance() {
	cat.provenance.Clear()
}

// DeleteProvider deletes a provider.
func (cat *Builder) DeleteProvider(id ProviderID) error {
	return cat.providers.Delete(id)
}

// DeleteAuthor deletes an author.
func (cat *Builder) DeleteAuthor(id AuthorID) error {
	author, err := cat.Author(id)
	if err != nil {
		return err
	}
	for _, record := range cat.authoredModels.list() {
		if record.AuthorID == author.ID {
			return &errors.ConflictError{
				Resource: "author",
				Actual:   string(author.ID),
				Message:  "cannot be deleted while it owns authored models",
			}
		}
		for _, modelAuthor := range record.Model.Authors {
			canonical, resolveErr := cat.Author(modelAuthor.ID)
			if resolveErr != nil {
				return errors.WrapResource(
					"resolve",
					"authored model author",
					string(modelAuthor.ID),
					resolveErr,
				)
			}
			if canonical.ID == author.ID {
				return &errors.ConflictError{
					Resource: "author",
					Actual:   string(author.ID),
					Message:  "cannot be deleted while it is credited by authored models",
				}
			}
		}
	}
	return cat.authors.Delete(author.ID)
}

// DeleteProviderModel deletes a model from a provider atomically.
func (cat *Builder) DeleteProviderModel(providerID ProviderID, modelID string) error {
	return cat.providers.DeleteModel(providerID, modelID)
}

// DeleteAuthorModel deletes one provider-independent model from an author.
func (cat *Builder) DeleteAuthorModel(authorID AuthorID, slug string) error {
	author, err := cat.Author(authorID)
	if err != nil {
		return err
	}
	return cat.authoredModels.delete(AuthoredModelID(author.ID, slug))
}

// ReplaceWith replaces this catalog's contents with another.
func (cat *Builder) ReplaceWith(source Reader) error {
	// Clear existing data
	cat.providers.Clear()
	cat.authors.Clear()
	cat.authoredModels.clear()
	cat.provenance.Clear()

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
	for _, record := range source.AuthoredModels() {
		if err := cat.SetAuthorModel(record.AuthorID, record.Model); err != nil {
			return errors.WrapResource("set", "authored model", string(record.ID()), err)
		}
	}

	// Copy provenance
	cat.provenance.Set(source.Provenance().Map())

	return nil
}

// MergeWith merges another catalog into this one.
func (cat *Builder) MergeWith(source Reader, opts ...MergeOption) error {
	mergeOpts := &MergeOptions{Strategy: MergeEnrichEmpty}
	for _, opt := range opts {
		opt(mergeOpts)
	}

	switch mergeOpts.Strategy {
	case MergeReplaceAll:
		return cat.ReplaceWith(source)
	case MergeEnrichEmpty:
		return cat.mergeEnrichEmpty(source)
	case MergeAppendOnly:
		return cat.mergeAppendOnly(source)
	}

	return nil
}

func (cat *Builder) mergeEnrichEmpty(source Reader) error {
	for _, provider := range source.Providers().List() {
		if err := cat.mergeEnrichedProvider(provider); err != nil {
			return err
		}
	}
	for _, sourceAuthor := range source.Authors().List() {
		existingAuthor, err := cat.Author(sourceAuthor.ID)
		if err != nil {
			if err := cat.SetAuthor(sourceAuthor); err != nil {
				return errors.WrapResource("set", "new author", string(sourceAuthor.ID), err)
			}
			continue
		}
		if err := cat.SetAuthor(existingAuthor); err != nil {
			return errors.WrapResource("set", "merged author", string(existingAuthor.ID), err)
		}
	}
	cat.provenance.Merge(source.Provenance().Map())
	return nil
}

func (cat *Builder) mergeEnrichedProvider(source Provider) error {
	existing, err := cat.Provider(source.ID)
	if err != nil {
		if err := cat.SetProvider(source); err != nil {
			return errors.WrapResource("set", "new provider", string(source.ID), err)
		}
		return nil
	}
	if source.Models != nil {
		existing.Models = mergeEnrichedModels(existing.Models, source.Models)
	}
	if err := cat.SetProvider(existing); err != nil {
		return errors.WrapResource("set", "merged provider", string(existing.ID), err)
	}
	return nil
}

func mergeEnrichedModels(existing, source map[string]*Model) map[string]*Model {
	merged := make(map[string]Model, len(existing)+len(source))
	for modelID, model := range existing {
		merged[modelID] = *model
	}
	for modelID, sourceModel := range source {
		if existingModel, found := merged[modelID]; found {
			merged[modelID] = MergeModels(existingModel, *sourceModel)
		} else if sourceModel.Pricing != nil || sourceModel.Limits != nil {
			merged[modelID] = *sourceModel
		}
	}
	result := make(map[string]*Model, len(merged))
	for modelID, model := range merged {
		modelCopy := model
		result[modelID] = &modelCopy
	}
	return result
}

func (cat *Builder) mergeAppendOnly(source Reader) error {
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
	cat.provenance.Merge(source.Provenance().Map())
	return nil
}

// Copy creates a deep copy of the catalog.
func (cat *Builder) Copy() (*Builder, error) {
	// Create a new catalog with the same configuration
	NewCat := &Builder{
		providers:      NewProviders(),
		authors:        NewAuthors(),
		authoredModels: newAuthoredModelStore(),
		provenance:     NewProvenance(),
		config:         cat.config.copy(),
		loadReport:     cat.LoadReport(),
	}

	// Copy all data
	return NewCat, NewCat.ReplaceWith(cat)
}

// Build publishes an immutable deep copy of the builder's current state.
func (cat *Builder) Build() (*Catalog, error) {
	return NewCatalog(cat)
}

// MergeStrategy returns the default merge strategy.
func (cat *Builder) MergeStrategy() MergeStrategy {
	return cat.config.strategy()
}

// SetMergeStrategy sets the default merge strategy.
func (cat *Builder) SetMergeStrategy(strategy MergeStrategy) {
	cat.config.setStrategy(strategy)
}
