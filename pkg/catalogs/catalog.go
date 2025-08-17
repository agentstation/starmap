package catalogs

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

	// Updates a provider, author, model, or endpoint by the object's id
	UpdateProvider(provider Provider) error
	UpdateAuthor(author Author) error
	UpdateModel(model Model) error
	UpdateEndpoint(endpoint Endpoint) error

	// Deletes a provider, author, model, or endpoint by id
	DeleteProvider(id ProviderID) error
	DeleteAuthor(id AuthorID) error
	DeleteModel(id string) error
	DeleteEndpoint(id string) error

	// Adds a provider, author, model, or endpoint, must be unique by id
	AddProvider(provider Provider) error
	AddAuthor(author Author) error
	AddModel(model Model) error
	AddEndpoint(endpoint Endpoint) error

	// Sync the catalog with another catalog
	Sync(source Catalog) error

	// Return a copy of the catalog
	Copy() (Catalog, error)

	// Save the catalog
	Save() error

	// Load the catalog
	Load() error
}
