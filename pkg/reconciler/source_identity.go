package reconciler

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

type evidenceLocator struct {
	resource catalogmeta.ResourceType
	id       string
	field    string
	source   sources.ID
}

type modelIdentity struct {
	providerID catalogs.ProviderID
	modelID    string
}

func (identity modelIdentity) resourceID() string {
	return provenance.ModelResourceID(string(identity.providerID), identity.modelID)
}

func (merger *merger) modelSourcesForPolicy(
	providerID catalogs.ProviderID,
	modelID string,
	policy authority.Policy,
	sourceModels map[sources.ID]*catalogs.Model,
) map[sources.ID]*catalogs.Model {
	return merger.modelSourcesForValue(
		providerID,
		modelID,
		policy,
		sourceModels,
		func(model *catalogs.Model) any {
			return merger.modelFieldValue(model, policy.Path)
		},
	)
}

func (merger *merger) modelSourcesForValue(
	providerID catalogs.ProviderID,
	modelID string,
	policy authority.Policy,
	sourceModels map[sources.ID]*catalogs.Model,
	value func(*catalogs.Model) any,
) map[sources.ID]*catalogs.Model {
	sourceModels = merger.suppressStaleModelFallback(providerID, modelID, sourceModels, value)
	localModel := sourceModels[sources.LocalCatalogID]
	if localModel == nil {
		return sourceModels
	}
	localValue := value(localModel)
	if localValue == nil {
		return sourceModels
	}
	evidence, ok := merger.projectedModelEvidence(providerID, modelID, policy.Evidence(), localValue)
	if !ok || !slices.Contains(policy.SourceOrder, evidence.Source) {
		return sourceModels
	}

	resolved := cloneModelSources(sourceModels)
	if evidence.Source == sources.LocalCatalogID {
		merger.rememberCarriedEvidence(
			catalogmeta.ResourceTypeModel,
			provenance.ModelResourceID(string(providerID), modelID),
			policy.Evidence(),
			evidence,
		)
		return resolved
	}

	if current := resolved[evidence.Source]; current != nil && value(current) != nil {
		delete(resolved, sources.LocalCatalogID)
		return resolved
	}
	delete(resolved, sources.LocalCatalogID)
	resolved[evidence.Source] = localModel
	merger.rememberCarriedEvidence(
		catalogmeta.ResourceTypeModel,
		provenance.ModelResourceID(string(providerID), modelID),
		policy.Evidence(),
		evidence,
	)
	return resolved
}

func (merger *merger) suppressStaleModelFallback(
	providerID catalogs.ProviderID,
	modelID string,
	sourceModels map[sources.ID]*catalogs.Model,
	value func(*catalogs.Model) any,
) map[sources.ID]*catalogs.Model {
	baseline := merger.baselineModel(providerID, modelID)
	if baseline == nil || value(baseline) == nil {
		return sourceModels
	}
	resolved := sourceModels
	for source := range sourceModels {
		if !merger.observationIsStaleFallback(source) {
			continue
		}
		if len(resolved) == len(sourceModels) {
			resolved = cloneModelSources(sourceModels)
		}
		delete(resolved, source)
	}
	return resolved
}

func (merger *merger) providerSourcesForPolicy(
	providerID catalogs.ProviderID,
	policy authority.Policy,
	sourceProviders map[sources.ID]*catalogs.Provider,
) map[sources.ID]*catalogs.Provider {
	sourceProviders = merger.suppressStaleProviderFallback(providerID, policy, sourceProviders)
	localProvider := sourceProviders[sources.LocalCatalogID]
	if localProvider == nil {
		return sourceProviders
	}
	localValue := merger.providerFieldValue(*localProvider, policy.Path)
	if localValue == nil {
		return sourceProviders
	}
	evidence, ok := merger.projectedProviderEvidence(providerID, policy.Evidence(), localValue)
	if !ok || !slices.Contains(policy.SourceOrder, evidence.Source) {
		return sourceProviders
	}

	resolved := cloneProviderSources(sourceProviders)
	if evidence.Source == sources.LocalCatalogID {
		merger.rememberCarriedEvidence(
			catalogmeta.ResourceTypeProvider,
			string(providerID),
			policy.Evidence(),
			evidence,
		)
		return resolved
	}

	if current := resolved[evidence.Source]; current != nil &&
		merger.providerFieldValue(*current, policy.Path) != nil {
		delete(resolved, sources.LocalCatalogID)
		return resolved
	}
	delete(resolved, sources.LocalCatalogID)
	resolved[evidence.Source] = localProvider
	merger.rememberCarriedEvidence(
		catalogmeta.ResourceTypeProvider,
		string(providerID),
		policy.Evidence(),
		evidence,
	)
	return resolved
}

func (merger *merger) suppressStaleProviderFallback(
	providerID catalogs.ProviderID,
	policy authority.Policy,
	sourceProviders map[sources.ID]*catalogs.Provider,
) map[sources.ID]*catalogs.Provider {
	if merger.baseline == nil {
		return sourceProviders
	}
	baseline, err := merger.baseline.Provider(providerID)
	if err != nil || merger.providerFieldValue(baseline, policy.Path) == nil {
		return sourceProviders
	}
	resolved := sourceProviders
	for source := range sourceProviders {
		if !merger.observationIsStaleFallback(source) {
			continue
		}
		if len(resolved) == len(sourceProviders) {
			resolved = cloneProviderSources(sourceProviders)
		}
		delete(resolved, source)
	}
	return resolved
}

func (merger *merger) observationIsStaleFallback(source sources.ID) bool {
	evidence, exists := merger.observations[source]
	if !exists || evidence.status != sources.ObservationStatusDegraded {
		return false
	}
	for _, issue := range evidence.issues {
		if issue.Scope == sources.ObservationIssueScopeStaleFallback ||
			issue.Code == sources.ObservationIssueCodeStaleFallback ||
			issue.Code == sources.ObservationIssueCodeBootstrapFallback {
			return true
		}
	}
	return false
}

func (merger *merger) projectedModelEvidence(
	providerID catalogs.ProviderID,
	modelID, field string,
	value any,
) (provenance.Entry, bool) {
	catalog := merger.sourceCatalogs[sources.LocalCatalogID]
	if catalog == nil {
		return provenance.Entry{}, false
	}
	entries := catalog.Provenance().FindModelField(providerID, modelID, field)
	if len(entries) == 0 && modelIDIsUnique(catalog, modelID) {
		entries = catalog.Provenance().FindByField(catalogmeta.ResourceTypeModel, modelID, field)
	}
	return matchingCurrentEvidence(entries, value)
}

func (merger *merger) projectedProviderEvidence(
	providerID catalogs.ProviderID,
	field string,
	value any,
) (provenance.Entry, bool) {
	catalog := merger.sourceCatalogs[sources.LocalCatalogID]
	if catalog == nil {
		return provenance.Entry{}, false
	}
	return matchingCurrentEvidence(
		catalog.Provenance().FindByField(catalogmeta.ResourceTypeProvider, string(providerID), field),
		value,
	)
}

func matchingCurrentEvidence(entries []provenance.Entry, value any) (provenance.Entry, bool) {
	if len(entries) == 0 {
		return provenance.Entry{}, false
	}
	current := entries[0]
	for _, entry := range entries[1:] {
		if entry.Timestamp.After(current.Timestamp) {
			current = entry
		}
	}
	if current.Source == "" || !semanticValueEqual(current.Value, value) {
		return provenance.Entry{}, false
	}
	return current, true
}

func semanticValueEqual(left, right any) bool {
	leftValue, leftErr := normalizedSemanticValue(left)
	rightValue, rightErr := normalizedSemanticValue(right)
	return leftErr == nil && rightErr == nil && reflect.DeepEqual(leftValue, rightValue)
}

func normalizedSemanticValue(value any) (any, error) {
	yamlData, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}
	var yamlValue any
	if err := yaml.Unmarshal(yamlData, &yamlValue); err != nil {
		return nil, err
	}
	jsonData, err := json.Marshal(yamlValue)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(jsonData))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func modelIDIsUnique(catalog catalogs.Reader, modelID string) bool {
	count := 0
	for _, provider := range catalog.Providers().List() {
		if _, exists := provider.Models[modelID]; !exists {
			continue
		}
		count++
		if count > 1 {
			return false
		}
	}
	return count == 1
}

func (merger *merger) rememberCarriedEvidence(
	resource catalogmeta.ResourceType,
	resourceID, field string,
	evidence provenance.Entry,
) {
	if merger.carriedEvidence == nil {
		merger.carriedEvidence = make(map[evidenceLocator]provenance.Entry)
	}
	evidence.Rejections = append([]provenance.Rejection(nil), evidence.Rejections...)
	merger.carriedEvidence[evidenceLocator{
		resource: resource,
		id:       resourceID,
		field:    field,
		source:   evidence.Source,
	}] = evidence
}

func (merger *merger) carried(
	resource catalogmeta.ResourceType,
	resourceID, field string,
	source sources.ID,
) (provenance.Entry, bool) {
	evidence, ok := merger.carriedEvidence[evidenceLocator{
		resource: resource,
		id:       resourceID,
		field:    field,
		source:   source,
	}]
	return evidence, ok
}

func cloneModelSources(source map[sources.ID]*catalogs.Model) map[sources.ID]*catalogs.Model {
	result := make(map[sources.ID]*catalogs.Model, len(source))
	for id, model := range source {
		result[id] = model
	}
	return result
}

func cloneProviderSources(source map[sources.ID]*catalogs.Provider) map[sources.ID]*catalogs.Provider {
	result := make(map[sources.ID]*catalogs.Provider, len(source))
	for id, provider := range source {
		result[id] = provider
	}
	return result
}
