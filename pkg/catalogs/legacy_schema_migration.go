package catalogs

import (
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// MigrationChangeClassification describes how a legacy field was transformed.
type MigrationChangeClassification string

const (
	// MigrationChangeExact means the value moved without semantic change.
	MigrationChangeExact MigrationChangeClassification = "exact"
	// MigrationChangeDefaulted means the legacy schema had no value and a documented default was used.
	MigrationChangeDefaulted MigrationChangeClassification = "defaulted"
	// MigrationChangeConflict means multiple legacy records disagreed and deterministic precedence was applied.
	MigrationChangeConflict MigrationChangeClassification = "conflict"
	// MigrationChangeMissing means the legacy corpus had no authoritative value and none was invented.
	MigrationChangeMissing MigrationChangeClassification = "missing"
)

// LegacySchemaMigrationChange is one reviewable legacy-to-definition/offering difference.
type LegacySchemaMigrationChange struct {
	Classification MigrationChangeClassification `json:"classification" yaml:"classification"`
	Field          string                        `json:"field" yaml:"field"`
	OfferingKey    OfferingKey                   `json:"offering_key" yaml:"offering_key"`
	Message        string                        `json:"message" yaml:"message"`
}

// LegacySchemaMigrationReport summarizes all classified migration differences.
type LegacySchemaMigrationReport struct {
	Exact        int                           `json:"exact" yaml:"exact"`
	Defaulted    int                           `json:"defaulted" yaml:"defaulted"`
	Conflicts    int                           `json:"conflicts" yaml:"conflicts"`
	Missing      int                           `json:"missing" yaml:"missing"`
	Unclassified int                           `json:"unclassified" yaml:"unclassified"`
	Changes      []LegacySchemaMigrationChange `json:"changes" yaml:"changes"`
}

// LegacySchemaMigration is the loss-accounted result of converting overloaded
// legacy Model records into canonical definitions and provider offerings.
type LegacySchemaMigration struct {
	Definitions map[ModelDefinitionID]ModelDefinition `json:"definitions" yaml:"definitions"`
	Offerings   map[OfferingKey]ProviderOffering      `json:"-" yaml:"-"`
	Report      LegacySchemaMigrationReport           `json:"report" yaml:"report"`
}

// MigrateLegacySchema deterministically converts every provider model in a
// legacy catalog. It never mutates the input and reports every default or
// conflicting canonical-definition choice.
func MigrateLegacySchema(reader Reader) (*LegacySchemaMigration, error) {
	if reader == nil {
		return nil, &errors.ValidationError{Field: "catalog", Message: "reader is required"}
	}
	result := &LegacySchemaMigration{
		Definitions: make(map[ModelDefinitionID]ModelDefinition),
		Offerings:   make(map[OfferingKey]ProviderOffering),
	}
	authorIndex := legacyAuthorIndex(reader)
	providers := reader.Providers().List()
	slices.SortFunc(providers, func(left, right Provider) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	for _, provider := range providers {
		modelIDs := make([]string, 0, len(provider.Models))
		for modelID := range provider.Models {
			modelIDs = append(modelIDs, modelID)
		}
		slices.Sort(modelIDs)
		for _, mapModelID := range modelIDs {
			legacy := provider.Models[mapModelID]
			if legacy == nil {
				continue
			}
			fallbackAuthors := authorIndex[legacy.ID]
			fallbackSource := "legacy author-model catalog"
			if len(fallbackAuthors) == 0 {
				var err error
				fallbackAuthors, err = legacyAttributedAuthors(reader, provider.ID, legacy.ID)
				if err != nil {
					return nil, err
				}
				fallbackSource = "author attribution rules"
			}
			// A provider catalog may enumerate every author available through a
			// marketplace. That is a candidate set, not joint authorship of every
			// model. Only a single unambiguous declaration is a safe fallback.
			if len(fallbackAuthors) == 0 && provider.Catalog != nil && len(provider.Catalog.Authors) == 1 {
				fallbackAuthors = canonicalLegacyAuthors(reader, provider.Catalog.Authors)
				fallbackSource = "single provider catalog author declaration"
			}
			if err := migrateLegacyModel(result, provider.ID, mapModelID, *legacy, fallbackAuthors, fallbackSource); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func canonicalLegacyAuthors(reader Reader, ids []AuthorID) []AuthorID {
	canonical := make([]AuthorID, 0, len(ids))
	for _, id := range ids {
		if author, found := reader.Authors().Resolve(id); found {
			canonical = append(canonical, author.ID)
			continue
		}
		canonical = append(canonical, id)
	}
	slices.Sort(canonical)
	return slices.Compact(canonical)
}

func migrateLegacyModel(result *LegacySchemaMigration, providerID ProviderID, mapModelID string, legacy Model, fallbackAuthors []AuthorID, fallbackSource string) error {
	providerModelID := ProviderModelID(legacy.ID)
	key := OfferingKey{ProviderID: providerID, ProviderModelID: providerModelID}
	if mapModelID != legacy.ID {
		result.addChange(MigrationChangeConflict, "provider_model_id", key,
			fmt.Sprintf("provider map key %q differs from model ID %q; exact model ID retained", mapModelID, legacy.ID))
	}
	if strings.TrimSpace(legacy.Name) == "" {
		result.addChange(MigrationChangeDefaulted, "name", key,
			"legacy display name was absent; exact provider model ID retained as the definition name")
		legacy.Name = legacy.ID
	}
	definition := legacyModelDefinition(legacy, fallbackAuthors)
	if len(legacy.Authors) == 0 && len(fallbackAuthors) > 0 {
		result.addChange(MigrationChangeDefaulted, "author_ids", key,
			"inline authorship was absent; author IDs derived from "+fallbackSource)
	}
	if len(legacy.Authors) == 0 && len(fallbackAuthors) == 0 {
		result.addChange(MigrationChangeMissing, "author_ids", key,
			"no inline, author-model, provider-declared, or attribution-rule authorship exists; preserved as unknown")
	}
	if definition.Lineage.Root != nil && *definition.Lineage.Root == definition.ID {
		result.addChange(MigrationChangeConflict, "lineage.root", key,
			"legacy root referenced the definition itself; invalid self-reference removed")
		definition.Lineage.Root = nil
	}
	if definition.Lineage.Parent != nil && *definition.Lineage.Parent == definition.ID {
		result.addChange(MigrationChangeConflict, "lineage.parent", key,
			"legacy parent referenced the definition itself; invalid self-reference removed")
		definition.Lineage.Parent = nil
	}
	if err := definition.Validate(); err != nil {
		return errors.WrapResource("migrate", "model definition", legacy.ID, err)
	}
	if existing, found := result.Definitions[definition.ID]; found {
		if !reflect.DeepEqual(existing, definition) {
			result.addChange(MigrationChangeConflict, "definition", key,
				"provider-independent fields disagree; first provider in canonical ID order retained")
		}
	} else {
		result.Definitions[definition.ID] = definition
	}

	offering, changes, err := legacyProviderOffering(providerID, legacy)
	if err != nil {
		return err
	}
	if err := offering.Validate(); err != nil {
		return errors.WrapResource("migrate", "provider offering", string(providerID)+"/"+legacy.ID, err)
	}
	if _, exists := result.Offerings[offering.Key()]; exists {
		return &errors.ConflictError{Resource: "provider offering", Message: "duplicate offering key"}
	}
	result.Offerings[offering.Key()] = offering
	for _, change := range changes {
		result.addChange(change.Classification, change.Field, offering.Key(), change.Message)
	}
	return nil
}

func legacyModelDefinition(legacy Model, fallbackAuthors []AuthorID) ModelDefinition {
	copied := DeepCopyModel(legacy)
	authorIDs := make([]AuthorID, 0, len(copied.Authors))
	for _, author := range copied.Authors {
		authorIDs = append(authorIDs, author.ID)
	}
	if len(authorIDs) == 0 {
		authorIDs = append(authorIDs, fallbackAuthors...)
	}
	definition := ModelDefinition{
		ID:          ModelDefinitionID(copied.ID),
		Name:        copied.Name,
		AuthorIDs:   authorIDs,
		Description: copied.Description,
		CreatedAt:   copied.CreatedAt,
		UpdatedAt:   copied.UpdatedAt,
		Capabilities: ModelDefinitionCapabilities{
			Features:        copied.Features,
			Attachments:     copied.Attachments,
			Generation:      copied.Generation,
			Reasoning:       copied.Reasoning,
			ReasoningTokens: copied.ReasoningTokens,
			Verbosity:       copied.Verbosity,
			Tools:           copied.Tools,
			Delivery:        copied.Delivery,
		},
	}
	if copied.Metadata != nil {
		definition.Metadata = ModelDefinitionMetadata{
			ReleaseDate:     copied.Metadata.ReleaseDate,
			KnowledgeCutoff: copied.Metadata.KnowledgeCutoff,
			Tags:            append([]ModelTag(nil), copied.Metadata.Tags...),
		}
		definition.Weights = ModelDefinitionWeights{
			Open:         copied.Metadata.OpenWeights,
			Architecture: copied.Metadata.Architecture,
		}
	}
	if copied.Lineage != nil {
		definition.Lineage.Family = copied.Lineage.Family
		definition.Lineage.Root = definitionIDPointer(copied.Lineage.Root)
		definition.Lineage.Parent = definitionIDPointer(copied.Lineage.Parent)
	}
	return definition
}

func legacyAuthorIndex(reader Reader) map[string][]AuthorID {
	index := make(map[string][]AuthorID)
	for _, author := range reader.Authors().List() {
		for modelID, model := range author.Models {
			if model == nil {
				continue
			}
			index[modelID] = append(index[modelID], author.ID)
		}
	}
	for modelID := range index {
		slices.Sort(index[modelID])
		index[modelID] = slices.Compact(index[modelID])
	}
	return index
}

func legacyAttributedAuthors(reader Reader, providerID ProviderID, modelID string) ([]AuthorID, error) {
	var resolved []AuthorID
	for _, author := range reader.Authors().List() {
		if author.Catalog == nil || author.Catalog.Attribution == nil {
			continue
		}
		attribution := author.Catalog.Attribution
		if attribution.ProviderID != "" && attribution.ProviderID != providerID {
			continue
		}
		if len(attribution.Patterns) == 0 {
			if attribution.ProviderID == providerID {
				resolved = append(resolved, author.ID)
			}
			continue
		}
		for _, pattern := range attribution.Patterns {
			matched, err := path.Match(strings.ToLower(pattern), strings.ToLower(modelID))
			if err != nil {
				return nil, errors.WrapParse("glob", "author "+string(author.ID)+" pattern", err)
			}
			if matched {
				resolved = append(resolved, author.ID)
				break
			}
		}
	}
	slices.Sort(resolved)
	return slices.Compact(resolved), nil
}

func legacyProviderOffering(providerID ProviderID, legacy Model) (ProviderOffering, []LegacySchemaMigrationChange, error) {
	copied := DeepCopyModel(legacy)
	offering := ProviderOffering{
		ProviderID:      providerID,
		ProviderModelID: ProviderModelID(copied.ID),
		DefinitionID:    ModelDefinitionID(copied.ID),
		Pricing:         copied.Pricing,
		Limits:          copied.Limits,
		Availability:    OfferingAvailabilityAvailable,
		Lifecycle:       legacyOfferingLifecycle(copied.Status),
	}
	changes := []LegacySchemaMigrationChange{{
		Classification: MigrationChangeDefaulted,
		Field:          "availability",
		Message:        "legacy schema had no availability; defaulted to available",
	}}
	if copied.Status == "" || copied.Status == ModelStatusUnknown {
		changes = append(changes, LegacySchemaMigrationChange{
			Classification: MigrationChangeDefaulted,
			Field:          "lifecycle",
			Message:        "legacy lifecycle was absent or unknown; defaulted to active",
		})
	}
	if len(copied.Modes) > 0 {
		offering.Modes = make(map[string]ProviderOfferingMode, len(copied.Modes))
	}
	for modeName, legacyMode := range copied.Modes {
		mode := ProviderOfferingMode{Pricing: legacyMode.Pricing}
		if legacyMode.Provider != nil {
			mode.Request.Headers = OfferingRequestHeaders(legacyMode.Provider.Headers)
			if len(legacyMode.Provider.Body) > 0 {
				mode.Request.Body = make(OfferingRequestBody, len(legacyMode.Provider.Body))
				for field, value := range legacyMode.Provider.Body {
					encoded, err := json.Marshal(value)
					if err != nil {
						return ProviderOffering{}, nil, errors.WrapParse("json", "mode "+modeName+" body field "+field, err)
					}
					mode.Request.Body[field] = encoded
				}
			}
		}
		offering.Modes[modeName] = mode
	}
	return offering, changes, nil
}

func legacyOfferingLifecycle(status ModelStatus) OfferingLifecycle {
	switch status {
	case ModelStatusBeta, ModelStatusPreview:
		return OfferingLifecyclePreview
	case ModelStatusDeprecated:
		return OfferingLifecycleDeprecated
	case ModelStatusActive:
		return OfferingLifecycleActive
	default:
		return OfferingLifecycleActive
	}
}

func definitionIDPointer(value *string) *ModelDefinitionID {
	if value == nil {
		return nil
	}
	converted := ModelDefinitionID(*value)
	return &converted
}

func (r *LegacySchemaMigration) addChange(
	classification MigrationChangeClassification,
	field string,
	key OfferingKey,
	message string,
) {
	r.Report.Changes = append(r.Report.Changes, LegacySchemaMigrationChange{
		Classification: classification,
		Field:          field,
		OfferingKey:    key,
		Message:        message,
	})
	switch classification {
	case MigrationChangeExact:
		r.Report.Exact++
	case MigrationChangeDefaulted:
		r.Report.Defaulted++
	case MigrationChangeConflict:
		r.Report.Conflicts++
	case MigrationChangeMissing:
		r.Report.Missing++
	default:
		r.Report.Unclassified++
	}
}
