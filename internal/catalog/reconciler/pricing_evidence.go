package reconciler

import (
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// retainsBaselinePricingDiagnostic prevents a synthetic replay from reporting a new rejection.
// A real pricing observation must still pass validation and record its outcome.
func (merger *merger) retainsBaselinePricingDiagnostic(identity modelIdentity, models map[sources.ID]*catalogs.Model) bool {
	if !merger.sparseLocalModelDelta(identity.providerID, identity.modelID) {
		return false
	}
	baseline := merger.baselineModel(identity.providerID, identity.modelID)
	if baseline == nil || baseline.Pricing == nil {
		return false
	}
	if baseline.Pricing.Validate() == nil && baseline.Pricing.IsEffectiveAt(merger.pricingAt) {
		return false
	}
	entries := merger.baseline.Provenance().FindModelField(identity.providerID, identity.modelID, modelProvenancePricing)
	if len(entries) == 0 {
		return false
	}
	entry := entries[0]
	for _, candidate := range entries[1:] {
		if candidate.Timestamp.After(entry.Timestamp) {
			entry = candidate
		}
	}
	if entry.Source != "" || !semanticValueEqual(modelProvenancePricing, entry.Value, baseline.Pricing) {
		return false
	}
	found := false
	for source, model := range models {
		if model == nil || model.Pricing == nil {
			continue
		}
		if !merger.acceptedBaselineCarrier(source) || !semanticValueEqual(modelProvenancePricing, model.Pricing, baseline.Pricing) {
			return false
		}
		found = true
	}
	return found
}

func mergePricingRejections(retained, current []provenance.Rejection) []provenance.Rejection {
	merged := slices.Clone(retained)
	for _, rejection := range current {
		if !slices.Contains(merged, rejection) {
			merged = append(merged, rejection)
		}
	}
	return merged
}
