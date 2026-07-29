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
	FindModelField(ProviderID, string, string) []provenance.Provenance
	FindModel(ProviderID, string) map[string][]provenance.Provenance
	FormatYAML() string
}

// Reader provides read-only access to catalog data.
type Reader interface {
	// Lists providers, authors, authored models, and provenance.
	Providers() ProvidersReader
	Authors() AuthorsReader
	AuthoredModels() []AuthoredModel
	Provenance() ProvenanceReader

	// Gets a provider or author by ID.
	Provider(id ProviderID) (Provider, error)
	Author(id AuthorID) (Author, error)
}
