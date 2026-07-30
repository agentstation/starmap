package catalogs

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/internal/catalog/authority"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

type catalogReadViews struct {
	definitions       map[ModelDefinitionID]ModelDefinition
	offerings         map[OfferingKey]ProviderOffering
	authorDefinitions map[AuthorID][]ModelDefinitionID
}

type providerModelCandidate struct {
	providerID   ProviderID
	definitionID ModelDefinitionID
	endpoint     ProviderOfferingEndpoint
	model        Model
}

type rankedDefinitionValue[T any] struct {
	value       T
	providerID  ProviderID
	evidence    provenance.Entry
	hasEvidence bool
	authority   float64
	recordTime  utc.Time
	semantic    string
}

type definitionPresenceClaim[T any] struct {
	Value T
	State ValuePresence
}

func deriveReadViews(reader Reader) (*catalogReadViews, error) {
	if reader == nil {
		return nil, &errors.ValidationError{Field: "catalog", Message: "reader is required"}
	}

	byDefinition, authoredDefinitions, err := indexAuthoredDefinitionCandidates(reader)
	if err != nil {
		return nil, err
	}
	offerings, err := indexProviderOfferings(reader, authoredDefinitions)
	if err != nil {
		return nil, err
	}
	definitions, definitionIDs, err := buildDefinitions(reader, byDefinition)
	if err != nil {
		return nil, err
	}
	if err := normalizeDefinitionLineages(definitions, offerings, definitionIDs); err != nil {
		return nil, err
	}

	return &catalogReadViews{
		definitions:       definitions,
		offerings:         offerings,
		authorDefinitions: deriveAuthorDefinitions(reader, definitions),
	}, nil
}

func indexAuthoredDefinitionCandidates(
	reader Reader,
) (map[ModelDefinitionID][]providerModelCandidate, map[ModelDefinitionID]struct{}, error) {
	byDefinition := make(map[ModelDefinitionID][]providerModelCandidate)
	authored := make(map[ModelDefinitionID]struct{})
	for _, record := range reader.AuthoredModels() {
		if err := validateAuthoredModel(record.AuthorID, record.Model); err != nil {
			return nil, nil, errors.WrapResource(
				"validate", "authored model", string(record.ID()), err,
			)
		}
		definitionID := record.ID()
		if _, exists := authored[definitionID]; exists {
			return nil, nil, &errors.ConflictError{
				Resource: "authored model",
				Message:  "duplicate canonical identity " + string(definitionID),
			}
		}
		authored[definitionID] = struct{}{}
		byDefinition[definitionID] = []providerModelCandidate{{
			definitionID: definitionID,
			model:        DeepCopyModel(record.Model),
		}}
	}
	return byDefinition, authored, nil
}

func indexProviderOfferings(
	reader Reader,
	authored map[ModelDefinitionID]struct{},
) (map[OfferingKey]ProviderOffering, error) {
	offerings := make(map[OfferingKey]ProviderOffering)
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
			model := provider.Models[mapModelID]
			if model == nil {
				continue
			}
			candidate, err := validatedProviderModelCandidate(
				provider,
				mapModelID,
				model,
				authored,
			)
			if err != nil {
				return nil, err
			}
			offering, err := deriveProviderOffering(candidate)
			if err != nil {
				return nil, err
			}
			if _, exists := offerings[offering.Key()]; exists {
				return nil, &errors.ConflictError{
					Resource: "provider offering",
					Message:  "duplicate offering key " + string(provider.ID) + "/" + model.ID,
				}
			}
			offerings[offering.Key()] = offering
		}
	}
	return offerings, nil
}

func validatedProviderModelCandidate(
	provider Provider,
	mapModelID string,
	model *Model,
	authored map[ModelDefinitionID]struct{},
) (providerModelCandidate, error) {
	if mapModelID != model.ID {
		return providerModelCandidate{}, &errors.ValidationError{
			Field: "provider.models",
			Value: string(provider.ID) + "/" + mapModelID,
			Message: fmt.Sprintf(
				"map key does not match model ID %q",
				model.ID,
			),
		}
	}
	if err := validateProviderModelPathID(model.ID); err != nil {
		return providerModelCandidate{}, errors.WrapResource(
			"validate", "provider model", string(provider.ID)+"/"+model.ID, err,
		)
	}
	if err := validateModelGeneration(model.Generation); err != nil {
		return providerModelCandidate{}, errors.WrapResource(
			"validate", "provider model", string(provider.ID)+"/"+model.ID, err,
		)
	}
	if err := validateModelFactConsistency(*model); err != nil {
		return providerModelCandidate{}, errors.WrapResource(
			"validate", "provider model", string(provider.ID)+"/"+model.ID, err,
		)
	}
	candidate := providerModelCandidate{
		providerID:   provider.ID,
		definitionID: model.ModelRef,
		endpoint:     deriveProviderOfferingEndpoint(provider, *model),
		model:        DeepCopyModel(*model),
	}
	if candidate.definitionID == "" {
		return providerModelCandidate{}, &errors.ValidationError{
			Field:   "provider_model.model",
			Value:   string(provider.ID) + "/" + model.ID,
			Message: "explicit canonical author/model reference is required",
		}
	}
	if _, _, err := ParseModelDefinitionID(model.ModelRef); err != nil {
		return providerModelCandidate{}, errors.WrapResource(
			"validate", "provider model reference", string(model.ModelRef), err,
		)
	}
	if _, exists := authored[candidate.definitionID]; !exists {
		return providerModelCandidate{}, &errors.NotFoundError{
			Resource: "authored model",
			ID:       string(candidate.definitionID),
		}
	}
	return candidate, nil
}

func buildDefinitions(
	reader Reader,
	byDefinition map[ModelDefinitionID][]providerModelCandidate,
) (map[ModelDefinitionID]ModelDefinition, []ModelDefinitionID, error) {
	policies := authority.New()
	definitions := make(map[ModelDefinitionID]ModelDefinition, len(byDefinition))
	definitionIDs := make([]ModelDefinitionID, 0, len(byDefinition))
	for id := range byDefinition {
		definitionIDs = append(definitionIDs, id)
	}
	slices.Sort(definitionIDs)
	for _, id := range definitionIDs {
		definition, err := deriveModelDefinition(reader, policies, id, byDefinition[id])
		if err != nil {
			return nil, nil, err
		}
		definitions[id] = definition
	}
	return definitions, definitionIDs, nil
}

func normalizeDefinitionLineages(
	definitions map[ModelDefinitionID]ModelDefinition,
	offerings map[OfferingKey]ProviderOffering,
	definitionIDs []ModelDefinitionID,
) error {
	lineageAliases, ambiguousLineageAliases := buildDefinitionAliases(definitions, offerings)
	for _, id := range definitionIDs {
		definition := definitions[id]
		if err := normalizeDefinitionLineage(
			&definition,
			lineageAliases,
			ambiguousLineageAliases,
		); err != nil {
			return err
		}
		definitions[id] = definition
	}
	return nil
}

func normalizeDefinitionLineage(
	definition *ModelDefinition,
	aliases map[string]ModelDefinitionID,
	ambiguous map[string][]ModelDefinitionID,
) error {
	resolve := func(field string, reference **ModelDefinitionID) error {
		if *reference == nil {
			return nil
		}
		raw := string(**reference)
		canonical, found := aliases[raw]
		if candidates := ambiguous[raw]; len(candidates) != 0 {
			return &errors.ConflictError{
				Resource: "model lineage " + field,
				Message:  fmt.Sprintf("%q resolves ambiguously to %v", raw, candidates),
			}
		}
		if !found {
			return &errors.NotFoundError{
				Resource: "model lineage " + field,
				ID:       raw,
			}
		}
		if canonical == definition.ID {
			if field == "root" {
				// A source sometimes repeats the model's own provider ID as its
				// lineage root. It is resolvable but carries no parent relation.
				*reference = nil
				return nil
			}
			return &errors.ValidationError{
				Field:   "model.lineage." + field,
				Value:   raw,
				Message: "must not refer to the model itself",
			}
		}
		*reference = &canonical
		return nil
	}
	if err := resolve("root", &definition.Lineage.Root); err != nil {
		return err
	}
	return resolve("parent", &definition.Lineage.Parent)
}

func deriveModelDefinition(
	reader Reader,
	policies authority.Reader,
	id ModelDefinitionID,
	candidates []providerModelCandidate,
) (ModelDefinition, error) {
	authorIDs, err := deriveDefinitionAuthors(reader, candidates)
	if err != nil {
		return ModelDefinition{}, err
	}
	name, description, err := deriveDefinitionIdentity(reader, policies, id, candidates)
	if err != nil {
		return ModelDefinition{}, err
	}
	metadata, weights, err := deriveDefinitionMetadata(reader, policies, candidates)
	if err != nil {
		return ModelDefinition{}, err
	}
	lineage, err := deriveDefinitionLineage(reader, policies, id, candidates)
	if err != nil {
		return ModelDefinition{}, err
	}
	capabilities, err := deriveDefinitionCapabilities(reader, policies, candidates)
	if err != nil {
		return ModelDefinition{}, err
	}
	createdAt, updatedAt := deriveDefinitionTimestamps(candidates)

	definition := ModelDefinition{
		ID:           id,
		Name:         name,
		AuthorIDs:    authorIDs,
		Description:  description,
		Metadata:     metadata,
		Lineage:      lineage,
		Weights:      weights,
		Capabilities: capabilities,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
	if err := definition.Validate(); err != nil {
		return ModelDefinition{}, errors.WrapResource("derive", "model definition", string(id), err)
	}
	return copyModelDefinition(definition), nil
}

func deriveDefinitionIdentity(
	reader Reader,
	policies authority.Reader,
	id ModelDefinitionID,
	candidates []providerModelCandidate,
) (string, string, error) {
	name, found, err := selectDefinitionValue(reader, policies, "Name", candidates, func(model Model) (string, bool) {
		return model.Name, strings.TrimSpace(model.Name) != ""
	})
	if err != nil {
		return "", "", err
	}
	if !found {
		name = string(id)
	}
	description, found, err := selectDefinitionValue(reader, policies, "Description", candidates, func(model Model) (string, bool) {
		value, presence := model.DescriptionValue()
		return value, presence == ValueKnown
	})
	if err != nil {
		return "", "", err
	}
	if !found {
		description = ""
	}
	return name, description, nil
}

func deriveDefinitionLineage(
	reader Reader,
	policies authority.Reader,
	id ModelDefinitionID,
	candidates []providerModelCandidate,
) (ModelDefinitionLineage, error) {
	var lineage ModelDefinitionLineage
	family, found, err := selectDefinitionValue(reader, policies, "Lineage.Family", candidates, func(model Model) (string, bool) {
		return modelLineageFamily(model)
	})
	if err != nil {
		return lineage, err
	}
	if found {
		lineage.Family = family
	}
	root, found, err := selectDefinitionValue(reader, policies, "Lineage.Root", candidates, func(model Model) (*string, bool) {
		if model.Lineage == nil || model.Lineage.Root == nil {
			return nil, false
		}
		return model.Lineage.Root, true
	})
	if err != nil {
		return lineage, err
	}
	if found && root != nil && ModelDefinitionID(*root) != id {
		lineage.Root = definitionIDPointer(root)
	}
	parent, found, err := selectDefinitionValue(reader, policies, "Lineage.Parent", candidates, func(model Model) (*string, bool) {
		if model.Lineage == nil || model.Lineage.Parent == nil {
			return nil, false
		}
		return model.Lineage.Parent, true
	})
	if err != nil {
		return lineage, err
	}
	if found && parent != nil && ModelDefinitionID(*parent) != id {
		lineage.Parent = definitionIDPointer(parent)
	}
	return lineage, nil
}

func deriveDefinitionCapabilities(
	reader Reader,
	policies authority.Reader,
	candidates []providerModelCandidate,
) (ModelDefinitionCapabilities, error) {
	capabilities := ModelDefinitionCapabilities{Features: deriveDefinitionFeatures(candidates)}
	var err error
	capabilities.Attachments, _, err = selectDefinitionValue(
		reader, policies, "Attachments", candidates,
		func(model Model) (*ModelAttachments, bool) { return model.Attachments, model.Attachments != nil },
	)
	if err != nil {
		return capabilities, err
	}
	capabilities.Generation, _, err = selectDefinitionValue(
		reader, policies, "Generation", candidates,
		func(model Model) (*ModelGeneration, bool) { return model.Generation, model.Generation != nil },
	)
	if err != nil {
		return capabilities, err
	}
	capabilities.Reasoning, _, err = selectDefinitionValue(
		reader, policies, "Reasoning", candidates,
		func(model Model) (*ModelControlLevels, bool) { return model.Reasoning, model.Reasoning != nil },
	)
	if err != nil {
		return capabilities, err
	}
	capabilities.ReasoningTokens, _, err = selectDefinitionValue(
		reader, policies, "ReasoningTokens", candidates,
		func(model Model) (*IntRange, bool) { return model.ReasoningTokens, model.ReasoningTokens != nil },
	)
	if err != nil {
		return capabilities, err
	}
	capabilities.Verbosity, _, err = selectDefinitionValue(
		reader, policies, "Verbosity", candidates,
		func(model Model) (*ModelControlLevels, bool) { return model.Verbosity, model.Verbosity != nil },
	)
	if err != nil {
		return capabilities, err
	}
	capabilities.Tools, _, err = selectDefinitionValue(
		reader, policies, "Tools", candidates,
		func(model Model) (*ModelTools, bool) { return model.Tools, model.Tools != nil },
	)
	if err != nil {
		return capabilities, err
	}
	capabilities.Delivery, _, err = selectDefinitionValue(
		reader, policies, "Delivery", candidates,
		func(model Model) (*ModelDelivery, bool) { return model.Delivery, model.Delivery != nil },
	)
	if err != nil {
		return capabilities, err
	}
	return capabilities, nil
}

func deriveDefinitionTimestamps(candidates []providerModelCandidate) (utc.Time, utc.Time) {
	var createdAt, updatedAt utc.Time
	for _, candidate := range candidates {
		if !candidate.model.CreatedAt.IsZero() &&
			(createdAt.IsZero() || candidate.model.CreatedAt.Before(createdAt)) {
			createdAt = candidate.model.CreatedAt
		}
		if !candidate.model.UpdatedAt.IsZero() &&
			(updatedAt.IsZero() || candidate.model.UpdatedAt.After(updatedAt)) {
			updatedAt = candidate.model.UpdatedAt
		}
	}
	return createdAt, updatedAt
}

func deriveDefinitionMetadata(
	reader Reader,
	policies authority.Reader,
	candidates []providerModelCandidate,
) (ModelDefinitionMetadata, ModelDefinitionWeights, error) {
	var metadata ModelDefinitionMetadata
	var weights ModelDefinitionWeights

	releaseDate, found, err := selectDefinitionValue(
		reader, policies, "Metadata", candidates,
		func(model Model) (utc.Time, bool) {
			if model.Metadata == nil {
				return utc.Time{}, false
			}
			return model.Metadata.ReleaseDate, !model.Metadata.ReleaseDate.IsZero()
		},
	)
	if err != nil {
		return metadata, weights, err
	}
	if found {
		metadata.ReleaseDate = releaseDate
	}

	knowledgeCutoff, found, err := selectDefinitionValue(
		reader, policies, "Metadata", candidates,
		func(model Model) (*utc.Time, bool) {
			if model.Metadata == nil || model.Metadata.KnowledgeCutoff == nil {
				return nil, false
			}
			return model.Metadata.KnowledgeCutoff, true
		},
	)
	if err != nil {
		return metadata, weights, err
	}
	if found && knowledgeCutoff != nil {
		value := *knowledgeCutoff
		metadata.KnowledgeCutoff = &value
	}

	tagSet := make(map[ModelTag]struct{})
	for _, candidate := range candidates {
		if candidate.model.Metadata == nil {
			continue
		}
		for _, tag := range candidate.model.Metadata.Tags {
			tagSet[tag] = struct{}{}
		}
	}
	metadata.Tags = make([]ModelTag, 0, len(tagSet))
	for tag := range tagSet {
		metadata.Tags = append(metadata.Tags, tag)
	}
	slices.Sort(metadata.Tags)

	openWeights, found, err := selectDefinitionValue(
		reader, policies, "Metadata", candidates,
		func(model Model) (definitionPresenceClaim[bool], bool) {
			value, state := model.Metadata.OpenWeightsValue()
			return definitionPresenceClaim[bool]{Value: value, State: state}, state == ValueKnown
		},
	)
	if err != nil {
		return metadata, weights, err
	}
	if found && openWeights.State == ValueKnown {
		open := openWeights.Value
		weights.Open = &open
	}

	architecture, err := deriveDefinitionArchitecture(reader, policies, candidates)
	if err != nil {
		return metadata, weights, err
	}
	weights.Architecture = architecture
	return metadata, weights, nil
}

func deriveDefinitionArchitecture(
	reader Reader,
	policies authority.Reader,
	candidates []providerModelCandidate,
) (*ModelArchitecture, error) {
	architectures := make([]providerModelCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.model.Metadata != nil && candidate.model.Metadata.Architecture != nil {
			architectures = append(architectures, candidate)
		}
	}
	if len(architectures) == 0 {
		return nil, nil
	}
	result := &ModelArchitecture{}
	var err error
	result.ParameterCount, _, err = selectDefinitionValue(
		reader, policies, "Metadata", architectures,
		func(model Model) (string, bool) {
			architecture := modelArchitecture(model)
			if architecture == nil {
				return "", false
			}
			value := architecture.ParameterCount
			return value, strings.TrimSpace(value) != ""
		},
	)
	if err != nil {
		return nil, err
	}
	result.Type, _, err = selectDefinitionValue(
		reader, policies, "Metadata", architectures,
		func(model Model) (ArchitectureType, bool) {
			architecture := modelArchitecture(model)
			if architecture == nil {
				return "", false
			}
			value := architecture.Type
			return value, value != ""
		},
	)
	if err != nil {
		return nil, err
	}
	result.Tokenizer, _, err = selectDefinitionValue(
		reader, policies, "Metadata", architectures,
		func(model Model) (Tokenizer, bool) {
			architecture := modelArchitecture(model)
			if architecture == nil {
				return "", false
			}
			value := architecture.Tokenizer
			return value, value != ""
		},
	)
	if err != nil {
		return nil, err
	}
	result.Quantization, _, err = selectDefinitionValue(
		reader, policies, "Metadata", architectures,
		func(model Model) (Quantization, bool) {
			architecture := modelArchitecture(model)
			if architecture == nil {
				return "", false
			}
			value := architecture.Quantization
			return value, value != ""
		},
	)
	if err != nil {
		return nil, err
	}
	result.Quantized, _, err = selectDefinitionValue(
		reader, policies, "Metadata", architectures,
		func(model Model) (bool, bool) {
			architecture := modelArchitecture(model)
			if architecture == nil {
				return false, false
			}
			value := architecture.Quantized
			return value, value
		},
	)
	if err != nil {
		return nil, err
	}
	result.FineTuned, _, err = selectDefinitionValue(
		reader, policies, "Metadata", architectures,
		func(model Model) (bool, bool) {
			architecture := modelArchitecture(model)
			if architecture == nil {
				return false, false
			}
			value := architecture.FineTuned
			return value, value
		},
	)
	if err != nil {
		return nil, err
	}
	result.BaseModel, _, err = selectDefinitionValue(
		reader, policies, "Metadata", architectures,
		func(model Model) (*string, bool) {
			architecture := modelArchitecture(model)
			if architecture == nil {
				return nil, false
			}
			value := architecture.BaseModel
			return value, value != nil
		},
	)
	if err != nil {
		return nil, err
	}
	return deepCopyModelArchitecture(result), nil
}

func modelArchitecture(model Model) *ModelArchitecture {
	if model.Metadata == nil {
		return nil
	}
	return model.Metadata.Architecture
}

func deriveDefinitionFeatures(candidates []providerModelCandidate) *ModelFeatures {
	withFeatures := make([]providerModelCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.model.Features != nil {
			withFeatures = append(withFeatures, candidate)
		}
	}
	if len(withFeatures) == 0 {
		return nil
	}

	result := &ModelFeatures{}
	inputModalities := make(map[ModelModality]struct{})
	outputModalities := make(map[ModelModality]struct{})
	for _, candidate := range withFeatures {
		for _, modality := range candidate.model.Features.Modalities.Input {
			inputModalities[modality] = struct{}{}
		}
		for _, modality := range candidate.model.Features.Modalities.Output {
			outputModalities[modality] = struct{}{}
		}
	}
	for modality := range inputModalities {
		result.Modalities.Input = append(result.Modalities.Input, modality)
	}
	for modality := range outputModalities {
		result.Modalities.Output = append(result.Modalities.Output, modality)
	}
	slices.Sort(result.Modalities.Input)
	slices.Sort(result.Modalities.Output)

	for _, feature := range modelFeatures() {
		known := false
		supported := false
		for _, candidate := range withFeatures {
			value, state := candidate.model.Features.Support(feature)
			if state != ValueKnown {
				continue
			}
			known = true
			supported = supported || value
		}
		if known {
			result.SetSupport(feature, supported)
		}
	}
	return result
}

func selectDefinitionValue[T any](
	reader Reader,
	policies authority.Reader,
	field string,
	candidates []providerModelCandidate,
	extract func(Model) (T, bool),
) (T, bool, error) {
	var zero T
	policy, found := policies.Find(catalogmeta.ResourceTypeModel, field)
	if !found {
		return zero, false, &errors.ValidationError{
			Field:   "authority." + field,
			Message: "has no executable model field policy",
		}
	}

	rankedValues := make([]rankedDefinitionValue[T], 0, len(candidates))
	for _, candidate := range candidates {
		value, present := extract(candidate.model)
		if !present {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return zero, false, errors.WrapParse(
				"json",
				"model definition field "+field,
				err,
			)
		}
		evidence, hasEvidence := currentDefinitionEvidence(
			reader,
			candidate.providerID,
			candidate.model,
			policy.Evidence(),
			func(evidenceValue any) bool {
				evidenceModel, decoded := decodeProvenanceModelField(evidenceValue, policy.Path)
				if !decoded {
					return false
				}
				evidenceFieldValue, evidencePresent := extract(evidenceModel)
				if !evidencePresent {
					return false
				}
				evidenceEncoded, marshalErr := json.Marshal(evidenceFieldValue)
				return marshalErr == nil && string(evidenceEncoded) == string(encoded)
			},
		)
		authorityScore := policy.Authority(evidence.Source)
		if hasEvidence && authorityScore == 0 {
			evidence = provenance.Entry{}
			hasEvidence = false
		}
		rankedValues = append(rankedValues, rankedDefinitionValue[T]{
			value:       value,
			providerID:  candidate.providerID,
			evidence:    evidence,
			hasEvidence: hasEvidence,
			authority:   authorityScore,
			recordTime:  candidate.model.UpdatedAt,
			semantic:    string(encoded),
		})
	}
	if len(rankedValues) == 0 {
		return zero, false, nil
	}
	slices.SortFunc(rankedValues, compareDefinitionValueRank[T])
	selected := rankedValues[0]
	for _, contender := range rankedValues[1:] {
		if compareDefinitionEvidenceRank(contender, selected) != 0 {
			break
		}
		if contender.semantic != selected.semantic {
			// No provider-independent fact is safer than choosing by provider
			// name, map order, or serialized value. Reconciliation can later
			// attach decisive authority evidence; until then the read view
			// leaves this field unknown.
			return zero, false, nil
		}
	}
	return selected.value, true, nil
}

func currentDefinitionEvidence(
	reader Reader,
	providerID ProviderID,
	model Model,
	field string,
	matches func(any) bool,
) (provenance.Entry, bool) {
	entries := matchingProvenanceValues(reader.Provenance().FindModelField(providerID, model.ID, field), matches)
	if len(entries) == 0 {
		bareEntries := reader.Provenance().FindByField(catalogmeta.ResourceTypeModel, model.ID, field)
		entries = matchingProvenanceValues(bareEntries, matches)
	}
	if len(entries) == 0 {
		return provenance.Entry{}, false
	}
	selected := entries[0]
	for _, candidate := range entries[1:] {
		if candidate.Timestamp.After(selected.Timestamp) ||
			(candidate.Timestamp.Equal(selected.Timestamp) && candidate.ObservedAt.After(selected.ObservedAt)) {
			selected = candidate
		}
	}
	return selected, true
}

func matchingProvenanceValues(entries []provenance.Entry, matches func(any) bool) []provenance.Entry {
	matched := make([]provenance.Entry, 0, len(entries))
	for _, entry := range entries {
		if matches(entry.Value) {
			matched = append(matched, entry)
		}
	}
	return matched
}

func decodeProvenanceModelField(value any, field string) (Model, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return Model{}, false
	}
	var model Model
	switch field {
	case "Name":
		if err := json.Unmarshal(encoded, &model.Name); err != nil {
			return Model{}, false
		}
	case "Description":
		var description string
		if err := json.Unmarshal(encoded, &description); err != nil {
			return Model{}, false
		}
		model.SetDescription(description)
	case "Metadata":
		if err := json.Unmarshal(encoded, &model.Metadata); err != nil {
			return Model{}, false
		}
	case "Lineage.Family":
		model.Lineage = &ModelLineage{}
		if err := json.Unmarshal(encoded, &model.Lineage.Family); err != nil {
			return Model{}, false
		}
	case "Lineage.Root":
		model.Lineage = &ModelLineage{}
		if err := json.Unmarshal(encoded, &model.Lineage.Root); err != nil {
			return Model{}, false
		}
	case "Lineage.Parent":
		model.Lineage = &ModelLineage{}
		if err := json.Unmarshal(encoded, &model.Lineage.Parent); err != nil {
			return Model{}, false
		}
	case "Features":
		if err := json.Unmarshal(encoded, &model.Features); err != nil {
			return Model{}, false
		}
	case "Attachments":
		if err := json.Unmarshal(encoded, &model.Attachments); err != nil {
			return Model{}, false
		}
	case "Generation":
		if err := json.Unmarshal(encoded, &model.Generation); err != nil {
			return Model{}, false
		}
	case "Reasoning":
		if err := json.Unmarshal(encoded, &model.Reasoning); err != nil {
			return Model{}, false
		}
	case "ReasoningTokens":
		if err := json.Unmarshal(encoded, &model.ReasoningTokens); err != nil {
			return Model{}, false
		}
	case "Verbosity":
		if err := json.Unmarshal(encoded, &model.Verbosity); err != nil {
			return Model{}, false
		}
	case "Tools":
		if err := json.Unmarshal(encoded, &model.Tools); err != nil {
			return Model{}, false
		}
	case "Delivery":
		if err := json.Unmarshal(encoded, &model.Delivery); err != nil {
			return Model{}, false
		}
	default:
		return Model{}, false
	}
	return model, true
}

func compareDefinitionValueRank[T any](left, right rankedDefinitionValue[T]) int {
	if rank := compareDefinitionEvidenceRank(left, right); rank != 0 {
		return rank
	}
	if left.semantic != right.semantic {
		return strings.Compare(left.semantic, right.semantic)
	}
	return strings.Compare(string(left.providerID), string(right.providerID))
}

func compareDefinitionEvidenceRank[T any](left, right rankedDefinitionValue[T]) int {
	if left.hasEvidence != right.hasEvidence {
		if left.hasEvidence {
			return -1
		}
		return 1
	}
	if left.authority != right.authority {
		if left.authority > right.authority {
			return -1
		}
		return 1
	}
	if !left.evidence.ObservedAt.Equal(right.evidence.ObservedAt) {
		if left.evidence.ObservedAt.After(right.evidence.ObservedAt) {
			return -1
		}
		return 1
	}
	if !left.evidence.Timestamp.Equal(right.evidence.Timestamp) {
		if left.evidence.Timestamp.After(right.evidence.Timestamp) {
			return -1
		}
		return 1
	}
	if left.evidence.Confidence != right.evidence.Confidence {
		if left.evidence.Confidence > right.evidence.Confidence {
			return -1
		}
		return 1
	}
	if !left.recordTime.Equal(right.recordTime) {
		if left.recordTime.After(right.recordTime) {
			return -1
		}
		return 1
	}
	return 0
}

func modelLineageFamily(model Model) (string, bool) {
	if model.Lineage == nil || strings.TrimSpace(model.Lineage.Family) == "" {
		return "", false
	}
	return model.Lineage.Family, true
}

func definitionIDPointer(value *string) *ModelDefinitionID {
	if value == nil {
		return nil
	}
	converted := ModelDefinitionID(*value)
	return &converted
}
