package reconciler

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

const modelProvenancePricing = "pricing"

func (merger *merger) applyModelPolicy(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	switch policy.Path {
	case "Lineage":
		// Leaf policies select lineage facts before record presence.
	case "Description":
		merger.mergeModelDescription(identity, target, policy, models, history)
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
	for _, limit := range catalogs.PublishedModelLimits() {
		for _, presence := range []catalogs.ValuePresence{catalogs.ValueKnown, catalogs.ValueUnknown} {
			if merger.selectModelLimit(identity, target, policy, models, history, limit, presence) {
				break
			}
		}
	}
}

func (merger *merger) selectModelLimit(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
	limit catalogs.ModelLimit,
	presence catalogs.ValuePresence,
) bool {
	fieldPolicy := policy
	fieldPolicy.EvidencePath = "limits." + string(limit)
	fieldSources := merger.modelSourcesForValue(
		identity.providerID,
		identity.modelID,
		fieldPolicy,
		models,
		func(model *catalogs.Model) any {
			if model == nil {
				return nil
			}
			value, state := model.Limits.Value(limit)
			if state != presence {
				return nil
			}
			if state == catalogs.ValueUnknown {
				// A typed nil preserves an explicit unknown claim during projection.
				var unknown *int64
				return unknown
			}
			return value
		},
	)
	for _, source := range policy.SourceOrder {
		model := fieldSources[source]
		if model == nil {
			continue
		}
		value, state := model.Limits.Value(limit)
		if state != presence {
			continue
		}
		if target.Limits == nil {
			target.Limits = &catalogs.ModelLimits{}
		}
		var evidenceValue any
		if state == catalogs.ValueKnown {
			target.Limits.Set(limit, value)
			evidenceValue = value
		} else {
			target.Limits.SetUnknown(limit)
		}
		merger.recordModelHistory(
			identity, history, fieldPolicy, source, evidenceValue,
			fmt.Sprintf("selected from %s by limits presence and authority order", source),
		)
		return true
	}
	return false
}

func (merger *merger) mergeModelMetadata(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model == nil || model.Metadata == nil {
			continue
		}
		merger.mergeMetadataContributions(identity, target, policy, models, history)
		if target.Metadata == nil {
			merger.clearCompositeEvidence(identity, history, policy.Evidence())
			return
		}
		merger.recordCompositeSummary(history, policy, target.Metadata)
		return
	}
}

func (merger *merger) mergeModelFeatures(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	originalModels := merger.suppressProjectedFeatureDefaults(identity, models)
	merger.selectModelContribution(identity, policy, originalModels, history, ".present", func(model *catalogs.Model) (any, bool) {
		return true, model.Features != nil
	}, func(any) {})
	models = originalModels

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
		if local := originalModels[sources.LocalCatalogID]; local != nil && local.Features != nil {
			target.Features = nil
			merger.clearCompositeEvidence(identity, history, policy.Evidence())
		}
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
	merger.mergeFeatureEvidence(identity, target.Features, policy, originalModels, history)
	completeCompositePresence(history, policy, policy.Evidence())
	if history != nil && (*history)[policy.Evidence()+".present"].Current.Source == "" {
		target.Features = nil
		merger.clearCompositeEvidence(identity, history, policy.Evidence())
		return
	}
	merger.recordCompositeSummary(history, policy, target.Features)
}

// suppressProjectedFeatureDefaults prevents the human YAML capability checklist
// from becoming synthetic local evidence. An untouched, all-false projection
// only formats a baseline that had no feature record. It does not assert an
// operator choice, so a later source claim must replace it.
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
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model == nil || len(model.Modes) == 0 {
			continue
		}
		merger.mergeModeContributions(identity, target, policy, models, history)
		if len(target.Modes) == 0 {
			merger.clearCompositeEvidence(identity, history, policy.Evidence())
			return
		}
		merger.recordCompositeSummary(history, policy, target.Modes)
		return
	}
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
				Reason: pricingExclusionReason(model.Pricing, merger.pricingAt),
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
			Timestamp:  merger.changeTime(),
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
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model == nil || len(model.Extensions) == 0 {
			continue
		}
		merger.mergeExtensionContributions(identity, target, policy, models, history)
		if len(target.Extensions) == 0 {
			merger.clearCompositeEvidence(identity, history, policy.Evidence())
			return
		}
		merger.recordCompositeSummary(history, policy, target.Extensions)
		return
	}
}

func (merger *merger) mergeModelAuthors(
	identity modelIdentity,
	target *catalogs.Model,
	policy authority.Policy,
	models map[sources.ID]*catalogs.Model,
	history *map[string]provenance.Field,
) {
	merger.mergeAuthorshipContributions(identity, target, policy, models, history)
}

// pricingExclusionReason names the immutable interval boundary that caused refusal.
// Rechecking the same expired evidence does not change its materialized receipt.
func pricingExclusionReason(pricing *catalogs.ModelPricing, at time.Time) string {
	if pricing.EffectiveFrom != nil && at.Before(pricing.EffectiveFrom.Time()) {
		return fmt.Sprintf("pricing is not effective before %s", pricing.EffectiveFrom.Time().UTC().Format(time.RFC3339Nano))
	}
	if pricing.EffectiveUntil != nil {
		return fmt.Sprintf("pricing is not effective at or after %s", pricing.EffectiveUntil.Time().UTC().Format(time.RFC3339Nano))
	}
	return "pricing is outside its effective interval"
}
