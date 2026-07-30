package reconciler

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/internal/catalog/authority"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

type provenanceTracker interface {
	Track(catalogmeta.ResourceType, string, string, provenance.Entry)
}

// merger implements strategic three-way merge.
type merger struct {
	authorities     authority.Reader
	strategy        *AuthorityStrategy
	tracker         provenanceTracker
	baseline        *catalogs.Catalog // Baseline catalog for timestamp preservation
	baselineModels  map[catalogs.ProviderID]map[string]*catalogs.Model
	pricingAt       time.Time
	observations    map[sources.ID]sourceObservationEvidence
	sourceCatalogs  map[sources.ID]*catalogs.Catalog
	carriedEvidence map[evidenceLocator]provenance.Entry
}

type sourceObservationEvidence struct {
	id               string
	observedAt       time.Time
	revision         sources.Revision
	evidenceChecksum string
	completeness     sources.ObservationCompleteness
	status           sources.ObservationStatus
	records          sources.ObservationRecordCounts
	issues           []sources.ObservationIssue
}

func (e sourceObservationEvidence) healthReason() string {
	if e.status == sources.ObservationStatusSucceeded &&
		e.completeness == sources.ObservationCompletenessComplete &&
		e.records.Rejected == 0 &&
		len(e.issues) == 0 {
		return ""
	}
	issueCodes := make([]string, 0, len(e.issues))
	for _, issue := range e.issues {
		issueCodes = append(issueCodes, string(issue.Code))
	}
	return fmt.Sprintf(
		"observation health status=%s completeness=%s accepted=%d rejected=%d issues=%s",
		e.status,
		e.completeness,
		e.records.Accepted,
		e.records.Rejected,
		strings.Join(issueCodes, ","),
	)
}

// newMerger creates a new strategic merger.
func newMerger(authorities authority.Reader, strategy *AuthorityStrategy, baseline *catalogs.Catalog) *merger {
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
	merger.sourceCatalogs = make(map[sources.ID]*catalogs.Catalog, len(observations))
	for _, observation := range observations {
		merger.observations[observation.SourceID] = sourceObservationEvidence{
			id:               observation.ID,
			observedAt:       observation.ObservedAt,
			revision:         observation.Revision,
			evidenceChecksum: observation.EvidenceChecksum,
			completeness:     observation.Completeness,
			status:           observation.Status,
			records:          observation.Records,
			issues:           append([]sources.ObservationIssue(nil), observation.Issues...),
		}
		merger.sourceCatalogs[observation.SourceID] = observation.Catalog
	}
}

// newMergerWithProvenance creates a new strategic merger with provenance tracking.
func newMergerWithProvenance(authorities authority.Reader, strategy *AuthorityStrategy, tracker provenanceTracker, baseline *catalogs.Catalog) *merger {
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

func (merger *merger) calculateAuthorityScore(resourceType sources.ResourceType, fieldPath string, source sources.ID) float64 {
	policy, found := merger.authorities.Find(resourceType, fieldPath)
	if !found {
		return 0.0
	}
	return policy.Authority(source)
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
	// Absence is not lifecycle evidence. Seed the identity set from the
	// last-known-good baseline so a complete, partial, or degraded observation
	// cannot retire a model merely by omitting it.
	for _, model := range merger.baselineModels[providerID] {
		if model == nil || model.ID == "" {
			continue
		}
		if _, exists := modelsByID[model.ID]; !exists {
			modelsByID[model.ID] = make(map[sources.ID]*catalogs.Model)
		}
	}

	mergedModels := make([]*catalogs.Model, 0, len(modelsByID))
	allProvenance := make(provenance.Map)

	// Merge each model
	for modelID, sourceModels := range modelsByID {
		merged, history := merger.model(providerID, modelID, sourceModels)
		mergedModels = append(mergedModels, merged)
		modelResourceID := provenance.ModelResourceID(string(providerID), modelID)

		// Add provenance with model prefix
		if merger.tracker != nil {
			for field, fieldProv := range history {
				key := fmt.Sprintf("models.%s.%s", modelID, field)
				// Convert FieldProvenance to []ProvenanceInfo
				provInfos := make([]provenance.Entry, 1, 1+len(fieldProv.History))
				provInfos[0] = fieldProv.Current
				provInfos = append(provInfos, fieldProv.History...)
				allProvenance[key] = provInfos
				merger.tracker.Track(sources.ResourceTypeModel, modelResourceID, field, fieldProv.Current)
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

		merged, history := merger.provider(providerID, sourcePtrs)
		if merged != nil {
			mergedProviders = append(mergedProviders, merged)
		}
		if merger.tracker != nil {
			for field, fieldProvenance := range history {
				if field == "Models" {
					continue
				}
				merger.tracker.Track(
					sources.ResourceTypeProvider,
					string(providerID),
					field,
					fieldProvenance.Current,
				)
			}
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
	identity := modelIdentity{providerID: providerID, modelID: modelID}

	for _, policy := range merger.authorities.Policies(sources.ResourceTypeModel) {
		policySources := sourceModels
		if policy.Path != "Limits" {
			policySources = merger.modelSourcesForPolicy(providerID, modelID, policy, sourceModels)
		}
		merger.applyModelPolicy(identity, merged, policy, policySources, &history)
	}

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
		hasContentChanged = !baselineCopy.Equal(mergedCopy)
	}

	// Update timestamps based on model state
	if isNewModel {
		now := utc.Now()
		createdAt := merger.sourceTime(identity, "CreatedAt", sourceModels)
		if createdAt.IsZero() {
			createdAt = now
		}
		updatedAt := merger.sourceTime(identity, "UpdatedAt", sourceModels)
		if updatedAt.IsZero() {
			updatedAt = createdAt
		}
		if createdAt.After(updatedAt) {
			createdAt, updatedAt = updatedAt, createdAt
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

func (merger *merger) sourceTime(identity modelIdentity, path string, sourceModels map[sources.ID]*catalogs.Model) utc.Time {
	policy, found := merger.authorities.Find(sources.ResourceTypeModel, path)
	if !found {
		return utc.Time{}
	}
	sourceModels = merger.modelSourcesForPolicy(identity.providerID, identity.modelID, policy, sourceModels)
	for _, source := range policy.SourceOrder {
		model := sourceModels[source]
		if model == nil {
			continue
		}
		value, ok := merger.modelFieldValue(model, path).(utc.Time)
		if ok && !value.IsZero() {
			return value
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
	if merger.baseline != nil {
		if baseline, err := merger.baseline.Provider(providerID); err == nil {
			merged = catalogs.DeepCopyProvider(baseline)
		}
	}
	history := make(map[string]provenance.Field)

	// Merge each field
	for _, policy := range merger.authorities.Policies(sources.ResourceTypeProvider) {
		policySources := merger.providerSourcesForPolicy(providerID, policy, sourceProviders)
		merger.applyProviderPolicy(providerID, &merged, policy, policySources, &history)
	}

	// Ensure ID is set
	merged.ID = providerID

	return &merged, history
}

// modelField merges a single field from multiple model sources.
func (merger *merger) modelField(policy authority.Policy, sourceModels map[sources.ID]*catalogs.Model) (any, sources.ID, string) {
	// Collect all values from sources
	values := make(map[sources.ID]any)
	for source, model := range sourceModels {
		if value := merger.modelFieldValue(model, policy.Path); value != nil {
			values[source] = value
		}
	}

	if len(values) > 0 {
		value, source, reason := merger.resolveConflict(policy.Resource, policy.Path, values)
		return value, source, reason
	}

	return nil, "", ""
}

// providerField merges a single provider field from multiple sources.
func (merger *merger) providerField(policy authority.Policy, sourceProviders map[sources.ID]*catalogs.Provider) (any, sources.ID) {
	// Collect all values from sources
	values := make(map[sources.ID]any)
	for source, provider := range sourceProviders {
		if provider != nil {
			if value := merger.providerFieldValue(*provider, policy.Path); value != nil {
				values[source] = value
			}
		}
	}

	if len(values) > 0 {
		value, source, _ := merger.resolveConflict(policy.Resource, policy.Path, values)
		return value, source
	}

	return nil, ""
}

func (merger *merger) resolveConflict(resourceType sources.ResourceType, fieldPath string, values map[sources.ID]any) (any, sources.ID, string) {
	return merger.strategy.ResolveResourceConflict(resourceType, fieldPath, values)
}

func (merger *merger) recordModelHistory(
	identity modelIdentity,
	history *map[string]provenance.Field,
	policy authority.Policy,
	source sources.ID,
	value any,
	reason string,
) {
	if history == nil {
		return
	}

	provenancePath := policy.Evidence()
	if carried, ok := merger.carried(
		sources.ResourceTypeModel,
		identity.resourceID(),
		provenancePath,
		source,
	); ok {
		carried.Field = provenancePath
		carried.Value = value
		(*history)[provenancePath] = provenance.Field{Current: carried}
		return
	}
	current := provenance.Entry{
		Source:     source,
		Field:      provenancePath,
		Value:      value,
		Timestamp:  time.Now(),
		Authority:  merger.calculateAuthorityScore(policy.Resource, policy.Path, source),
		Confidence: merger.calculateConfidence(value),
		Reason:     reason,
	}
	if evidence, exists := merger.observations[source]; exists {
		current.ObservationID = evidence.id
		current.ObservedAt = evidence.observedAt
		current.Revision = evidence.revision
		current.EvidenceChecksum = evidence.evidenceChecksum
		if health := evidence.healthReason(); health != "" {
			current.Reason += "; " + health
		}
	}
	(*history)[provenancePath] = provenance.Field{
		Current: current,
	}
}

// getModelFieldValue extracts a field value from a model using reflection.
func (merger *merger) modelFieldValue(model *catalogs.Model, fieldPath string) any {
	if model == nil {
		return nil
	}
	if fieldPath == "Description" {
		value, state := model.DescriptionValue()
		if state == catalogs.ValueKnown {
			return value
		}
		return nil
	}
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
	if fieldPath == "Description" {
		if description, ok := value.(string); ok {
			model.SetDescription(description)
			return
		}
	}
	if fieldPath == "Features" {
		if features, ok := value.(*catalogs.ModelFeatures); ok {
			copied := *features
			copied.Modalities.Input = append([]catalogs.ModelModality(nil), features.Modalities.Input...)
			copied.Modalities.Output = append([]catalogs.ModelModality(nil), features.Modalities.Output...)
			model.Features = &copied
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
	openWeights, sourcePresence := source.OpenWeightsValue()
	_, targetPresence := target.OpenWeightsValue()
	switch {
	case sourcePresence == catalogs.ValueKnown && targetPresence != catalogs.ValueKnown:
		target.SetOpenWeights(openWeights)
	case sourcePresence == catalogs.ValueUnknown && targetPresence == catalogs.ValueMissing:
		target.SetOpenWeightsUnknown()
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
	copied.CacheRead = copyModelTokenCost(source.CacheRead)
	copied.CacheWrite = copyModelTokenCost(source.CacheWrite)
	return &copied
}

func copyModelTokenCost(source *catalogs.ModelTokenCost) *catalogs.ModelTokenCost {
	return copyValuePtr(source)
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

func (merger *merger) mergeProviderExtensions(
	providerID catalogs.ProviderID,
	merged *catalogs.Provider,
	policy authority.Policy,
	sourceProviders map[sources.ID]*catalogs.Provider,
	history *map[string]provenance.Field,
) {
	protectedExtensionFields := make(sourceExtensionFieldSet)
	var winner sources.ID
	for _, sourceType := range policy.SourceOrder {
		provider, exists := sourceProviders[sourceType]
		if !exists || provider == nil || len(provider.Extensions) == 0 {
			continue
		}
		if winner == "" {
			winner = sourceType
		}
		merged.Extensions = mergeSourceExtensions(merged.Extensions, provider.Extensions, protectedExtensionFields)
	}
	if history != nil && winner != "" {
		merger.recordProviderHistory(
			providerID,
			history,
			policy,
			winner,
			merged.Extensions,
			"merged namespaced extension fields by authority order",
		)
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
