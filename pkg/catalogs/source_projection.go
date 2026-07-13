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

// sourceProjectionChangeClassification describes how an ingestion field was projected.
type sourceProjectionChangeClassification string

const (
	projectionChangeExact     sourceProjectionChangeClassification = "exact"
	projectionChangeDefaulted sourceProjectionChangeClassification = "defaulted"
	projectionChangeConflict  sourceProjectionChangeClassification = "conflict"
	projectionChangeMissing   sourceProjectionChangeClassification = "missing"
)

// sourceProjectionChange is one reviewable source-to-canonical difference.
type sourceProjectionChange struct {
	Classification sourceProjectionChangeClassification `json:"classification" yaml:"classification"`
	Field          string                               `json:"field" yaml:"field"`
	OfferingKey    OfferingKey                          `json:"offering_key" yaml:"offering_key"`
	Message        string                               `json:"message" yaml:"message"`
}

// sourceProjectionReport summarizes classified ingestion projections.
type sourceProjectionReport struct {
	Exact        int                      `json:"exact" yaml:"exact"`
	Defaulted    int                      `json:"defaulted" yaml:"defaulted"`
	Conflicts    int                      `json:"conflicts" yaml:"conflicts"`
	Missing      int                      `json:"missing" yaml:"missing"`
	Unclassified int                      `json:"unclassified" yaml:"unclassified"`
	Changes      []sourceProjectionChange `json:"changes" yaml:"changes"`
}

// sourceProjection is the loss-accounted result of projecting provider-source
// Model records into canonical definitions and provider offerings.
type sourceProjection struct {
	Definitions map[ModelDefinitionID]ModelDefinition `json:"definitions" yaml:"definitions"`
	Offerings   map[OfferingKey]ProviderOffering      `json:"-" yaml:"-"`
	Report      sourceProjectionReport                `json:"report" yaml:"report"`
}

func newSourceProjection() *sourceProjection {
	return &sourceProjection{
		Definitions: make(map[ModelDefinitionID]ModelDefinition),
		Offerings:   make(map[OfferingKey]ProviderOffering),
	}
}

// projectSourceModels derives canonical records from current provider-source
// models while a Builder is assembled. It is not a payload decoder or a
// compatibility migration path.
func projectSourceModels(reader ModelSourceReader) (*sourceProjection, error) {
	if reader == nil {
		return nil, &errors.ValidationError{Field: "catalog", Message: "reader is required"}
	}
	result := newSourceProjection()
	authorIndex := sourceAuthorIndex(reader)
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
			fallbackSource := "source author-model catalog"
			if len(fallbackAuthors) == 0 {
				var err error
				fallbackAuthors, err = sourceAttributedAuthors(reader, provider.ID, legacy.ID)
				if err != nil {
					return nil, err
				}
				fallbackSource = "author attribution rules"
			}
			// A provider catalog may enumerate every author available through a
			// marketplace. That is a candidate set, not joint authorship of every
			// model. Only a single unambiguous declaration is a safe fallback.
			if len(fallbackAuthors) == 0 && provider.Catalog != nil && len(provider.Catalog.Authors) == 1 {
				fallbackAuthors = canonicalSourceAuthors(reader, provider.Catalog.Authors)
				fallbackSource = "single provider catalog author declaration"
			}
			if err := projectSourceModel(result, provider, mapModelID, *legacy, fallbackAuthors, fallbackSource); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func canonicalSourceAuthors(reader ModelSourceReader, ids []AuthorID) []AuthorID {
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

func projectSourceModel(result *sourceProjection, provider Provider, mapModelID string, legacy Model, fallbackAuthors []AuthorID, fallbackSource string) error {
	providerID := provider.ID
	providerModelID := ProviderModelID(legacy.ID)
	key := OfferingKey{ProviderID: providerID, ProviderModelID: providerModelID}
	if mapModelID != legacy.ID {
		result.addChange(projectionChangeConflict, "provider_model_id", key,
			fmt.Sprintf("provider map key %q differs from model ID %q; exact model ID retained", mapModelID, legacy.ID))
	}
	if strings.TrimSpace(legacy.Name) == "" {
		result.addChange(projectionChangeDefaulted, "name", key,
			"source display name was absent; exact provider model ID retained as the definition name")
		legacy.Name = legacy.ID
	}
	definition := sourceModelDefinition(legacy, fallbackAuthors)
	if len(legacy.Authors) == 0 && len(fallbackAuthors) > 0 {
		result.addChange(projectionChangeDefaulted, "author_ids", key,
			"inline authorship was absent; author IDs derived from "+fallbackSource)
	}
	if len(legacy.Authors) == 0 && len(fallbackAuthors) == 0 {
		result.addChange(projectionChangeMissing, "author_ids", key,
			"no inline, author-model, provider-declared, or attribution-rule authorship exists; preserved as unknown")
	}
	if definition.Lineage.Root != nil && *definition.Lineage.Root == definition.ID {
		result.addChange(projectionChangeConflict, "lineage.root", key,
			"source root referenced the definition itself; invalid self-reference removed")
		definition.Lineage.Root = nil
	}
	if definition.Lineage.Parent != nil && *definition.Lineage.Parent == definition.ID {
		result.addChange(projectionChangeConflict, "lineage.parent", key,
			"source parent referenced the definition itself; invalid self-reference removed")
		definition.Lineage.Parent = nil
	}
	if err := definition.Validate(); err != nil {
		return errors.WrapResource("migrate", "model definition", legacy.ID, err)
	}
	if existing, found := result.Definitions[definition.ID]; found {
		if !reflect.DeepEqual(existing, definition) {
			result.addChange(projectionChangeConflict, "definition", key,
				"provider-independent fields disagree; first provider in canonical ID order retained")
		}
	} else {
		result.Definitions[definition.ID] = definition
	}

	offering, changes, err := sourceProviderOffering(provider, legacy)
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

func sourceModelDefinition(legacy Model, fallbackAuthors []AuthorID) ModelDefinition {
	copied := DeepCopyModel(legacy)
	authorIDs := make([]AuthorID, 0, len(copied.Authors))
	for _, author := range copied.Authors {
		authorIDs = append(authorIDs, author.ID)
	}
	if len(authorIDs) == 0 {
		authorIDs = append(authorIDs, fallbackAuthors...)
	}
	definitionID := ModelDefinitionID(copied.ID)
	if copied.DefinitionID != "" {
		definitionID = copied.DefinitionID
	}
	definition := ModelDefinition{
		ID:          definitionID,
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

func sourceAuthorIndex(reader ModelSourceReader) map[string][]AuthorID {
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

func sourceAttributedAuthors(reader ModelSourceReader, providerID ProviderID, modelID string) ([]AuthorID, error) {
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

func sourceProviderOffering(provider Provider, legacy Model) (ProviderOffering, []sourceProjectionChange, error) { //nolint:gocyclo // Explicit precedence stages keep definition/offering projection auditable.
	copied := DeepCopyModel(legacy)
	endpointType := EndpointTypeOpenAI
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type != "" {
		endpointType = provider.Catalog.Endpoint.Type
	}
	api := InvocationAPIChatCompletions
	if endpointType == EndpointTypeAnthropic {
		api = InvocationAPIMessages
	}
	apis := []InvocationAPI{api}
	var providerDefaults *ProviderOfferingDefaults
	if provider.Catalog != nil {
		if provider.Catalog.Offering != nil {
			defaults := deepCopyProviderOfferingDefaults(provider.Catalog.Offering)
			defaults.Endpoint = provider.CatalogOfferingEndpoint()
			providerDefaults = defaults
		}
	}
	if providerDefaults != nil {
		apis = append([]InvocationAPI(nil), providerDefaults.Access.APIs...)
	}
	if copied.InvocationAPIs != nil {
		apis = append([]InvocationAPI(nil), copied.InvocationAPIs...)
	}
	routability := OfferingRoutabilityRoutable
	if len(apis) == 0 {
		routability = OfferingRoutabilityDiscoverable
	}
	access := OfferingAccess{Channel: OfferingAccessChannelServerToServer, Routability: routability, APIs: apis}
	if providerDefaults != nil {
		access = providerDefaults.Access
		access.APIs = append([]InvocationAPI(nil), providerDefaults.Access.APIs...)
	}
	if copied.OfferingAccess != nil {
		access = *copied.OfferingAccess
		access.APIs = append([]InvocationAPI(nil), copied.OfferingAccess.APIs...)
	}
	deployment := ProviderDeployment{Type: "serverless"}
	if providerDefaults != nil {
		deployment = providerDefaults.Deployment
	}
	if copied.OfferingDeployment.Type != "" {
		deployment = copied.OfferingDeployment
	}
	if access.Channel == OfferingAccessChannelApplication {
		deployment.Type = "application"
	}
	offeringEndpoint := ProviderOfferingEndpoint{}
	if providerDefaults != nil {
		offeringEndpoint = providerDefaults.Endpoint
	}
	if copied.OfferingEndpoint != (ProviderOfferingEndpoint{}) {
		offeringEndpoint = copied.OfferingEndpoint
	}
	if offeringEndpoint.Type == "" {
		offeringEndpoint.Type = endpointType
	}
	offering := ProviderOffering{
		ProviderID:      provider.ID,
		ProviderModelID: ProviderModelID(copied.ID),
		DefinitionID:    ModelDefinitionID(copied.ID),
		Pricing:         copied.Pricing,
		Limits:          copied.Limits,
		Availability:    OfferingAvailabilityAvailable,
		Access:          access,
		Endpoint:        offeringEndpoint,
		Deployment:      deployment,
		Lifecycle:       sourceOfferingLifecycle(copied.Status),
	}
	if copied.DefinitionID != "" {
		offering.DefinitionID = copied.DefinitionID
	}
	if copied.OfferingAvailability != "" {
		offering.Availability = copied.OfferingAvailability
	}
	if copied.AggregatorUpstream != nil {
		upstream := *copied.AggregatorUpstream
		offering.AggregatorUpstream = &upstream
	}
	if providerDefaults != nil {
		offering.Regions = copyProviderOffering(ProviderOffering{Regions: providerDefaults.Regions}).Regions
	}
	if copied.OfferingRegions != nil {
		offering.Regions = copyProviderOffering(ProviderOffering{Regions: copied.OfferingRegions}).Regions
	}
	if copied.OfferingInferenceProfile != nil {
		profile := *copied.OfferingInferenceProfile
		profile.SourceRegions = append([]string(nil), copied.OfferingInferenceProfile.SourceRegions...)
		profile.DestinationRegions = append([]string(nil), copied.OfferingInferenceProfile.DestinationRegions...)
		offering.InferenceProfile = &profile
	}
	changes := []sourceProjectionChange{{
		Classification: projectionChangeDefaulted,
		Field:          "availability",
		Message:        "source model had no availability; defaulted to available",
	}, {
		Classification: projectionChangeDefaulted,
		Field:          "access",
		Message:        "configured provider endpoint defaulted to a routable server-to-server access contract",
	}, {
		Classification: projectionChangeDefaulted,
		Field:          "deployment",
		Message:        "source model had no deployment type; defaulted to serverless",
	}}
	if copied.InvocationAPIs != nil || copied.OfferingAccess != nil {
		changes[1].Classification = projectionChangeExact
		changes[1].Message = "source access channel and invocation APIs preserved in the provider offering access contract"
	}
	if providerDefaults != nil && copied.InvocationAPIs == nil && copied.OfferingAccess == nil {
		changes[1].Classification = projectionChangeExact
		changes[1].Message = "provider-configured access channel and invocation APIs preserved in the offering contract"
		changes[2].Classification = projectionChangeExact
		changes[2].Message = "provider-configured deployment default preserved in the offering contract"
	}
	if copied.OfferingEndpoint.Type != "" {
		changes = append(changes, sourceProjectionChange{
			Classification: projectionChangeExact,
			Field:          "endpoint",
			Message:        "source offering endpoint preserved exactly",
		})
	}
	if access.Channel == OfferingAccessChannelApplication {
		changes[2].Classification = projectionChangeExact
		changes[2].Message = "application-only source access mapped to an application deployment"
	}
	if copied.Status == "" || copied.Status == ModelStatusUnknown {
		changes = append(changes, sourceProjectionChange{
			Classification: projectionChangeDefaulted,
			Field:          "lifecycle",
			Message:        "source lifecycle was absent or unknown; defaulted to active",
		})
	}
	if len(copied.Modes) > 0 {
		offering.Modes = make(map[string]ProviderOfferingMode, len(copied.Modes))
	}
	for modeName, legacyMode := range copied.Modes {
		mode := ProviderOfferingMode{Pricing: legacyMode.Pricing, Deployment: legacyMode.Deployment}
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

func sourceOfferingLifecycle(status ModelStatus) OfferingLifecycle {
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

func (r *sourceProjection) addChange(
	classification sourceProjectionChangeClassification,
	field string,
	key OfferingKey,
	message string,
) {
	r.Report.Changes = append(r.Report.Changes, sourceProjectionChange{
		Classification: classification,
		Field:          field,
		OfferingKey:    key,
		Message:        message,
	})
	switch classification {
	case projectionChangeExact:
		r.Report.Exact++
	case projectionChangeDefaulted:
		r.Report.Defaulted++
	case projectionChangeConflict:
		r.Report.Conflicts++
	case projectionChangeMissing:
		r.Report.Missing++
	default:
		r.Report.Unclassified++
	}
}
