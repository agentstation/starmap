package reconciler

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/catalog/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	modelProvenanceLimitsContextWindow = "limits.context_window"
	modelProvenanceLimitsInputTokens   = "limits.input_tokens"
	modelProvenanceLimitsOutputTokens  = "limits.output_tokens"
	modelProvenancePricing             = "pricing"
)

func (merger *merger) applyModelPolicy(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	switch policy.Path {
	case "Limits":
		merger.mergeModelLimits(identity, target, policy, models, history)
	case "Metadata":
		merger.mergeModelMetadata(identity, target, policy, models, history)
	case "Features":
		merger.mergeModelFeatures(identity, target, policy, models, history)
	case "Modes":
		merger.mergeModelModes(identity, target, policy, models, history)
	case "Pricing":
		merger.mergeModelPricing(identity, target, policy, models, history)
	case "Extensions":
		merger.mergeModelExtensions(identity, target, policy, models, history)
	case "Authors":
		merger.mergeModelAuthors(identity, target, policy, models, history)
	case "CreatedAt", "UpdatedAt":
		// Timestamp publication is change-aware and executes after facts merge.
	default:
		value, source, reason := merger.modelField(policy, models)
		if value == nil {
			return
		}
		merger.setModelFieldValue(target, policy.Path, value)
		merger.recordModelHistory(identity, history, policy, source, value, reason)
	}
}

func (merger *merger) mergeModelLimits(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	fields := []struct {
		evidence string
		limit    catalogs.ModelLimit
	}{
		{
			evidence: modelProvenanceLimitsContextWindow,
			limit:    catalogs.ModelLimitContextWindow,
		},
		{
			evidence: modelProvenanceLimitsInputTokens,
			limit:    catalogs.ModelLimitInputTokens,
		},
		{
			evidence: modelProvenanceLimitsOutputTokens,
			limit:    catalogs.ModelLimitOutputTokens,
		},
	}
	for _, field := range fields {
		fieldPolicy := policy
		fieldPolicy.EvidencePath = field.evidence
		fieldSources := merger.modelSourcesForValue(
			identity.providerID,
			identity.modelID,
			fieldPolicy,
			models,
			func(model *catalogs.Model) any {
				if model == nil || model.Limits == nil {
					return nil
				}
				value, state := model.Limits.Value(field.limit)
				if state != catalogs.ValueKnown {
					return nil
				}
				return value
			},
		)
		for _, source := range policy.SourceOrder {
			model := fieldSources[source]
			if model == nil || model.Limits == nil {
				continue
			}
			value, state := model.Limits.Value(field.limit)
			if state != catalogs.ValueKnown {
				continue
			}
			if target.Limits == nil {
				target.Limits = &catalogs.ModelLimits{}
			}
			target.Limits.Set(field.limit, value)
			merger.recordModelHistory(
				identity,
				history,
				fieldPolicy,
				source,
				value,
				fmt.Sprintf("selected from %s by limits authority order", source),
			)
			break
		}
	}
}

func (merger *merger) mergeModelMetadata(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	var (
		merged *catalogs.ModelMetadata
		winner sources.ID
	)
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model == nil || model.Metadata == nil {
			continue
		}
		if winner == "" {
			winner = source
		}
		merged = mergeSupplementalMetadata(merged, model.Metadata)
	}
	merged = mergeSupplementalMetadata(merged, target.Metadata)
	if merged == nil {
		return
	}
	target.Metadata = merged
	if winner != "" {
		merger.recordModelHistory(
			identity,
			history,
			policy,
			winner,
			merged,
			fmt.Sprintf("merged by %s policy with lower-authority gap filling", policy.Path),
		)
	}
}

func (merger *merger) mergeModelFeatures(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	models = merger.modelSourcesForValue(
		identity.providerID,
		identity.modelID,
		policy,
		models,
		func(model *catalogs.Model) any {
			if model == nil {
				return nil
			}
			return model.Features
		},
	)
	models = merger.suppressProjectedFeatureDefaults(identity, models)

	var (
		winner     sources.ID
		modalities catalogs.ModelModalities
	)
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model == nil || model.Features == nil {
			continue
		}
		if winner == "" {
			winner = source
		}
		modalities.Input = mergeModelModalities(modalities.Input, model.Features.Modalities.Input)
		modalities.Output = mergeModelModalities(modalities.Output, model.Features.Modalities.Output)
	}
	if winner == "" {
		return
	}

	// Merge from lowest to highest authority. MergeModels understands compact
	// feature presence, so a higher source replaces explicit true/false claims
	// while a missing claim permits a lower source to fill the gap.
	merged := catalogs.Model{}
	for index := len(policy.SourceOrder) - 1; index >= 0; index-- {
		model := models[policy.SourceOrder[index]]
		if model == nil || model.Features == nil {
			continue
		}
		merged = catalogs.MergeModels(merged, catalogs.Model{
			Features: copyModelFeatures(model.Features),
		})
	}
	merged.Features.Modalities = modalities
	target.Features = merged.Features
	merger.recordModelHistory(
		identity,
		history,
		policy,
		winner,
		merged.Features,
		fmt.Sprintf("merged capabilities by field presence with %s authority; accumulated documented modalities", winner),
	)
}

// suppressProjectedFeatureDefaults prevents the complete human YAML
// capability checklist from becoming synthetic local evidence. When the
// committed baseline had no feature record, an untouched all-false projection
// is formatting—not an operator assertion—and a later real source claim must
// be able to replace it.
func (merger *merger) suppressProjectedFeatureDefaults(
	identity modelIdentity,
	models map[sources.ID]*catalogs.Model,
) map[sources.ID]*catalogs.Model {
	local := models[sources.LocalCatalogID]
	if local == nil || !onlyConservativeFeatureDefaults(local.Features) {
		return models
	}
	baseline := merger.baselineModel(identity.providerID, identity.modelID)
	if baseline == nil || baseline.Features != nil {
		return models
	}
	resolved := cloneModelSources(models)
	delete(resolved, sources.LocalCatalogID)
	return resolved
}

func onlyConservativeFeatureDefaults(features *catalogs.ModelFeatures) bool {
	if features == nil ||
		len(features.Modalities.Input) > 0 ||
		len(features.Modalities.Output) > 0 {
		return false
	}
	value := reflect.ValueOf(*features)
	for index := 0; index < value.NumField(); index++ {
		if value.Field(index).Kind() == reflect.Bool && value.Field(index).Bool() {
			return false
		}
	}
	return true
}

func copyModelFeatures(features *catalogs.ModelFeatures) *catalogs.ModelFeatures {
	if features == nil {
		return nil
	}
	copied := catalogs.DeepCopyModel(catalogs.Model{Features: features})
	return copied.Features
}

func (merger *merger) mergeModelModes(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	var winner sources.ID
	merged := make(map[string]catalogs.ModelMode)
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model == nil || len(model.Modes) == 0 {
			continue
		}
		if winner == "" {
			winner = source
		}
		sourceCopy := catalogs.DeepCopyModel(catalogs.Model{Modes: model.Modes}).Modes
		for name, mode := range sourceCopy {
			existing, exists := merged[name]
			if !exists {
				merged[name] = mode
				continue
			}
			merged[name] = fillModelMode(existing, mode)
		}
	}
	if len(merged) == 0 {
		return
	}
	target.Modes = merged
	merger.recordModelHistory(
		identity,
		history,
		policy,
		winner,
		merged,
		fmt.Sprintf("merged named modes by %s policy", policy.Path),
	)
}

func fillModelMode(target, fallback catalogs.ModelMode) catalogs.ModelMode {
	if target.Pricing == nil {
		target.Pricing = fallback.Pricing
	}
	if target.Provider == nil {
		target.Provider = fallback.Provider
		return target
	}
	if fallback.Provider == nil {
		return target
	}
	if target.Provider.Headers == nil {
		target.Provider.Headers = make(map[string]string)
	}
	for key, value := range fallback.Provider.Headers {
		if _, exists := target.Provider.Headers[key]; !exists {
			target.Provider.Headers[key] = value
		}
	}
	if target.Provider.Body == nil {
		target.Provider.Body = make(map[string]any)
	}
	for key, value := range fallback.Provider.Body {
		if _, exists := target.Provider.Body[key]; !exists {
			target.Provider.Body[key] = value
		}
	}
	return target
}

func (merger *merger) mergeModelPricing(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	rejected := make([]provenance.Rejection, 0, len(policy.SourceOrder))
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model == nil || model.Pricing == nil {
			continue
		}
		if err := model.Pricing.Validate(); err != nil {
			rejected = append(rejected, provenance.Rejection{Source: source, Reason: err.Error()})
			continue
		}
		if !model.Pricing.IsEffectiveAt(merger.pricingAt) {
			rejected = append(rejected, provenance.Rejection{
				Source: source,
				Reason: fmt.Sprintf("pricing is not effective at %s", merger.pricingAt.Format(time.RFC3339)),
			})
			continue
		}

		target.Pricing = copyModelPricing(model.Pricing)
		reason := fmt.Sprintf("selected complete provider-offering pricing from %s", source)
		if len(rejected) > 0 {
			reasons := make([]string, 0, len(rejected))
			for _, rejection := range rejected {
				reasons = append(reasons, fmt.Sprintf("%s: %s", rejection.Source, rejection.Reason))
			}
			reason += fmt.Sprintf(" after rejecting %s", strings.Join(reasons, "; "))
		}
		merger.recordModelHistory(identity, history, policy, source, model.Pricing, reason)
		if history != nil {
			field := (*history)[policy.Evidence()]
			field.Current.Rejections = append([]provenance.Rejection(nil), rejected...)
			(*history)[policy.Evidence()] = field
		}
		return
	}
	if len(rejected) == 0 || history == nil {
		return
	}
	reason := "no valid current pricing candidate"
	var (
		retained   any
		confidence float64
	)
	if target.Pricing != nil {
		reason += "; retained prior pricing"
		retained = copyModelPricing(target.Pricing)
		confidence = merger.calculateConfidence(target.Pricing)
	}
	(*history)[policy.Evidence()] = provenance.Field{
		Current: provenance.Entry{
			Field:      policy.Evidence(),
			Value:      retained,
			Timestamp:  time.Now(),
			Rejections: append([]provenance.Rejection(nil), rejected...),
			Confidence: confidence,
			Reason:     reason,
		},
	}
}

func (merger *merger) mergeModelExtensions(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	protected := make(sourceExtensionFieldSet)
	var winner sources.ID
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model == nil || len(model.Extensions) == 0 {
			continue
		}
		if winner == "" {
			winner = source
		}
		target.Extensions = mergeSourceExtensions(target.Extensions, model.Extensions, protected)
	}
	if winner != "" {
		merger.recordModelHistory(
			identity,
			history,
			policy,
			winner,
			target.Extensions,
			"merged namespaced extension fields by authority order",
		)
	}
}

func (merger *merger) mergeModelAuthors(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	seen := make(map[catalogs.AuthorID]struct{})
	merged := make([]catalogs.Author, 0)
	var winner sources.ID
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model == nil || len(model.Authors) == 0 {
			continue
		}
		if winner == "" {
			winner = source
		}
		for _, author := range model.Authors {
			if _, exists := seen[author.ID]; exists {
				continue
			}
			seen[author.ID] = struct{}{}
			merged = append(merged, author)
		}
	}
	if len(merged) == 0 {
		return
	}
	target.Authors = merged
	merger.recordModelHistory(identity, history, policy, winner, merged, "merged non-duplicate authors by authority order")
}
