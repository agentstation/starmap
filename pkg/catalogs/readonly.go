package catalogs

import (
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/types"
)

// NewCatalog copies source into an immutable canonical catalog.
func NewCatalog(source Reader) (*Catalog, error) {
	if source == nil {
		return nil, &errors.ValidationError{
			Field:   "source",
			Message: "catalog source cannot be nil",
		}
	}
	builder, err := NewBuilderFrom(source)
	if err != nil {
		return nil, errors.WrapResource("create", "immutable catalog", "", err)
	}
	return buildCatalog(builder)
}

var _ Reader = (*Catalog)(nil)

// Catalog is Starmap's immutable canonical catalog. Its state is unexported,
// safe to retain across goroutines, and accessible only through read methods.
type Catalog struct {
	source            Reader
	models            ModelsReader
	providerModels    map[ProviderID]ModelsReader
	definitions       map[ModelDefinitionID]ModelDefinition
	offerings         map[OfferingKey]ProviderOffering
	providerOfferings map[ProviderID][]OfferingKey
}

func buildCatalog(source Reader) (*Catalog, error) {
	providerModels := make(map[ProviderID]ModelsReader)
	for _, provider := range source.Providers().List() {
		models := NewModels()
		for modelID, model := range provider.Models {
			if model == nil {
				continue
			}
			if err := models.Set(modelID, model); err != nil {
				return nil, errors.WrapResource("index", "provider model", string(provider.ID)+"/"+modelID, err)
			}
		}
		reader := modelsReader{source: models}
		providerModels[provider.ID] = reader
		for _, alias := range provider.Aliases {
			providerModels[alias] = reader
		}
	}
	migrated, err := MigrateLegacySchema(source)
	if err != nil {
		return nil, errors.WrapResource("index", "definition and offering catalog", "", err)
	}
	providerOfferings := make(map[ProviderID][]OfferingKey)
	for key := range migrated.Offerings {
		providerOfferings[key.ProviderID] = append(providerOfferings[key.ProviderID], key)
	}
	for providerID, keys := range providerOfferings {
		slices.SortFunc(keys, func(left, right OfferingKey) int {
			return strings.Compare(string(left.ProviderModelID), string(right.ProviderModelID))
		})
		providerOfferings[providerID] = keys
	}
	for _, provider := range source.Providers().List() {
		keys := providerOfferings[provider.ID]
		for _, alias := range provider.Aliases {
			providerOfferings[alias] = keys
		}
	}

	return &Catalog{
		source:            source,
		models:            modelsReader{source: source.Models()},
		providerModels:    providerModels,
		definitions:       migrated.Definitions,
		offerings:         migrated.Offerings,
		providerOfferings: providerOfferings,
	}, nil
}

// Providers returns the immutable catalog's provider collection reader.
func (r *Catalog) Providers() ProvidersReader {
	return providersReader{source: r.source.Providers()}
}

// Authors returns the immutable catalog's author collection reader.
func (r *Catalog) Authors() AuthorsReader {
	return authorsReader{source: r.source.Authors()}
}

// Endpoints returns the immutable catalog's endpoint collection reader.
func (r *Catalog) Endpoints() EndpointsReader {
	return endpointsReader{source: r.source.Endpoints()}
}

// Models returns the immutable catalog's legacy bare-ID model reader.
//
// Deprecated: use Definition, Offering, ProviderOfferings, or LegacyV0.
func (r *Catalog) Models() ModelsReader {
	return r.LegacyV0().Models()
}

// Provenance returns the immutable catalog's provenance reader.
func (r *Catalog) Provenance() ProvenanceReader {
	return provenanceReader{source: r.source.Provenance()}
}

// Provider returns a caller-owned copy of a provider.
func (r *Catalog) Provider(id ProviderID) (Provider, error) { return r.source.Provider(id) }

// Author returns a caller-owned copy of an author.
func (r *Catalog) Author(id AuthorID) (Author, error) { return r.source.Author(id) }

// Endpoint returns a caller-owned copy of an endpoint.
func (r *Catalog) Endpoint(id string) (Endpoint, error) { return r.source.Endpoint(id) }

// ProviderModels returns legacy model records for a provider or alias.
//
// Deprecated: use ProviderOfferings or LegacyV0.
func (r *Catalog) ProviderModels(id ProviderID) (ModelsReader, error) {
	return r.LegacyV0().ProviderModels(id)
}

// ProviderModel returns one legacy provider-scoped model record.
//
// Deprecated: use Offering or LegacyV0.
func (r *Catalog) ProviderModel(providerID ProviderID, modelID string) (Model, error) {
	return r.LegacyV0().ProviderModel(providerID, modelID)
}

// Definition returns one caller-owned canonical model definition.
func (r *Catalog) Definition(id ModelDefinitionID) (ModelDefinition, error) {
	definition, found := r.definitions[id]
	if !found {
		return ModelDefinition{}, &errors.NotFoundError{Resource: "model definition", ID: string(id)}
	}
	return copyModelDefinition(definition), nil
}

// Definitions returns caller-owned canonical definitions in ID order.
func (r *Catalog) Definitions() []ModelDefinition {
	ids := make([]ModelDefinitionID, 0, len(r.definitions))
	for id := range r.definitions {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	definitions := make([]ModelDefinition, 0, len(ids))
	for _, id := range ids {
		definitions = append(definitions, copyModelDefinition(r.definitions[id]))
	}
	return definitions
}

// Offering returns one caller-owned provider-scoped model offering. Provider
// aliases resolve to their canonical provider before key lookup.
func (r *Catalog) Offering(providerID ProviderID, providerModelID ProviderModelID) (ProviderOffering, error) {
	provider, found := r.source.Providers().Resolve(providerID)
	if !found || provider == nil {
		return ProviderOffering{}, &errors.NotFoundError{Resource: "provider", ID: string(providerID)}
	}
	key := OfferingKey{ProviderID: provider.ID, ProviderModelID: providerModelID}
	offering, found := r.offerings[key]
	if !found {
		return ProviderOffering{}, &errors.NotFoundError{
			Resource: "provider offering",
			ID:       string(provider.ID) + "/" + string(providerModelID),
		}
	}
	return copyProviderOffering(offering), nil
}

// ProviderOfferings returns caller-owned offerings in provider-model-ID order.
func (r *Catalog) ProviderOfferings(providerID ProviderID) ([]ProviderOffering, error) {
	keys, found := r.providerOfferings[providerID]
	if !found {
		return nil, &errors.NotFoundError{Resource: "provider", ID: string(providerID)}
	}
	offerings := make([]ProviderOffering, 0, len(keys))
	for _, key := range keys {
		offerings = append(offerings, copyProviderOffering(r.offerings[key]))
	}
	return offerings, nil
}

// FindModel returns the canonical provider-independent model definition.
// Use Offering for provider price, limits, availability, and request behavior;
// use LegacyV0 when migrating code that requires the old flattened Model.
func (r *Catalog) FindModel(id string) (ModelDefinition, error) {
	return r.Definition(ModelDefinitionID(id))
}

type providersReader struct{ source ProvidersReader }

func (r providersReader) Get(id ProviderID) (*Provider, bool) { return r.source.Get(id) }
func (r providersReader) Resolve(id ProviderID) (*Provider, bool) {
	return r.source.Resolve(id)
}
func (r providersReader) Exists(id ProviderID) bool                   { return r.source.Exists(id) }
func (r providersReader) Len() int                                    { return r.source.Len() }
func (r providersReader) List() []Provider                            { return r.source.List() }
func (r providersReader) Map() map[ProviderID]*Provider               { return r.source.Map() }
func (r providersReader) ForEach(fn func(ProviderID, *Provider) bool) { r.source.ForEach(fn) }
func (r providersReader) FormatYAML() string                          { return r.source.FormatYAML() }

type authorsReader struct{ source AuthorsReader }

func (r authorsReader) Get(id AuthorID) (*Author, bool) { return r.source.Get(id) }
func (r authorsReader) Resolve(id AuthorID) (*Author, bool) {
	return r.source.Resolve(id)
}
func (r authorsReader) Exists(id AuthorID) bool                 { return r.source.Exists(id) }
func (r authorsReader) Len() int                                { return r.source.Len() }
func (r authorsReader) List() []Author                          { return r.source.List() }
func (r authorsReader) Map() map[AuthorID]*Author               { return r.source.Map() }
func (r authorsReader) ForEach(fn func(AuthorID, *Author) bool) { r.source.ForEach(fn) }
func (r authorsReader) FormatYAML() string                      { return r.source.FormatYAML() }

type endpointsReader struct{ source EndpointsReader }

func (r endpointsReader) Get(id string) (*Endpoint, bool)         { return r.source.Get(id) }
func (r endpointsReader) Exists(id string) bool                   { return r.source.Exists(id) }
func (r endpointsReader) Len() int                                { return r.source.Len() }
func (r endpointsReader) List() []Endpoint                        { return r.source.List() }
func (r endpointsReader) Map() map[string]*Endpoint               { return r.source.Map() }
func (r endpointsReader) ForEach(fn func(string, *Endpoint) bool) { r.source.ForEach(fn) }

type modelsReader struct{ source ModelsReader }

func (r modelsReader) Get(id string) (*Model, bool)         { return r.source.Get(id) }
func (r modelsReader) Exists(id string) bool                { return r.source.Exists(id) }
func (r modelsReader) Len() int                             { return r.source.Len() }
func (r modelsReader) List() []Model                        { return r.source.List() }
func (r modelsReader) Map() map[string]*Model               { return r.source.Map() }
func (r modelsReader) ForEach(fn func(string, *Model) bool) { r.source.ForEach(fn) }

type provenanceReader struct{ source ProvenanceReader }

func (r provenanceReader) Map() provenance.Map { return r.source.Map() }
func (r provenanceReader) Len() int            { return r.source.Len() }
func (r provenanceReader) FindByField(resourceType types.ResourceType, resourceID, field string) []provenance.Provenance {
	return r.source.FindByField(resourceType, resourceID, field)
}
func (r provenanceReader) FindByResource(resourceType types.ResourceType, resourceID string) map[string][]provenance.Provenance {
	return r.source.FindByResource(resourceType, resourceID)
}
func (r provenanceReader) FormatYAML() string { return r.source.FormatYAML() }
