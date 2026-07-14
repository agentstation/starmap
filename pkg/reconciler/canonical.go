package reconciler

import (
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type definitionCandidates map[sources.ID]catalogs.ModelDefinition
type offeringCandidates map[sources.ID]catalogs.ProviderOffering

func (r *Reconciler) reconcileCanonical(rctx *reconcileContext, output *catalogs.Builder) error {
	definitions := make(map[catalogs.ModelDefinitionID]definitionCandidates)
	offerings := make(map[catalogs.OfferingKey]offeringCandidates)
	if rctx.baseline != nil {
		collectCanonicalCandidates(sources.LocalCatalogID, rctx.baseline, definitions, offerings)
	}
	for _, observation := range rctx.collector.sources {
		if observation.Catalog == nil {
			continue
		}
		collectCanonicalCandidates(observation.SourceID, observation.Catalog, definitions, offerings)
	}
	for _, key := range applyCompleteOfferingDeletions(rctx.collector.sources, offerings) {
		output.DeleteOffering(key.ProviderID, key.ProviderModelID)
		_ = output.DeleteProviderModel(key.ProviderID, string(key.ProviderModelID))
	}

	definitionIDs := make([]catalogs.ModelDefinitionID, 0, len(definitions))
	for id := range definitions {
		definitionIDs = append(definitionIDs, id)
	}
	slices.Sort(definitionIDs)
	for _, id := range definitionIDs {
		definition, err := mergeDefinition(id, definitions[id])
		if err != nil {
			return errors.WrapResource("reconcile", "model definition", string(id), err)
		}
		if err := output.SetDefinition(definition); err != nil {
			return errors.WrapResource("set", "model definition", string(id), err)
		}
	}

	keys := make([]catalogs.OfferingKey, 0, len(offerings))
	for key := range offerings {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(left, right catalogs.OfferingKey) int {
		if order := strings.Compare(string(left.ProviderID), string(right.ProviderID)); order != 0 {
			return order
		}
		return strings.Compare(string(left.ProviderModelID), string(right.ProviderModelID))
	})
	for _, key := range keys {
		offering, err := mergeOffering(key, offerings[key], rctx.merger.pricingAt, rctx.merger.observations)
		if err != nil {
			return errors.WrapResource("reconcile", "provider offering", string(key.ProviderID)+"/"+string(key.ProviderModelID), err)
		}
		if _, found := definitions[offering.DefinitionID]; !found {
			return &errors.NotFoundError{Resource: "model definition", ID: string(offering.DefinitionID)}
		}
		if err := output.SetOffering(offering); err != nil {
			return errors.WrapResource("set", "provider offering", string(key.ProviderID)+"/"+string(key.ProviderModelID), err)
		}
	}
	return nil
}

func collectCanonicalCandidates(sourceID sources.ID, catalog *catalogs.Catalog, definitions map[catalogs.ModelDefinitionID]definitionCandidates, offerings map[catalogs.OfferingKey]offeringCandidates) {
	for _, definition := range catalog.Definitions() {
		if definitions[definition.ID] == nil {
			definitions[definition.ID] = make(definitionCandidates)
		}
		definitions[definition.ID][sourceID] = definition
	}
	for _, offering := range catalog.Offerings() {
		if offerings[offering.Key()] == nil {
			offerings[offering.Key()] = make(offeringCandidates)
		}
		offerings[offering.Key()][sourceID] = offering
	}
}

func mergeDefinition(id catalogs.ModelDefinitionID, candidates definitionCandidates) (catalogs.ModelDefinition, error) {
	ordered := orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, "Name")
	if len(ordered) == 0 {
		return catalogs.ModelDefinition{}, &errors.NotFoundError{Resource: "model definition", ID: string(id)}
	}
	merged := ordered[0]
	merged.ID = id
	merged.Name = firstDefinitionValue(candidates, "Name", func(value catalogs.ModelDefinition) string { return value.Name })
	merged.AuthorIDs = unionAuthorIDs(orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, "AuthorIDs"))
	merged.Description = firstDefinitionValue(candidates, "Description", func(value catalogs.ModelDefinition) string { return value.Description })
	merged.Metadata = catalogs.ModelDefinitionMetadata{}
	for _, candidate := range orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, "Metadata") {
		if err := fillMissing(&merged.Metadata, candidate.Metadata); err != nil {
			return catalogs.ModelDefinition{}, err
		}
	}
	merged.Lineage = catalogs.ModelDefinitionLineage{}
	for _, candidate := range orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, "Lineage") {
		if err := fillMissing(&merged.Lineage, candidate.Lineage); err != nil {
			return catalogs.ModelDefinition{}, err
		}
	}
	weights := orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, "Weights.Open")
	if len(weights) > 0 {
		merged.Weights.Open = weights[0].Weights.Open
	}
	merged.Weights.Architecture = nil
	for _, candidate := range orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, "Weights.Architecture") {
		if merged.Weights.Architecture == nil && candidate.Weights.Architecture != nil {
			architecture := *candidate.Weights.Architecture
			merged.Weights.Architecture = &architecture
		}
	}
	capabilities, err := mergeDefinitionCapabilities(orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, "Capabilities"))
	if err != nil {
		return catalogs.ModelDefinition{}, err
	}
	merged.Capabilities = capabilities
	created := orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, "CreatedAt")
	for _, candidate := range created {
		if !candidate.CreatedAt.IsZero() {
			merged.CreatedAt = candidate.CreatedAt
			break
		}
	}
	updated := orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, "UpdatedAt")
	for _, candidate := range updated {
		if !candidate.UpdatedAt.IsZero() {
			merged.UpdatedAt = candidate.UpdatedAt
			break
		}
	}
	return merged, merged.Validate()
}

func mergeOffering(key catalogs.OfferingKey, candidates offeringCandidates, pricingAt time.Time, observations map[sources.ID]sourceObservationEvidence) (catalogs.ProviderOffering, error) {
	identity := orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, "DefinitionID")
	if len(identity) == 0 {
		return catalogs.ProviderOffering{}, &errors.NotFoundError{Resource: "provider offering", ID: string(key.ProviderID) + "/" + string(key.ProviderModelID)}
	}
	definitionID := identity[0].DefinitionID
	for _, candidate := range identity[1:] {
		if candidate.DefinitionID != definitionID {
			return catalogs.ProviderOffering{}, &errors.ConflictError{Resource: "provider offering definition", Expected: string(definitionID), Actual: string(candidate.DefinitionID), Message: "immutable offering identity resolved to different definitions"}
		}
	}
	merged := identity[0]
	merged.ProviderID = key.ProviderID
	merged.ProviderModelID = key.ProviderModelID
	merged.DeploymentID = key.DeploymentID
	merged.DefinitionID = definitionID
	merged.Aliases = unionOfferingAliases(orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, "Aliases"))
	merged.Pricing = firstValidPricing(candidates, pricingAt, observations)
	limits, err := mergeLimits(orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, "Limits"))
	if err != nil {
		return catalogs.ProviderOffering{}, err
	}
	merged.Limits = limits
	merged.Availability = firstOfferingValue(candidates, "Availability", func(value catalogs.ProviderOffering) catalogs.OfferingAvailability { return value.Availability })
	merged.Access = firstOfferingStruct(candidates, "Access", func(value catalogs.ProviderOffering) catalogs.OfferingAccess { return value.Access })
	merged.Regions = unionOfferingRegions(orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, "Regions"))
	merged.Deployment = firstOfferingStruct(candidates, "Deployment", func(value catalogs.ProviderOffering) catalogs.ProviderDeployment { return value.Deployment })
	merged.InferenceProfile = firstNonNilProfile(orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, "InferenceProfile"))
	merged.AggregatorUpstream = firstNonNilUpstream(orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, "AggregatorUpstream"))
	merged.Endpoint = catalogs.ProviderOfferingEndpoint{}
	for _, candidate := range orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, "Endpoint") {
		if err := fillMissing(&merged.Endpoint, candidate.Endpoint); err != nil {
			return catalogs.ProviderOffering{}, err
		}
	}
	merged.Lifecycle = firstOfferingValue(candidates, "Lifecycle", func(value catalogs.ProviderOffering) catalogs.OfferingLifecycle { return value.Lifecycle })
	merged.Modes = mergeModes(orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, "Modes"))
	return merged, merged.Validate()
}

func orderedDefinitionCandidates(candidates definitionCandidates, resource sources.ResourceType, path string) []catalogs.ModelDefinition {
	ids := orderedCandidateSources(resource, path, mapKeysDefinition(candidates))
	result := make([]catalogs.ModelDefinition, 0, len(ids))
	for _, id := range ids {
		result = append(result, candidates[id])
	}
	return result
}

func orderedOfferingCandidates(candidates offeringCandidates, resource sources.ResourceType, path string) []catalogs.ProviderOffering {
	ids := orderedCandidateSources(resource, path, mapKeysOffering(candidates))
	result := make([]catalogs.ProviderOffering, 0, len(ids))
	for _, id := range ids {
		result = append(result, candidates[id])
	}
	return result
}

func orderedCandidateSources(resource sources.ResourceType, path string, available []sources.ID) []sources.ID {
	policy, _ := authority.FindCanonicalPolicy(resource, path)
	result := make([]sources.ID, 0, len(available))
	for _, id := range policy.AuthorityOrder {
		if slices.Contains(available, id) {
			result = append(result, id)
		}
	}
	slices.Sort(available)
	for _, id := range available {
		if !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	return result
}

func mapKeysDefinition(values definitionCandidates) []sources.ID {
	result := make([]sources.ID, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}

func mapKeysOffering(values offeringCandidates) []sources.ID {
	result := make([]sources.ID, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}

func firstDefinitionValue(candidates definitionCandidates, path string, value func(catalogs.ModelDefinition) string) string {
	for _, candidate := range orderedDefinitionCandidates(candidates, sources.ResourceTypeModelDefinition, path) {
		if result := value(candidate); result != "" {
			return result
		}
	}
	return ""
}

func firstOfferingValue[T comparable](candidates offeringCandidates, path string, value func(catalogs.ProviderOffering) T) T {
	var zero T
	for _, candidate := range orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, path) {
		if result := value(candidate); result != zero {
			return result
		}
	}
	return zero
}

func firstOfferingStruct[T any](candidates offeringCandidates, path string, value func(catalogs.ProviderOffering) T) T {
	ordered := orderedOfferingCandidates(candidates, sources.ResourceTypeProviderOffering, path)
	if len(ordered) == 0 {
		var zero T
		return zero
	}
	return value(ordered[0])
}

func unionAuthorIDs(candidates []catalogs.ModelDefinition) []catalogs.AuthorID {
	result := make([]catalogs.AuthorID, 0)
	for _, candidate := range candidates {
		for _, id := range candidate.AuthorIDs {
			if !slices.Contains(result, id) {
				result = append(result, id)
			}
		}
	}
	return result
}

func mergeDefinitionCapabilities(candidates []catalogs.ModelDefinition) (catalogs.ModelDefinitionCapabilities, error) {
	var merged catalogs.ModelDefinitionCapabilities
	for _, candidate := range candidates {
		value := candidate.Capabilities
		if merged.Features == nil && value.Features != nil {
			featuresCopy := *value.Features
			merged.Features = &featuresCopy
		}
		value.Features = nil
		if err := fillMissing(&merged, value); err != nil {
			return catalogs.ModelDefinitionCapabilities{}, err
		}
	}
	return merged, nil
}

func firstValidPricing(candidates offeringCandidates, at time.Time, observations map[sources.ID]sourceObservationEvidence) *catalogs.ModelPricing {
	for _, sourceID := range orderedCandidateSources(sources.ResourceTypeProviderOffering, "Pricing", mapKeysOffering(candidates)) {
		if evidence, found := observations[sourceID]; found && observationPricingDegraded(evidence) {
			continue
		}
		candidate := candidates[sourceID]
		if candidate.Pricing != nil && candidate.Pricing.Validate() == nil && candidate.Pricing.IsEffectiveAt(at) {
			pricingCopy := *candidate.Pricing
			return &pricingCopy
		}
	}
	return nil
}

func observationPricingDegraded(evidence sourceObservationEvidence) bool {
	if evidence.completeness == sources.ObservationCompletenessPartial || evidence.status == sources.ObservationStatusDegraded {
		return true
	}
	for _, issue := range evidence.issues {
		if issue.Code == sources.ObservationIssueCodeStaleFallback || issue.Code == sources.ObservationIssueCodeInvalidRecord || issue.Code == sources.ObservationIssueCodeSchemaDrift {
			return true
		}
	}
	return false
}

func applyCompleteOfferingDeletions(observations []sources.Observation, offerings map[catalogs.OfferingKey]offeringCandidates) []catalogs.OfferingKey {
	deleted := make(map[catalogs.OfferingKey]struct{})
	for _, observation := range observations {
		if observation.Catalog == nil || observation.Completeness != sources.ObservationCompletenessComplete || observation.Status != sources.ObservationStatusSucceeded ||
			!sourceCanDeleteCatalogBaseline(observation.SourceID) {
			continue
		}
		providerIDs := make(map[catalogs.ProviderID]struct{})
		for _, provider := range observation.Catalog.Providers().List() {
			providerIDs[provider.ID] = struct{}{}
		}
		observed := make(map[catalogs.OfferingKey]struct{})
		for _, offering := range observation.Catalog.Offerings() {
			observed[offering.Key()] = struct{}{}
		}
		for key, candidates := range offerings {
			if _, inScope := providerIDs[key.ProviderID]; !inScope {
				continue
			}
			if _, found := observed[key]; found {
				continue
			}
			delete(candidates, sources.LocalCatalogID)
			if len(candidates) == 0 {
				delete(offerings, key)
				deleted[key] = struct{}{}
			}
		}
	}
	keys := make([]catalogs.OfferingKey, 0, len(deleted))
	for key := range deleted {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(left, right catalogs.OfferingKey) int {
		if order := strings.Compare(string(left.ProviderID), string(right.ProviderID)); order != 0 {
			return order
		}
		if order := strings.Compare(string(left.ProviderModelID), string(right.ProviderModelID)); order != 0 {
			return order
		}
		return strings.Compare(left.DeploymentID, right.DeploymentID)
	})
	return keys
}

func unionOfferingAliases(candidates []catalogs.ProviderOffering) []string {
	var result []string
	for _, candidate := range candidates {
		for _, alias := range candidate.Aliases {
			if alias != "" && !slices.Contains(result, alias) {
				result = append(result, alias)
			}
		}
	}
	slices.Sort(result)
	return result
}

func sourceCanDeleteCatalogBaseline(sourceID sources.ID) bool {
	if sourceID == sources.LocalCatalogID || sourceID == sources.ModelsDevHTTPID || sourceID == sources.ModelsDevGitID {
		return false
	}
	policy, found := authority.FindCanonicalPolicy(sources.ResourceTypeProviderOffering, "Availability")
	if !found {
		return false
	}
	sourceIndex := slices.Index(policy.AuthorityOrder, sourceID)
	baselineIndex := slices.Index(policy.AuthorityOrder, sources.LocalCatalogID)
	return sourceIndex >= 0 && baselineIndex >= 0 && sourceIndex < baselineIndex
}

func mergeLimits(candidates []catalogs.ProviderOffering) (*catalogs.ModelLimits, error) {
	var merged *catalogs.ModelLimits
	for _, candidate := range candidates {
		if candidate.Limits == nil {
			continue
		}
		if merged == nil {
			limitsCopy := *candidate.Limits
			merged = &limitsCopy
			continue
		}
		if err := fillMissing(merged, *candidate.Limits); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

func unionOfferingRegions(candidates []catalogs.ProviderOffering) []catalogs.CloudRegion {
	result := make([]catalogs.CloudRegion, 0)
	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		for _, region := range candidate.Regions {
			if _, found := seen[region.ID]; found {
				continue
			}
			seen[region.ID] = struct{}{}
			result = append(result, region)
		}
	}
	return result
}

func firstNonNilProfile(candidates []catalogs.ProviderOffering) *catalogs.CrossRegionInferenceProfile {
	for _, candidate := range candidates {
		if candidate.InferenceProfile != nil {
			profileCopy := *candidate.InferenceProfile
			profileCopy.SourceRegions = slices.Clone(candidate.InferenceProfile.SourceRegions)
			profileCopy.DestinationRegions = slices.Clone(candidate.InferenceProfile.DestinationRegions)
			return &profileCopy
		}
	}
	return nil
}

func firstNonNilUpstream(candidates []catalogs.ProviderOffering) *catalogs.AggregatorUpstream {
	for _, candidate := range candidates {
		if candidate.AggregatorUpstream != nil {
			upstreamCopy := *candidate.AggregatorUpstream
			return &upstreamCopy
		}
	}
	return nil
}

func mergeModes(candidates []catalogs.ProviderOffering) map[string]catalogs.ProviderOfferingMode {
	result := make(map[string]catalogs.ProviderOfferingMode)
	for _, candidate := range candidates {
		names := make([]string, 0, len(candidate.Modes))
		for name := range candidate.Modes {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if _, found := result[name]; !found {
				result[name] = candidate.Modes[name]
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func fillMissing(destination any, source any) error {
	dst := reflect.ValueOf(destination)
	if dst.Kind() != reflect.Pointer || dst.IsNil() {
		return &errors.ValidationError{Field: "canonical_merge.destination", Value: destination, Message: "must be a non-nil pointer"}
	}
	src := reflect.ValueOf(source)
	if !src.IsValid() || dst.Elem().Type() != src.Type() {
		return &errors.ValidationError{Field: "canonical_merge.source", Value: source, Message: "must match the destination element type"}
	}
	fillMissingValue(dst.Elem(), src)
	return nil
}

func fillMissingValue(destination, source reflect.Value) {
	if !source.IsValid() || destination.Type() != source.Type() {
		return
	}
	if destination.Kind() == reflect.Struct {
		for index := range destination.NumField() {
			if destination.Field(index).CanSet() {
				fillMissingValue(destination.Field(index), source.Field(index))
			}
		}
		return
	}
	if destination.Kind() == reflect.Pointer {
		if destination.IsNil() && !source.IsNil() {
			destination.Set(reflect.ValueOf(deepCopyReflect(source.Interface())))
		} else if !destination.IsNil() && !source.IsNil() {
			fillMissingValue(destination.Elem(), source.Elem())
		}
		return
	}
	if destination.IsZero() && !source.IsZero() {
		destination.Set(source)
	}
}

func deepCopyReflect(value any) any {
	// Canonical setters perform the authoritative deep copy. This helper only
	// allocates a distinct pointer shell while composing temporary values.
	reflected := reflect.ValueOf(value)
	if reflected.Kind() != reflect.Pointer || reflected.IsNil() {
		return value
	}
	valueCopy := reflect.New(reflected.Elem().Type())
	valueCopy.Elem().Set(reflected.Elem())
	return valueCopy.Interface()
}
