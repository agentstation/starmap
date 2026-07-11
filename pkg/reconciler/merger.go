package reconciler

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// merger implements strategic three-way merge.
type merger struct {
	authorities    authority.Authority
	strategy       Strategy
	tracker        provenance.Tracker
	baseline       *catalogs.Catalog // Baseline catalog for timestamp preservation
	baselineModels map[catalogs.ProviderID]map[string]*catalogs.Model
	pricingAt      time.Time
	observations   map[sources.ID]sourceObservationEvidence
}

type sourceObservationEvidence struct {
	id               string
	observedAt       time.Time
	revision         sources.Revision
	evidenceChecksum string
}

// newMerger creates a new strategic merger.
func newMerger(authorities authority.Authority, strategy Strategy, baseline *catalogs.Catalog) *merger {
	return &merger{
		authorities:    authorities,
		strategy:       strategy,
		baseline:       baseline,
		baselineModels: indexBaselineModels(baseline),
		pricingAt:      time.Now().UTC(),
	}
}

func (merger *merger) setObservations(observations []sources.Observation) {
	merger.observations = make(map[sources.ID]sourceObservationEvidence, len(observations))
	for _, observation := range observations {
		merger.observations[observation.SourceID] = sourceObservationEvidence{
			id:               observation.ID,
			observedAt:       observation.ObservedAt,
			revision:         observation.Revision,
			evidenceChecksum: observation.EvidenceChecksum,
		}
	}
}

// newMergerWithProvenance creates a new strategic merger with provenance tracking.
func newMergerWithProvenance(authorities authority.Authority, strategy Strategy, tracker provenance.Tracker, baseline *catalogs.Catalog) *merger {
	return &merger{
		authorities:    authorities,
		strategy:       strategy,
		tracker:        tracker,
		baseline:       baseline,
		baselineModels: indexBaselineModels(baseline),
		pricingAt:      time.Now().UTC(),
	}
}

func indexBaselineModels(baseline *catalogs.Catalog) map[catalogs.ProviderID]map[string]*catalogs.Model {
	if baseline == nil {
		return nil
	}
	providers := baseline.Providers().List()
	models := make(map[catalogs.ProviderID]map[string]*catalogs.Model, len(providers))
	for _, provider := range providers {
		providerModels := make(map[string]*catalogs.Model, len(provider.Models)*2)
		for id, model := range provider.Models {
			if model == nil {
				continue
			}
			modelCopy := catalogs.DeepCopyModel(*model)
			providerModels[modelCopy.ID] = &modelCopy
			if id != "" && id != modelCopy.ID {
				providerModels[id] = &modelCopy
			}
		}
		if len(providerModels) > 0 {
			models[provider.ID] = providerModels
		}
	}
	return models
}

func (merger *merger) baselineModel(providerID catalogs.ProviderID, modelID string) *catalogs.Model {
	if merger.baselineModels == nil {
		return nil
	}
	if providerID != "" {
		model, ok := merger.baselineModels[providerID][modelID]
		return copyBaselineModel(model, ok)
	}

	var found *catalogs.Model
	for _, providerModels := range merger.baselineModels {
		model, ok := providerModels[modelID]
		if !ok || model == nil {
			continue
		}
		if found != nil {
			return nil
		}
		found = model
	}
	return copyBaselineModel(found, found != nil)
}

func copyBaselineModel(model *catalogs.Model, ok bool) *catalogs.Model {
	if !ok || model == nil {
		return nil
	}
	modelCopy := catalogs.DeepCopyModel(*model)
	return &modelCopy
}

// calculateAuthorityScore converts priority to a 0-1.0 authority score.
// Higher priority = higher authority score.
func (merger *merger) calculateAuthorityScore(resourceType sources.ResourceType, fieldPath string, source sources.ID) float64 {
	// Find the authority configuration for this field
	auth := merger.authorities.Find(resourceType, fieldPath)
	if auth == nil || auth.Source != source {
		// No authority match for this source, return 0
		return 0.0
	}

	// Normalize priority to 0-1.0 scale
	// Known priority range: 70-110 (from authority.go defaults)
	// Using wider range for safety: 0-150
	minPriority := 0.0
	maxPriority := 150.0
	priority := float64(auth.Priority)

	if priority <= minPriority {
		return 0.0
	}
	if priority >= maxPriority {
		return 1.0
	}

	// Linear interpolation
	return (priority - minPriority) / (maxPriority - minPriority)
}

// calculateConfidence returns confidence level for a data value.
// Returns 1.0 for non-empty values (we trust the data we have).
// Future enhancement: could factor in data quality indicators, source reliability, etc.
func (merger *merger) calculateConfidence(value any) float64 {
	// Simple implementation: if we have a value, we're confident in it
	if value != nil && value != "" {
		return 1.0
	}
	return 0.0
}

// Models merges models from multiple sources.
func (merger *merger) Models(srcs map[sources.ID][]*catalogs.Model) ([]*catalogs.Model, provenance.Map, error) {
	return merger.ModelsForProvider("", srcs)
}

// ModelsForProvider merges models from multiple sources for one provider.
func (merger *merger) ModelsForProvider(providerID catalogs.ProviderID, srcs map[sources.ID][]*catalogs.Model) ([]*catalogs.Model, provenance.Map, error) {
	// Create a map of models by ID across all sources
	modelsByID := make(map[string]map[sources.ID]*catalogs.Model)

	// Collect all models
	for sourceType, models := range srcs {
		for _, model := range models {
			if modelsByID[model.ID] == nil {
				modelsByID[model.ID] = make(map[sources.ID]*catalogs.Model)
			}
			modelsByID[model.ID][sourceType] = model
		}
	}

	mergedModels := make([]*catalogs.Model, 0, len(modelsByID))
	allProvenance := make(provenance.Map)

	// Merge each model
	for modelID, sourceModels := range modelsByID {
		merged, history := merger.model(providerID, modelID, sourceModels)
		mergedModels = append(mergedModels, merged)

		// Add provenance with model prefix
		if merger.tracker != nil {
			for field, fieldProv := range history {
				key := fmt.Sprintf("models.%s.%s", modelID, field)
				// Convert FieldProvenance to []ProvenanceInfo
				provInfos := []provenance.Provenance{fieldProv.Current}
				provInfos = append(provInfos, fieldProv.History...)
				allProvenance[key] = provInfos
				merger.tracker.Track(sources.ResourceTypeModel, modelID, field, fieldProv.Current)
			}
		}
	}

	return mergedModels, allProvenance, nil
}

// Providers merges providers from multiple sources.
func (merger *merger) Providers(srcs map[sources.ID][]*catalogs.Provider) ([]*catalogs.Provider, error) {
	// Create a map of providers by ID across all sources
	providersByID := make(map[catalogs.ProviderID]map[sources.ID]*catalogs.Provider)

	// Collect all providers
	for sourceType, providers := range srcs {
		for _, provider := range providers {
			if providersByID[provider.ID] == nil {
				providersByID[provider.ID] = make(map[sources.ID]*catalogs.Provider)
			}
			providersByID[provider.ID][sourceType] = provider
		}
	}

	var mergedProviders []*catalogs.Provider

	// Merge each provider
	for providerID, sourceProviders := range providersByID {
		// Convert to pointer map for compatibility
		sourcePtrs := make(map[sources.ID]*catalogs.Provider)
		for source, provider := range sourceProviders {
			p := provider // Create a copy
			sourcePtrs[source] = p
		}

		merged, _ := merger.provider(providerID, sourcePtrs)
		if merged != nil {
			mergedProviders = append(mergedProviders, merged)
		}
	}

	return mergedProviders, nil
}

// model merges a single model from multiple sources.
func (merger *merger) model(providerID catalogs.ProviderID, modelID string, sourceModels map[sources.ID]*catalogs.Model) (*catalogs.Model, map[string]provenance.Field) {
	// Start with existing model from baseline if available to preserve timestamps
	var merged *catalogs.Model
	baselineModel := merger.baselineModel(providerID, modelID)
	if baselineModel != nil {
		merged = baselineModel
	}
	var baselineModelSnapshot *catalogs.Model
	if baselineModel != nil {
		snapshot := catalogs.DeepCopyModel(*baselineModel)
		baselineModelSnapshot = &snapshot
	}
	// Ensure ID is set even if not found in baseline
	if merged == nil || merged.ID == "" {
		merged = &catalogs.Model{
			ID: modelID,
		}
	}
	history := make(map[string]provenance.Field)

	// Merge each field according to authorities
	for _, rule := range fieldRulesFor(sources.ResourceTypeModel) {
		value, sourceType, reason := merger.modelField(rule, sourceModels)
		if value != nil {
			merger.setModelFieldValue(merged, rule.reflectPath, value)
			merger.recordModelHistory(&history, rule, sourceType, value, reason)
		}
	}

	// Enhanced merging for complex nested structures
	merged = merger.complexModelStructures(merged, sourceModels, &history)

	// Handle timestamps with change detection
	// Store baseline model for comparison (before it gets overwritten)
	baselineModel = baselineModelSnapshot

	// Determine if this is truly a new model (not found in baseline)
	isNewModel := baselineModel == nil

	// Check if content has actually changed by comparing with baseline
	hasContentChanged := true // Default to true if no baseline
	if baselineModel != nil {
		// Compare models excluding timestamps
		baselineCopy := *baselineModel
		mergedCopy := *merged // Create a copy, not just copy the pointer
		// Clear timestamps for comparison
		baselineCopy.CreatedAt = utc.Time{}
		baselineCopy.UpdatedAt = utc.Time{}
		mergedCopy.CreatedAt = utc.Time{}
		mergedCopy.UpdatedAt = utc.Time{}
		baselineCopy.Extensions = catalogs.NormalizeSourceExtensions(baselineCopy.Extensions)
		mergedCopy.Extensions = catalogs.NormalizeSourceExtensions(mergedCopy.Extensions)
		// Compare using reflect.DeepEqual
		hasContentChanged = !reflect.DeepEqual(baselineCopy, mergedCopy)
	}

	// Update timestamps based on model state
	if isNewModel {
		now := utc.Now()
		createdAt := sourceCreatedAt(sourceModels)
		if createdAt.IsZero() {
			createdAt = now
		}
		updatedAt := sourceUpdatedAt(sourceModels)
		if updatedAt.IsZero() {
			updatedAt = createdAt
		}
		merged.CreatedAt = createdAt
		merged.UpdatedAt = updatedAt
	} else if hasContentChanged {
		// Existing model with changes: preserve created_at, update updated_at
		merged.UpdatedAt = utc.Now()
	}
	// else: Existing model, no changes: preserve both timestamps
	// (timestamps already copied from baseline at line 178)

	return merged, history
}

func sourceCreatedAt(sourceModels map[sources.ID]*catalogs.Model) utc.Time {
	for _, sourceType := range []sources.ID{
		sources.ProvidersID,
		sources.ModelsDevHTTPID,
		sources.ModelsDevGitID,
		sources.LocalCatalogID,
	} {
		if model, ok := sourceModels[sourceType]; ok && model != nil && !model.CreatedAt.IsZero() {
			return model.CreatedAt
		}
	}
	return utc.Time{}
}

func sourceUpdatedAt(sourceModels map[sources.ID]*catalogs.Model) utc.Time {
	for _, sourceType := range []sources.ID{
		sources.ModelsDevHTTPID,
		sources.ModelsDevGitID,
		sources.ProvidersID,
		sources.LocalCatalogID,
	} {
		if model, ok := sourceModels[sourceType]; ok && model != nil && !model.UpdatedAt.IsZero() {
			return model.UpdatedAt
		}
	}
	return utc.Time{}
}

// provider merges a single provider from multiple sources.
func (merger *merger) provider(providerID catalogs.ProviderID, sourceProviders map[sources.ID]*catalogs.Provider) (*catalogs.Provider, map[string]provenance.Field) {
	if len(sourceProviders) == 0 {
		return nil, nil
	}

	// Start with a base provider
	var merged catalogs.Provider
	history := make(map[string]provenance.Field)

	// Merge each field
	for _, rule := range fieldRulesFor(sources.ResourceTypeProvider) {
		value, sourceType := merger.providerField(rule, sourceProviders)
		if value != nil {
			merger.setProviderFieldValue(&merged, rule.reflectPath, value)

			provenancePath := rule.provenance()
			history[provenancePath] = provenance.Field{
				Current: provenance.Provenance{
					Source:    sourceType,
					Field:     provenancePath,
					Value:     value,
					Timestamp: time.Now(),
				},
			}
		}
	}
	merger.mergeProviderExtensions(&merged, sourceProviders, &history)

	// Ensure ID is set
	merged.ID = providerID

	return &merged, history
}

// modelField merges a single field from multiple model sources.
func (merger *merger) modelField(rule fieldRule, sourceModels map[sources.ID]*catalogs.Model) (any, sources.ID, string) {
	// Collect all values from sources
	values := make(map[sources.ID]any)
	for source, model := range sourceModels {
		if value := merger.modelFieldValue(model, rule.reflectPath); value != nil {
			values[source] = value
		}
	}

	if len(values) > 0 {
		// Let the strategy decide - it will use authorities if it's AuthorityStrategy
		// or source priority order if it's SourceOrderStrategy
		value, source, reason := merger.resolveConflict(rule.resource, rule.authority(), values)
		return value, source, reason
	}

	return nil, "", ""
}

// providerField merges a single provider field from multiple sources.
func (merger *merger) providerField(rule fieldRule, sourceProviders map[sources.ID]*catalogs.Provider) (any, sources.ID) {
	// Collect all values from sources
	values := make(map[sources.ID]any)
	for source, provider := range sourceProviders {
		if provider != nil {
			if value := merger.providerFieldValue(*provider, rule.reflectPath); value != nil {
				values[source] = value
			}
		}
	}

	if len(values) > 0 {
		// Let the strategy decide - it will use authorities if it's AuthorityStrategy
		// or source priority order if it's SourceOrderStrategy
		value, source, _ := merger.resolveConflict(rule.resource, rule.authority(), values)
		return value, source
	}

	return nil, ""
}

func (merger *merger) resolveConflict(resourceType sources.ResourceType, fieldPath string, values map[sources.ID]any) (any, sources.ID, string) {
	if resolver, ok := merger.strategy.(resourceConflictResolver); ok {
		return resolver.ResolveResourceConflict(resourceType, fieldPath, values)
	}
	return merger.strategy.ResolveConflict(fieldPath, values)
}

func (merger *merger) recordModelHistory(history *map[string]provenance.Field, rule fieldRule, source sources.ID, value any, reason string) {
	if history == nil {
		return
	}

	provenancePath := rule.provenance()
	current := provenance.Provenance{
		Source:     source,
		Field:      provenancePath,
		Value:      value,
		Timestamp:  time.Now(),
		Authority:  merger.calculateAuthorityScore(rule.resource, rule.authority(), source),
		Confidence: merger.calculateConfidence(value),
		Reason:     reason,
	}
	if evidence, exists := merger.observations[source]; exists {
		current.ObservationID = evidence.id
		current.ObservedAt = evidence.observedAt
		current.Revision = evidence.revision
		current.EvidenceChecksum = evidence.evidenceChecksum
	}
	(*history)[provenancePath] = provenance.Field{
		Current: current,
	}
}

// getModelFieldValue extracts a field value from a model using reflection.
func (merger *merger) modelFieldValue(model *catalogs.Model, fieldPath string) any {
	return merger.fieldValue(reflect.ValueOf(model), fieldPath)
}

// providerFieldValue extracts a field value from a provider using reflection.
func (merger *merger) providerFieldValue(provider catalogs.Provider, fieldPath string) any {
	return merger.fieldValue(reflect.ValueOf(provider), fieldPath)
}

// fieldValue extracts a field value using reflection and dot notation.
func (merger *merger) fieldValue(v reflect.Value, fieldPath string) any {
	parts := strings.Split(fieldPath, ".")

	current := v
	for _, part := range parts {
		if current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return nil
			}
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return nil
		}

		// Use the field name directly (already properly capitalized)
		field := current.FieldByName(part)
		if !field.IsValid() {
			return nil
		}

		current = field
	}

	if !current.IsValid() || current.IsZero() {
		return nil
	}

	return current.Interface()
}

// setModelFieldValue sets a field value on a model using reflection.
func (merger *merger) setModelFieldValue(model *catalogs.Model, fieldPath string, value any) {
	if fieldPath == "Features" {
		if features, ok := value.(*catalogs.ModelFeatures); ok {
			copied := *features
			copied.Modalities.Input = append([]catalogs.ModelModality(nil), features.Modalities.Input...)
			copied.Modalities.Output = append([]catalogs.ModelModality(nil), features.Modalities.Output...)
			model.Features = &copied
			return
		}
	}
	if fieldPath == "Limits" {
		if limits, ok := value.(*catalogs.ModelLimits); ok {
			model.Limits = mergeModelLimitsOverlay(model.Limits, limits)
			return
		}
	}
	merger.setFieldValue(reflect.ValueOf(model).Elem(), fieldPath, value)
}

// setProviderFieldValue sets a field value on a provider using reflection.
func (merger *merger) setProviderFieldValue(provider *catalogs.Provider, fieldPath string, value any) {
	merger.setFieldValue(reflect.ValueOf(provider).Elem(), fieldPath, value)
}

// setFieldValue sets a field value using reflection and dot notation.
func (merger *merger) setFieldValue(v reflect.Value, fieldPath string, value any) {
	parts := strings.Split(fieldPath, ".")

	current := v
	for i, part := range parts {
		if current.Kind() == reflect.Pointer {
			if current.IsNil() {
				// Create new struct for pointer field
				newStruct := reflect.New(current.Type().Elem())
				current.Set(newStruct)
			}
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			logging.Warn().
				Str("field_path", fieldPath).
				Str("part", part).
				Msg("Cannot set field - not a struct")
			return
		}

		// Use the field name directly (already properly capitalized)
		field := current.FieldByName(part)
		if !field.IsValid() {
			logging.Warn().
				Str("field_name", part).
				Msg("Field not found in struct")
			return
		}

		// If this is the last part, set the value
		if i == len(parts)-1 {
			if field.CanSet() {
				valueReflect := reflect.ValueOf(value)
				if valueReflect.Type().ConvertibleTo(field.Type()) {
					field.Set(valueReflect.Convert(field.Type()))
				} else {
					logging.Warn().
						Interface("value", value).
						Str("target_type", field.Type().String()).
						Str("field_path", fieldPath).
						Msg("Cannot convert value to target type")
				}
			} else {
				logging.Warn().
					Str("field_path", fieldPath).
					Msg("Field is not settable")
			}
			return
		}

		current = field
	}
}

// complexModelStructures handles merging of complex nested structures.
//
//nolint:gocyclo // Complex field-by-field merge logic
func (merger *merger) complexModelStructures(merged *catalogs.Model, sourceModels map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) *catalogs.Model {
	// Define priority order for complex structure merging
	priorities := []sources.ID{
		sources.LocalCatalogID,
		sources.ModelsDevHTTPID,
		sources.ModelsDevGitID,
		sources.ProvidersID,
	}
	providerLineagePriorities := []sources.ID{
		sources.LocalCatalogID,
		sources.ProvidersID,
		sources.ModelsDevHTTPID,
		sources.ModelsDevGitID,
	}

	// Merge Limits structure. models.dev is authoritative for subfields it
	// reports; provider/local data fills gaps for sparse models.dev entries.
	claimedLimitFields := &modelLimitFieldSet{}
	for _, sourceType := range []sources.ID{sources.ModelsDevHTTPID, sources.ModelsDevGitID} {
		if model, exists := sourceModels[sourceType]; exists && model.Limits != nil {
			if merged.Limits == nil {
				merged.Limits = &catalogs.ModelLimits{}
			}

			merger.applyModelLimits(merged, model.Limits, claimedLimitFields, sourceType, history)
			break
		}
	}
	for _, sourceType := range []sources.ID{sources.ProvidersID, sources.LocalCatalogID} {
		if model, exists := sourceModels[sourceType]; exists && model.Limits != nil {
			if merged.Limits == nil {
				merged.Limits = &catalogs.ModelLimits{}
			}
			merger.applyModelLimits(merged, model.Limits, claimedLimitFields, sourceType, history)
		}
	}

	// Merge Lineage structure. models.dev is authoritative for family, while
	// provider APIs are authoritative for root/parent when present.
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && model.Lineage != nil && model.Lineage.Family != "" {
			if merged.Lineage == nil {
				merged.Lineage = &catalogs.ModelLineage{}
			}
			merged.Lineage.Family = model.Lineage.Family
			rule := modelProvenanceRule(modelProvenanceLineageFamily)
			merger.recordModelHistory(history, rule, sourceType, model.Lineage.Family, fmt.Sprintf("selected from %s (complex structure merge)", sourceType))
			break
		}
	}
	for _, sourceType := range providerLineagePriorities {
		if model, exists := sourceModels[sourceType]; exists && model.Lineage != nil && model.Lineage.Root != nil && *model.Lineage.Root != "" {
			if merged.Lineage == nil {
				merged.Lineage = &catalogs.ModelLineage{}
			}
			root := *model.Lineage.Root
			merged.Lineage.Root = &root
			rule := modelProvenanceRule(modelProvenanceLineageRoot)
			merger.recordModelHistory(history, rule, sourceType, root, fmt.Sprintf("selected from %s (complex structure merge)", sourceType))
			break
		}
	}
	for _, sourceType := range providerLineagePriorities {
		if model, exists := sourceModels[sourceType]; exists && model.Lineage != nil && model.Lineage.Parent != nil && *model.Lineage.Parent != "" {
			if merged.Lineage == nil {
				merged.Lineage = &catalogs.ModelLineage{}
			}
			parent := *model.Lineage.Parent
			merged.Lineage.Parent = &parent
			rule := modelProvenanceRule(modelProvenanceLineageParent)
			merger.recordModelHistory(history, rule, sourceType, parent, fmt.Sprintf("selected from %s (complex structure merge)", sourceType))
			break
		}
	}

	merger.applyCanonicalPricing(merged, sourceModels, history)

	// Merge Metadata structure (models.dev is authoritative)
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && model.Metadata != nil {
			switch sourceType {
			case sources.ModelsDevHTTPID, sources.ModelsDevGitID:
				merged.Metadata = mergeModelsDevMetadata(merged.Metadata, model.Metadata)

				if history != nil {
					rule := modelProvenanceRule(modelProvenanceMetadata)
					merger.recordModelHistory(history, rule, sourceType, model.Metadata, fmt.Sprintf("selected from %s (complex structure merge)", sourceType))
				}
			}
		}
	}
	for _, sourceType := range []sources.ID{sources.ProvidersID, sources.LocalCatalogID} {
		if model, exists := sourceModels[sourceType]; exists && model.Metadata != nil {
			merged.Metadata = mergeSupplementalMetadata(merged.Metadata, model.Metadata)
		}
	}

	// Boolean feature values come from the winning whole Features observation.
	// A non-nil provider Features record makes false explicit; only modalities
	// are set-valued and may accumulate documented lower-authority values.
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && model.Features != nil {
			if merged.Features == nil {
				merged.Features = &catalogs.ModelFeatures{}
			}

			mergeModelFeatureCapabilities(merged.Features, model.Features)
		}
	}
	protectedExtensionFields := make(sourceExtensionFieldSet)
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && len(model.Extensions) > 0 {
			merged.Extensions = mergeSourceExtensions(merged.Extensions, model.Extensions, protectedExtensionFields)
			if history != nil {
				rule := modelProvenanceRule("extensions")
				merger.recordModelHistory(history, rule, sourceType, model.Extensions, fmt.Sprintf("merged from %s (source extension merge)", sourceType))
			}
		}
	}

	return merged
}

func (merger *merger) applyCanonicalPricing(merged *catalogs.Model, sourceModels map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	policy, found := authority.FindCanonicalPolicy(sources.ResourceTypeProviderOffering, "Pricing")
	if !found {
		return
	}

	rejected := make([]provenance.Rejection, 0, len(policy.AuthorityOrder))
	for _, sourceType := range policy.AuthorityOrder {
		model, exists := sourceModels[sourceType]
		if !exists || model == nil || model.Pricing == nil {
			continue
		}
		if err := model.Pricing.Validate(); err != nil {
			rejected = append(rejected, provenance.Rejection{Source: sourceType, Reason: err.Error()})
			continue
		}
		if !model.Pricing.IsEffectiveAt(merger.pricingAt) {
			rejected = append(rejected, provenance.Rejection{Source: sourceType, Reason: fmt.Sprintf("pricing is not effective at %s", merger.pricingAt.Format(time.RFC3339))})
			continue
		}

		merged.Pricing = copyModelPricing(model.Pricing)
		reason := fmt.Sprintf("selected complete provider-offering pricing from %s", sourceType)
		if len(rejected) > 0 {
			reasons := make([]string, 0, len(rejected))
			for _, rejection := range rejected {
				reasons = append(reasons, fmt.Sprintf("%s: %s", rejection.Source, rejection.Reason))
			}
			reason += fmt.Sprintf(" after rejecting %s", strings.Join(reasons, "; "))
		}
		rule := modelProvenanceRule(modelProvenancePricing)
		merger.recordModelHistory(history, rule, sourceType, model.Pricing, reason)
		field := (*history)[rule.provenance()]
		field.Current.Rejections = append([]provenance.Rejection(nil), rejected...)
		(*history)[rule.provenance()] = field
		return
	}
}

type modelLimitFieldSet struct {
	contextWindow bool
	inputTokens   bool
	outputTokens  bool
}

func (merger *merger) applyModelLimits(
	target *catalogs.Model,
	limits *catalogs.ModelLimits,
	claimed *modelLimitFieldSet,
	sourceType sources.ID,
	history *map[string]provenance.Field,
) {
	if target == nil || limits == nil || claimed == nil {
		return
	}
	if target.Limits == nil {
		target.Limits = &catalogs.ModelLimits{}
	}
	reason := fmt.Sprintf("selected from %s (complex structure merge)", sourceType)
	if limits.ContextWindow > 0 && !claimed.contextWindow {
		target.Limits.ContextWindow = limits.ContextWindow
		claimed.contextWindow = true
		rule := modelProvenanceRule(modelProvenanceLimitsContextWindow)
		merger.recordModelHistory(history, rule, sourceType, limits.ContextWindow, reason)
	}
	if limits.InputTokens > 0 && !claimed.inputTokens {
		target.Limits.InputTokens = limits.InputTokens
		claimed.inputTokens = true
		rule := modelProvenanceRule(modelProvenanceLimitsInputTokens)
		merger.recordModelHistory(history, rule, sourceType, limits.InputTokens, reason)
	}
	if limits.OutputTokens > 0 && !claimed.outputTokens {
		target.Limits.OutputTokens = limits.OutputTokens
		claimed.outputTokens = true
		rule := modelProvenanceRule(modelProvenanceLimitsOutputTokens)
		merger.recordModelHistory(history, rule, sourceType, limits.OutputTokens, reason)
	}
}

func mergeModelLimitsOverlay(target, source *catalogs.ModelLimits) *catalogs.ModelLimits {
	if source == nil {
		return target
	}
	if target == nil {
		return copyModelLimits(source)
	}
	if source.ContextWindow > 0 {
		target.ContextWindow = source.ContextWindow
	}
	if source.InputTokens > 0 {
		target.InputTokens = source.InputTokens
	}
	if source.OutputTokens > 0 {
		target.OutputTokens = source.OutputTokens
	}
	return target
}

func copyModelLimits(source *catalogs.ModelLimits) *catalogs.ModelLimits {
	if source == nil {
		return nil
	}
	copied := *source
	return &copied
}

func mergeModelsDevMetadata(target, source *catalogs.ModelMetadata) *catalogs.ModelMetadata {
	if source == nil {
		return target
	}
	if target == nil {
		target = &catalogs.ModelMetadata{}
	}
	if !source.ReleaseDate.IsZero() {
		target.ReleaseDate = source.ReleaseDate
	}
	if source.KnowledgeCutoff != nil && !source.KnowledgeCutoff.IsZero() {
		knowledgeCutoff := *source.KnowledgeCutoff
		target.KnowledgeCutoff = &knowledgeCutoff
	}
	if source.OpenWeights {
		target.OpenWeights = true
	}
	target.Tags = mergeModelTags(target.Tags, source.Tags)
	target.Architecture = mergeModelArchitecture(target.Architecture, source.Architecture)
	return target
}

func mergeSupplementalMetadata(target, source *catalogs.ModelMetadata) *catalogs.ModelMetadata {
	if source == nil {
		return target
	}
	if target == nil {
		return copyModelMetadata(source)
	}
	if target.ReleaseDate.IsZero() && !source.ReleaseDate.IsZero() {
		target.ReleaseDate = source.ReleaseDate
	}
	if target.KnowledgeCutoff == nil && source.KnowledgeCutoff != nil && !source.KnowledgeCutoff.IsZero() {
		knowledgeCutoff := *source.KnowledgeCutoff
		target.KnowledgeCutoff = &knowledgeCutoff
	}
	if source.OpenWeights {
		target.OpenWeights = true
	}
	target.Tags = mergeModelTags(target.Tags, source.Tags)
	target.Architecture = mergeModelArchitecture(target.Architecture, source.Architecture)
	return target
}

func copyModelPricing(source *catalogs.ModelPricing) *catalogs.ModelPricing {
	if source == nil {
		return nil
	}
	copied := *source
	copied.EffectiveFrom = copyValuePtr(source.EffectiveFrom)
	copied.EffectiveUntil = copyValuePtr(source.EffectiveUntil)
	copied.Tokens = copyModelTokenPricing(source.Tokens)
	copied.Operations = copyModelOperationPricing(source.Operations)
	copied.Tiers = copyModelPricingTiers(source.Tiers)
	return &copied
}

func copyModelTokenPricing(source *catalogs.ModelTokenPricing) *catalogs.ModelTokenPricing {
	if source == nil {
		return nil
	}
	copied := *source
	copied.Input = copyModelTokenCost(source.Input)
	copied.Output = copyModelTokenCost(source.Output)
	copied.Reasoning = copyModelTokenCost(source.Reasoning)
	copied.Cache = copyModelTokenCachePricing(source.Cache)
	copied.CacheRead = copyModelTokenCost(source.CacheRead)
	copied.CacheWrite = copyModelTokenCost(source.CacheWrite)
	return &copied
}

func copyModelTokenCost(source *catalogs.ModelTokenCost) *catalogs.ModelTokenCost {
	return copyValuePtr(source)
}

func copyModelTokenCachePricing(source *catalogs.ModelTokenCachePricing) *catalogs.ModelTokenCachePricing {
	if source == nil {
		return nil
	}
	copied := *source
	copied.Read = copyModelTokenCost(source.Read)
	copied.Write = copyModelTokenCost(source.Write)
	return &copied
}

func copyModelOperationPricing(source *catalogs.ModelOperationPricing) *catalogs.ModelOperationPricing {
	if source == nil {
		return nil
	}
	copied := *source
	copied.Request = copyValuePtr(source.Request)
	copied.ImageInput = copyValuePtr(source.ImageInput)
	copied.AudioInput = copyValuePtr(source.AudioInput)
	copied.VideoInput = copyValuePtr(source.VideoInput)
	copied.ImageGen = copyValuePtr(source.ImageGen)
	copied.AudioGen = copyValuePtr(source.AudioGen)
	copied.VideoGen = copyValuePtr(source.VideoGen)
	copied.WebSearch = copyValuePtr(source.WebSearch)
	copied.FunctionCall = copyValuePtr(source.FunctionCall)
	copied.ToolUse = copyValuePtr(source.ToolUse)
	return &copied
}

func copyModelPricingTiers(source []catalogs.ModelPricingTier) []catalogs.ModelPricingTier {
	if source == nil {
		return nil
	}
	copied := make([]catalogs.ModelPricingTier, len(source))
	for i := range source {
		copied[i] = source[i]
		copied[i].Tokens = copyModelTokenPricing(source[i].Tokens)
		copied[i].Operations = copyModelOperationPricing(source[i].Operations)
	}
	return copied
}

func copyModelMetadata(source *catalogs.ModelMetadata) *catalogs.ModelMetadata {
	if source == nil {
		return nil
	}
	copied := *source
	copied.KnowledgeCutoff = copyValuePtr(source.KnowledgeCutoff)
	copied.Tags = append([]catalogs.ModelTag(nil), source.Tags...)
	copied.Architecture = copyModelArchitecture(source.Architecture)
	return &copied
}

func copyModelArchitecture(source *catalogs.ModelArchitecture) *catalogs.ModelArchitecture {
	if source == nil {
		return nil
	}
	copied := *source
	copied.Precision = copyValuePtr(source.Precision)
	copied.BaseModel = copyValuePtr(source.BaseModel)
	return &copied
}

func mergeModelArchitecture(target, source *catalogs.ModelArchitecture) *catalogs.ModelArchitecture {
	if source == nil {
		return target
	}
	if target == nil {
		return copyModelArchitecture(source)
	}
	if target.ParameterCount == "" {
		target.ParameterCount = source.ParameterCount
	}
	if target.Type == "" {
		target.Type = source.Type
	}
	if target.Tokenizer == "" {
		target.Tokenizer = source.Tokenizer
	}
	if target.Precision == nil {
		target.Precision = copyValuePtr(source.Precision)
	}
	if target.Quantization == "" {
		target.Quantization = source.Quantization
	}
	if source.Quantized {
		target.Quantized = true
	}
	if source.FineTuned {
		target.FineTuned = true
	}
	if target.BaseModel == nil {
		target.BaseModel = copyValuePtr(source.BaseModel)
	}
	return target
}

func mergeModelTags(target, source []catalogs.ModelTag) []catalogs.ModelTag {
	if len(source) == 0 {
		return target
	}
	seen := make(map[catalogs.ModelTag]struct{}, len(target)+len(source))
	merged := make([]catalogs.ModelTag, 0, len(target)+len(source))
	for _, tag := range target {
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		merged = append(merged, tag)
	}
	for _, tag := range source {
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		merged = append(merged, tag)
	}
	return merged
}

func copyValuePtr[T any](source *T) *T {
	if source == nil {
		return nil
	}
	copied := *source
	return &copied
}

func (merger *merger) mergeProviderExtensions(merged *catalogs.Provider, sourceProviders map[sources.ID]*catalogs.Provider, history *map[string]provenance.Field) {
	priorities := []sources.ID{
		sources.LocalCatalogID,
		sources.ModelsDevHTTPID,
		sources.ModelsDevGitID,
		sources.ProvidersID,
	}
	protectedExtensionFields := make(sourceExtensionFieldSet)
	for _, sourceType := range priorities {
		provider, exists := sourceProviders[sourceType]
		if !exists || provider == nil || len(provider.Extensions) == 0 {
			continue
		}
		merged.Extensions = mergeSourceExtensions(merged.Extensions, provider.Extensions, protectedExtensionFields)
		if history != nil {
			(*history)["extensions"] = provenance.Field{
				Current: provenance.Provenance{
					Source:     sourceType,
					Field:      "extensions",
					Value:      provider.Extensions,
					Timestamp:  time.Now(),
					Confidence: merger.calculateConfidence(provider.Extensions),
					Reason:     fmt.Sprintf("merged from %s (source extension merge)", sourceType),
				},
			}
		}
	}
}

type sourceExtensionFieldSet map[string]map[string]struct{}

func (set sourceExtensionFieldSet) has(sourceName, field string) bool {
	fields, ok := set[sourceName]
	if !ok {
		return false
	}
	_, ok = fields[field]
	return ok
}

func (set sourceExtensionFieldSet) add(sourceName, field string) {
	fields, ok := set[sourceName]
	if !ok {
		fields = make(map[string]struct{})
		set[sourceName] = fields
	}
	fields[field] = struct{}{}
}

func mergeSourceExtensions(target, source catalogs.SourceExtensions, protected sourceExtensionFieldSet) catalogs.SourceExtensions {
	if len(source) == 0 {
		return target
	}
	if target == nil {
		target = make(catalogs.SourceExtensions, len(source))
	}
	for sourceName, extension := range source {
		existing := target[sourceName]
		if existing.Fields == nil {
			existing.Fields = make(map[string]any)
		}
		fields := catalogs.NormalizeExtensionFields(extension.Copy().Fields)
		for key, value := range fields {
			if protected != nil && protected.has(sourceName, key) {
				continue
			}
			existing.Fields[key] = value
			if protected != nil {
				protected.add(sourceName, key)
			}
		}
		target[sourceName] = existing
	}
	return target
}

func mergeModelFeatureCapabilities(target, source *catalogs.ModelFeatures) {
	if target == nil || source == nil {
		return
	}

	target.Modalities.Input = mergeModelModalities(target.Modalities.Input, source.Modalities.Input)
	target.Modalities.Output = mergeModelModalities(target.Modalities.Output, source.Modalities.Output)
}

func mergeModelModalities(target, source []catalogs.ModelModality) []catalogs.ModelModality {
	if len(source) == 0 {
		return target
	}
	seen := make(map[catalogs.ModelModality]struct{}, len(target)+len(source))
	merged := make([]catalogs.ModelModality, 0, len(target)+len(source))
	for _, modality := range target {
		if _, ok := seen[modality]; ok {
			continue
		}
		seen[modality] = struct{}{}
		merged = append(merged, modality)
	}
	for _, modality := range source {
		if _, ok := seen[modality]; ok {
			continue
		}
		seen[modality] = struct{}{}
		merged = append(merged, modality)
	}
	return merged
}
