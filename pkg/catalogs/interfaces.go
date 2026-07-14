package catalogs

import (
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/provenance"
)

// ProvidersReader exposes provider collection reads without mutation methods.
type ProvidersReader interface {
	Get(ProviderID) (*Provider, bool)
	Resolve(ProviderID) (*Provider, bool)
	Exists(ProviderID) bool
	Len() int
	List() []Provider
	Map() map[ProviderID]*Provider
	ForEach(func(ProviderID, *Provider) bool)
	FormatYAML() string
}

// AuthorsReader exposes author collection reads without mutation methods.
type AuthorsReader interface {
	Get(AuthorID) (*Author, bool)
	Resolve(AuthorID) (*Author, bool)
	Exists(AuthorID) bool
	Len() int
	List() []Author
	Map() map[AuthorID]*Author
	ForEach(func(AuthorID, *Author) bool)
	FormatYAML() string
}

// EndpointsReader exposes endpoint collection reads without mutation methods.
type EndpointsReader interface {
	Get(string) (*Endpoint, bool)
	Exists(string) bool
	Len() int
	List() []Endpoint
	Map() map[string]*Endpoint
	ForEach(func(string, *Endpoint) bool)
}

// ModelsReader exposes model collection reads without mutation methods.
type ModelsReader interface {
	Get(string) (*Model, bool)
	Exists(string) bool
	Len() int
	List() []Model
	Map() map[string]*Model
	ForEach(func(string, *Model) bool)
}

// ProvenanceReader exposes provenance reads without mutation methods.
type ProvenanceReader interface {
	Map() provenance.Map
	Len() int
	FindByField(catalogmeta.ResourceType, string, string) []provenance.Provenance
	FindByResource(catalogmeta.ResourceType, string) map[string][]provenance.Provenance
	FormatYAML() string
}

// Reader provides read-only access to the canonical catalog schema.
type Reader interface {
	// Lists all providers, authors, and endpoints
	Providers() ProvidersReader
	Authors() AuthorsReader
	Endpoints() EndpointsReader
	Provenance() ProvenanceReader

	// Gets a provider, author, or endpoint by id
	Provider(id ProviderID) (Provider, error)
	Author(id AuthorID) (Author, error)
	Endpoint(id string) (Endpoint, error)
	Definitions() []ModelDefinition
	Offerings() []ProviderOffering
}

// modelSourceReader extends the canonical reader with mutable-source model
// records used only while importing provider and author data into a Builder.
// Published and immutable catalogs intentionally do not implement this
// ingestion boundary.
type modelSourceReader interface {
	Reader
	Models() ModelsReader
	ProviderModels(id ProviderID) (ModelsReader, error)
	ProviderModel(providerID ProviderID, modelID string) (Model, error)
}
