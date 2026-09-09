package reconciler

import (
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// mergeFeatureEvidence selects each capability with its original receipt.
// The aggregate feature entry still identifies the leading authority.
func (merger *merger) mergeFeatureEvidence(identity modelIdentity, target *catalogs.ModelFeatures, policy authority.Policy, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	for _, feature := range catalogs.PublishedModelFeatures() {
		field := policy
		field.EvidencePath = policy.Evidence() + "." + string(feature)
		accepted := false
		for _, presence := range []catalogs.ValuePresence{catalogs.ValueKnown, catalogs.ValueUnknown} {
			value := func(model *catalogs.Model) any {
				if model == nil {
					return nil
				}
				supported, state := model.Features.Support(feature)
				if state != presence {
					return nil
				}
				if state == catalogs.ValueUnknown {
					// A typed nil preserves an explicit unknown claim during projection.
					var unknown *bool
					return unknown
				}
				return supported
			}
			selected := merger.modelSourcesForValue(identity.providerID, identity.modelID, field, models, value)
			found := false
			for _, source := range policy.SourceOrder {
				model := selected[source]
				if model == nil {
					continue
				}
				supported, state := model.Features.Support(feature)
				if state != presence {
					continue
				}
				var evidenceValue any
				if state == catalogs.ValueKnown {
					target.SetSupport(feature, supported)
					evidenceValue = supported
				} else {
					target.SetSupportUnknown(feature)
				}
				merger.recordModelHistory(identity, history, field, source, evidenceValue, "selected capability by field presence and authority")
				found = true
				break
			}
			if found {
				accepted = true
				break
			}
		}
		if local := models[sources.LocalCatalogID]; !accepted && local != nil {
			if _, state := local.Features.Support(feature); state != catalogs.ValueMissing {
				target.UnsetSupport(feature)
				merger.clearModelContributionEvidence(history, field.Evidence(), nil)
			}
		}
	}
	merger.mergeModalityEvidence(identity, target, policy, models, history)
}

func (merger *merger) mergeModalityEvidence(identity modelIdentity, target *catalogs.ModelFeatures, policy authority.Policy, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	for _, direction := range []string{"input", "output"} {
		modalities := make([]catalogs.ModelModality, 0)
		for _, source := range policy.SourceOrder {
			modalities = mergeModelModalities(modalities, modelModalities(models[source], direction))
		}
		accepted := make([]catalogs.ModelModality, 0, len(modalities))
		for _, modality := range modalities {
			field := policy
			field.EvidencePath = policy.Evidence() + ".modalities." + direction + "." + string(modality)
			value := func(model *catalogs.Model) any {
				if slices.Contains(modelModalities(model, direction), modality) {
					return true
				}
				return nil
			}
			selected := merger.modelSourcesForValue(identity.providerID, identity.modelID, field, models, value)
			found := false
			for _, source := range policy.SourceOrder {
				if value(selected[source]) == nil {
					continue
				}
				accepted = append(accepted, modality)
				merger.recordModelHistory(identity, history, field, source, true, "selected documented modality by authority")
				found = true
				break
			}
			if !found && value(models[sources.LocalCatalogID]) != nil {
				merger.clearModelContributionEvidence(history, field.Evidence(), nil)
			}
		}
		if direction == "input" {
			target.Modalities.Input = accepted
		} else {
			target.Modalities.Output = accepted
		}
	}
}

func modelModalities(model *catalogs.Model, direction string) []catalogs.ModelModality {
	if model == nil || model.Features == nil {
		return nil
	}
	if direction == "input" {
		return model.Features.Modalities.Input
	}
	return model.Features.Modalities.Output
}

// clearOrphanedReasoningEvidence removes claims for values that reasoning policy clears.
func (merger *merger) clearOrphanedReasoningEvidence(identity modelIdentity, model *catalogs.Model, history map[string]provenance.Field) {
	if model.Features != nil && model.Features.Reasoning {
		return
	}
	for _, field := range []string{"Reasoning", "ReasoningTokens", "Features.reasoning_effort", "Features.reasoning_tokens", "Features.include_reasoning"} {
		entry, exists := history[field]
		if !exists && merger.baseline != nil {
			entries := merger.baseline.Provenance().FindModelField(identity.providerID, identity.modelID, field)
			if len(entries) > 0 {
				entry.Current = entries[0]
			}
		}
		if entry.Current.Value == nil || entry.Current.Value == false {
			continue
		}
		var value any
		if field != "Reasoning" && field != "ReasoningTokens" {
			value = false
		}
		history[field] = provenance.Field{Current: provenance.Entry{
			Field: field, Value: value, Timestamp: merger.changeTime(),
			Reason: "cleared by policy because the model does not support reasoning",
		}}
	}
}
