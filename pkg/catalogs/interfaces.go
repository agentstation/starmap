package catalogs

// Reader provides read-only access to catalog data.
type Reader interface {
	// Lists all providers, authors, and endpoints
	Providers() *Providers
	Authors() *Authors
	Endpoints() *Endpoints

	// Gets a provider, author, or endpoint by id
	Provider(id ProviderID) (Provider, error)
	Author(id AuthorID) (Author, error)
	Endpoint(id string) (Endpoint, error)

	// Helper methods for accessing models through providers/authors
	GetAllModels() []Model
	FindModel(id string) (Model, error)
}

// Writer provides write operations for catalog data.
type Writer interface {
	// Sets a provider, author, or endpoint (upsert semantics)
	SetProvider(provider Provider) error
	SetAuthor(author Author) error
	SetEndpoint(endpoint Endpoint) error

	// Deletes a provider, author, or endpoint by id
	DeleteProvider(id ProviderID) error
	DeleteAuthor(id AuthorID) error
	DeleteEndpoint(id string) error
}

// Merger provides catalog merging capabilities.
type Merger interface {
	// Replace this catalog's contents with another catalog
	// The source only needs to be readable
	ReplaceWith(source Reader) error

	// Merge another catalog into this one
	// The source only needs to be readable
	// Use WithStrategy option to specify merge strategy (defaults to MergeEnrichEmpty)
	MergeWith(source Reader, opts ...MergeOption) error

	// MergeStrategy returns the default merge strategy for this catalog
	MergeStrategy() MergeStrategy

	// SetMergeStrategy sets the default merge strategy for this catalog
	SetMergeStrategy(strategy MergeStrategy)
}

// Copier provides catalog copying capabilities.
type Copier interface {
	// Return a copy of the catalog
	Copy() (Catalog, error)
}

// Catalog is the complete interface combining all catalog capabilities.
// This interface is composed of smaller, focused interfaces following
// the Interface Segregation Principle.
type Catalog interface {
	Reader
	Writer
	Merger
	Copier
}

// ReadOnlyCatalog provides read-only access to a catalog.
type ReadOnlyCatalog interface {
	Reader
	Copier
}

// MutableCatalog provides read-write access without merge capabilities.
type MutableCatalog interface {
	Reader
	Writer
	Copier
}

// MergeableCatalog provides full catalog functionality.
type MergeableCatalog interface {
	Catalog
}
