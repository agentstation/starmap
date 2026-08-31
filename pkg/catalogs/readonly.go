package catalogs

import (
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
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

// NewObservationCatalog copies source records into an immutable source
// observation without deriving consumer definitions or offerings. It exists
// for acquisition boundaries that must preserve provider records before
// reconciliation resolves every ModelRef. Final publication must use
// NewCatalog or Builder.Build, which fail closed on unresolved references.
func NewObservationCatalog(source Reader) (*Catalog, error) {
	if source == nil {
		return nil, &errors.ValidationError{
			Field:   "source",
			Message: "catalog observation source cannot be nil",
		}
	}
	builder, err := NewBuilderFrom(source)
	if err != nil {
		return nil, errors.WrapResource("create", "immutable catalog observation", "", err)
	}
	if err := validateCatalogIdentities(builder); err != nil {
		return nil, errors.WrapResource("validate", "catalog observation identities", "", err)
	}
	for _, provider := range builder.Providers().List() {
		for modelID, model := range provider.Models {
			if model == nil {
				continue
			}
			if modelID != model.ID {
				return nil, &errors.ValidationError{
					Field:   "provider.models",
					Value:   string(provider.ID) + "/" + modelID,
					Message: "map key must match model ID",
				}
			}
			if err := validateProviderModelPathID(model.ID); err != nil {
				return nil, errors.WrapResource(
					"validate", "provider observation model", string(provider.ID)+"/"+model.ID, err,
				)
			}
			if err := validateModelGeneration(model.Generation); err != nil {
				return nil, errors.WrapResource(
					"validate", "provider observation model", string(provider.ID)+"/"+model.ID, err,
				)
			}
			if model.ModelRef != "" {
				if _, _, err := ParseModelDefinitionID(model.ModelRef); err != nil {
					return nil, errors.WrapResource(
						"validate", "provider observation model reference", string(model.ModelRef), err,
					)
				}
			}
		}
	}
	return &Catalog{
		source:                     builder,
		definitions:                map[ModelDefinitionID]ModelDefinition{},
		offerings:                  map[OfferingKey]ProviderOffering{},
		providerOfferings:          map[ProviderID][]OfferingKey{},
		definitionOfferings:        map[ModelDefinitionID][]OfferingKey{},
		authorDefinitions:          map[AuthorID][]ModelDefinitionID{},
		definitionAliases:          map[string]ModelDefinitionID{},
		ambiguousDefinitionAliases: map[string][]ModelDefinitionID{},
	}, nil
}

var _ Reader = (*Catalog)(nil)

// Catalog is Starmap's immutable canonical catalog. Read methods provide the only
// access to its private state. Callers can retain it across goroutines.
type Catalog struct {
	source                     Reader
	definitions                map[ModelDefinitionID]ModelDefinition
	offerings                  map[OfferingKey]ProviderOffering
	providerOfferings          map[ProviderID][]OfferingKey
	definitionOfferings        map[ModelDefinitionID][]OfferingKey
	authorDefinitions          map[AuthorID][]ModelDefinitionID
	definitionAliases          map[string]ModelDefinitionID
	ambiguousDefinitionAliases map[string][]ModelDefinitionID
}

func buildCatalog(source Reader) (*Catalog, error) {
	if err := validateCatalogIdentities(source); err != nil {
		return nil, errors.WrapResource("validate", "catalog identities", "", err)
	}
	if err := validateProviderAuthorMappingTargets(source); err != nil {
		return nil, errors.WrapResource("validate", "provider author mapping targets", "", err)
	}
	views, err := deriveReadViews(source)
	if err != nil {
		return nil, errors.WrapResource("index", "catalog read views", "", err)
	}
	providerOfferings := make(map[ProviderID][]OfferingKey)
	definitionOfferings := make(map[ModelDefinitionID][]OfferingKey)
	for key := range views.offerings {
		providerOfferings[key.ProviderID] = append(providerOfferings[key.ProviderID], key)
		definitionID := views.offerings[key].DefinitionID
		definitionOfferings[definitionID] = append(definitionOfferings[definitionID], key)
	}
	for providerID, keys := range providerOfferings {
		slices.SortFunc(keys, func(left, right OfferingKey) int {
			return strings.Compare(string(left.ProviderModelID), string(right.ProviderModelID))
		})
		providerOfferings[providerID] = keys
	}
	for _, provider := range source.Providers().List() {
		keys := providerOfferings[provider.ID]
		if keys == nil {
			keys = []OfferingKey{}
			providerOfferings[provider.ID] = keys
		}
		for _, alias := range provider.Aliases {
			providerOfferings[alias] = keys
		}
	}
	for definitionID := range views.definitions {
		keys := definitionOfferings[definitionID]
		slices.SortFunc(keys, compareOfferingKey)
		if keys == nil {
			keys = []OfferingKey{}
		}
		definitionOfferings[definitionID] = keys
	}
	definitionAliases, ambiguousDefinitionAliases := buildDefinitionAliases(
		views.definitions, views.offerings,
	)

	return &Catalog{
		source:                     source,
		definitions:                views.definitions,
		offerings:                  views.offerings,
		providerOfferings:          providerOfferings,
		definitionOfferings:        definitionOfferings,
		authorDefinitions:          views.authorDefinitions,
		definitionAliases:          definitionAliases,
		ambiguousDefinitionAliases: ambiguousDefinitionAliases,
	}, nil
}

func compareOfferingKey(left, right OfferingKey) int {
	if compared := strings.Compare(string(left.ProviderID), string(right.ProviderID)); compared != 0 {
		return compared
	}
	return strings.Compare(string(left.ProviderModelID), string(right.ProviderModelID))
}

func buildDefinitionAliases(
	definitions map[ModelDefinitionID]ModelDefinition,
	offerings map[OfferingKey]ProviderOffering,
) (map[string]ModelDefinitionID, map[string][]ModelDefinitionID) {
	candidates := make(map[string]map[ModelDefinitionID]struct{})
	add := func(alias string, definitionID ModelDefinitionID) {
		if alias == "" {
			return
		}
		if candidates[alias] == nil {
			candidates[alias] = make(map[ModelDefinitionID]struct{})
		}
		candidates[alias][definitionID] = struct{}{}
	}
	for definitionID := range definitions {
		add(string(definitionID), definitionID)
		if _, slug, err := ParseModelDefinitionID(definitionID); err == nil {
			add(slug, definitionID)
		}
	}
	for _, offering := range offerings {
		add(string(offering.ProviderModelID), offering.DefinitionID)
	}

	aliases := make(map[string]ModelDefinitionID)
	ambiguous := make(map[string][]ModelDefinitionID)
	for alias, ids := range candidates {
		values := make([]ModelDefinitionID, 0, len(ids))
		for id := range ids {
			values = append(values, id)
		}
		slices.Sort(values)
		if len(values) == 1 {
			aliases[alias] = values[0]
		} else {
			ambiguous[alias] = values
		}
	}
	return aliases, ambiguous
}

// Providers returns the immutable catalog's provider collection reader.
func (r *Catalog) Providers() ProvidersReader {
	return providersReader{source: r.source.Providers()}
}

// Authors returns the immutable catalog's author collection reader.
func (r *Catalog) Authors() AuthorsReader {
	return authorsReader{source: r.source.Authors()}
}

// AuthoredModels returns caller-owned provider-independent construction
// records. Ordinary consumers normally use Definitions and AuthorModels.
func (r *Catalog) AuthoredModels() []AuthoredModel {
	records := r.source.AuthoredModels()
	result := make([]AuthoredModel, len(records))
	for i, record := range records {
		result[i] = copyAuthoredModel(record)
	}
	return result
}

// Provenance returns the immutable catalog's provenance reader.
func (r *Catalog) Provenance() ProvenanceReader {
	return provenanceReader{source: r.source.Provenance()}
}

// Provider returns a caller-owned copy of a provider.
func (r *Catalog) Provider(id ProviderID) (Provider, error) { return r.source.Provider(id) }

// Author returns a caller-owned copy of an author.
func (r *Catalog) Author(id AuthorID) (Author, error) { return r.source.Author(id) }

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

// DefinitionOfferings returns caller-owned offerings for one canonical model,
// ordered by provider and exact provider model ID.
func (r *Catalog) DefinitionOfferings(id ModelDefinitionID) ([]ProviderOffering, error) {
	keys, found := r.definitionOfferings[id]
	if !found {
		return nil, &errors.NotFoundError{Resource: "model definition", ID: string(id)}
	}
	offerings := make([]ProviderOffering, 0, len(keys))
	for _, key := range keys {
		offerings = append(offerings, copyProviderOffering(r.offerings[key]))
	}
	return offerings, nil
}

// AuthorModel resolves an author ID or alias plus a model slug.
func (r *Catalog) AuthorModel(authorID AuthorID, slug string) (ModelDefinition, error) {
	author, found := r.source.Authors().Resolve(authorID)
	if !found || author == nil {
		return ModelDefinition{}, &errors.NotFoundError{
			Resource: "author", ID: string(authorID),
		}
	}
	if err := validatePathSegment("model.slug", slug); err != nil {
		return ModelDefinition{}, err
	}
	return r.Definition(AuthoredModelID(author.ID, slug))
}

// AuthorModels returns caller-owned canonical model definitions attributed to
// an author or one of its aliases, ordered by definition ID.
func (r *Catalog) AuthorModels(authorID AuthorID) ([]ModelDefinition, error) {
	author, found := r.source.Authors().Resolve(authorID)
	if !found || author == nil {
		return nil, &errors.NotFoundError{Resource: "author", ID: string(authorID)}
	}
	ids := r.authorDefinitions[author.ID]
	definitions := make([]ModelDefinition, 0, len(ids))
	for _, id := range ids {
		definitions = append(definitions, copyModelDefinition(r.definitions[id]))
	}
	return definitions, nil
}

// FindModel returns the canonical provider-independent model definition.
// Use Offering for provider price, limits, availability, and request behavior.
func (r *Catalog) FindModel(id string) (ModelDefinition, error) {
	if _, found := r.definitions[ModelDefinitionID(id)]; found {
		return r.Definition(ModelDefinitionID(id))
	}
	if definitionID, found := r.definitionAliases[id]; found {
		return r.Definition(definitionID)
	}
	if candidates := r.ambiguousDefinitionAliases[id]; len(candidates) > 0 {
		return ModelDefinition{}, &errors.ConflictError{
			Resource: "model definition alias",
			Actual:   id,
			Message:  "matches multiple canonical models: " + joinDefinitionIDs(candidates),
		}
	}
	return ModelDefinition{}, &errors.NotFoundError{
		Resource: "model definition", ID: id,
	}
}

func joinDefinitionIDs(ids []ModelDefinitionID) string {
	values := make([]string, len(ids))
	for index, id := range ids {
		values[index] = string(id)
	}
	return strings.Join(values, ", ")
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

type provenanceReader struct{ source ProvenanceReader }

func (r provenanceReader) Map() provenance.Map { return r.source.Map() }
func (r provenanceReader) Len() int            { return r.source.Len() }
func (r provenanceReader) FindByField(resourceType evidence.ResourceType, resourceID, field string) []provenance.Entry {
	return r.source.FindByField(resourceType, resourceID, field)
}
func (r provenanceReader) FindByResource(resourceType evidence.ResourceType, resourceID string) map[string][]provenance.Entry {
	return r.source.FindByResource(resourceType, resourceID)
}
func (r provenanceReader) FindModelField(providerID ProviderID, modelID, field string) []provenance.Entry {
	return r.source.FindModelField(providerID, modelID, field)
}
func (r provenanceReader) FindModel(providerID ProviderID, modelID string) map[string][]provenance.Entry {
	return r.source.FindModel(providerID, modelID)
}
func (r provenanceReader) FormatYAML() string { return r.source.FormatYAML() }
