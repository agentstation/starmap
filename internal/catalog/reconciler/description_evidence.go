package reconciler

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// mergeModelDescription preserves unknown data only when no known description wins.
// Each presence pass applies source eligibility before selecting its receipt.
func (merger *merger) mergeModelDescription(identity modelIdentity, target *catalogs.Model, policy authority.Policy, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	known := merger.modelSourcesForPolicy(identity.providerID, identity.modelID, policy, models)
	if value, source, reason := merger.modelField(policy, known); value != nil {
		merger.setModelFieldValue(target, policy.Path, value)
		merger.recordModelHistory(identity, history, policy, source, value, reason)
		return
	}
	unknown := merger.modelSourcesForValue(identity.providerID, identity.modelID, policy, models, func(model *catalogs.Model) any {
		if model != nil {
			if _, state := model.DescriptionValue(); state == catalogs.ValueUnknown {
				// A typed nil distinguishes unknown data from an omitted claim.
				var value *string
				return value
			}
		}
		return nil
	})
	for _, source := range policy.SourceOrder {
		model := unknown[source]
		if model == nil {
			continue
		}
		if _, state := model.DescriptionValue(); state != catalogs.ValueUnknown {
			continue
		}
		target.SetDescriptionUnknown()
		merger.recordModelHistory(identity, history, policy, source, nil, "selected unknown description without a known fallback")
		return
	}
	if local := models[sources.LocalCatalogID]; local != nil {
		if _, presence := local.DescriptionValue(); presence != catalogs.ValueMissing {
			target.UnsetDescription()
			merger.recordModelHistory(identity, history, policy, "", nil, "cleared description without permitted evidence")
		}
	}
}
