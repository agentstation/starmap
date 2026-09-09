package reconciler

import (
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

var recordPolicyPaths = map[catalogs.ModelRecord]string{
	catalogs.ModelRecordDeprecatedAt:    "DeprecatedAt",
	catalogs.ModelRecordRetiresAt:       "RetiresAt",
	catalogs.ModelRecordMetadata:        "Metadata",
	catalogs.ModelRecordLineage:         "Lineage",
	catalogs.ModelRecordFeatures:        "Features",
	catalogs.ModelRecordAttachments:     "Attachments",
	catalogs.ModelRecordGeneration:      "Generation",
	catalogs.ModelRecordReasoning:       "Reasoning",
	catalogs.ModelRecordReasoningTokens: "ReasoningTokens",
	catalogs.ModelRecordVerbosity:       "Verbosity",
	catalogs.ModelRecordTools:           "Tools",
	catalogs.ModelRecordDelivery:        "Delivery",
	catalogs.ModelRecordPricing:         "Pricing",
	catalogs.ModelRecordLimits:          "Limits",
}

// mergeModelRecordPresence retains unknown records after known facts merge.
// Record presence cannot replace a known record or grant authority to its leaves.
func (merger *merger) mergeModelRecordPresence(identity modelIdentity, target *catalogs.Model, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	merger.clearRefusedLineageClaims(identity, target, models, history)
	for _, record := range catalogs.PublishedModelRecords() {
		policy, found := merger.authorities.Find(evidence.ResourceTypeModel, recordPolicyPaths[record])
		if !found {
			continue
		}
		recordModels := models
		if record == catalogs.ModelRecordFeatures {
			recordModels = merger.suppressProjectedFeatureDefaults(identity, models)
		}
		if empty, composite := emptyCompositeRecord(record); composite {
			if target.RecordPresence(record) == catalogs.ValueKnown && (*history)[policy.Evidence()+".present"].Current.Source != "" {
				continue
			}
			selected := merger.selectModelContribution(identity, policy, recordModels, history, ".present", func(model *catalogs.Model) (any, bool) {
				return true, model.RecordPresence(record) == catalogs.ValueKnown
			}, func(any) {
				if target.RecordPresence(record) != catalogs.ValueKnown {
					merger.setModelFieldValue(target, policy.Path, empty)
				}
			})
			if selected != "" {
				continue
			}
			if target.RecordPresence(record) == catalogs.ValueKnown {
				presence := policy
				presence.EvidencePath = policy.Evidence() + ".present"
				if recordModels[sources.LocalCatalogID].RecordPresence(record) != catalogs.ValueKnown ||
					acceptedRecordChild(*history, policy.Evidence()) ||
					!merger.modelProjectionRefused(identity, presence, true) {
					continue
				}
				target.UnsetRecord(record)
				merger.clearCompositeEvidence(identity, history, policy.Evidence())
			}
		} else if target.RecordPresence(record) == catalogs.ValueKnown {
			current := (*history)[policy.Evidence()].Current
			local := recordModels[sources.LocalCatalogID]
			if current.Source != "" || local.RecordPresence(record) != catalogs.ValueKnown ||
				!merger.modelProjectionRefused(identity, policy, merger.modelFieldValue(local, policy.Path)) {
				continue
			}
			target.UnsetRecord(record)
			merger.clearCompositeEvidence(identity, history, policy.Evidence())
		}
		unknown := merger.modelSourcesForValue(identity.providerID, identity.modelID, policy, recordModels, func(model *catalogs.Model) any {
			if model.RecordPresence(record) == catalogs.ValueUnknown {
				var value *string
				return value
			}
			return nil
		})
		selected := false
		for _, source := range policy.SourceOrder {
			if unknown[source].RecordPresence(record) != catalogs.ValueUnknown {
				continue
			}
			target.SetRecordUnknown(record)
			merger.recordModelHistory(identity, history, policy, source, nil, "selected unknown record without a known fallback")
			selected = true
			break
		}
		if !selected && recordModels[sources.LocalCatalogID].RecordPresence(record) == catalogs.ValueUnknown {
			target.UnsetRecord(record)
			merger.recordModelHistory(identity, history, policy, "", nil, "cleared record without permitted evidence")
		}
	}
}

// clearRefusedLineageClaims prevents parent presence from restoring denied leaves.
func (merger *merger) clearRefusedLineageClaims(identity modelIdentity, target *catalogs.Model, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	local := models[sources.LocalCatalogID]
	if target.Lineage == nil || local == nil || local.Lineage == nil {
		return
	}
	for _, leaf := range []struct {
		path  string
		clear func(*catalogs.ModelLineage)
	}{
		{"Lineage.Family", func(lineage *catalogs.ModelLineage) { lineage.Family = "" }},
		{"Lineage.Root", func(lineage *catalogs.ModelLineage) { lineage.Root = nil }},
		{"Lineage.Parent", func(lineage *catalogs.ModelLineage) { lineage.Parent = nil }},
	} {
		policy, found := merger.authorities.Find(evidence.ResourceTypeModel, leaf.path)
		if !found || (*history)[policy.Evidence()].Current.Source != "" {
			continue
		}
		value := merger.modelFieldValue(local, policy.Path)
		if value == nil || !merger.modelProjectionRefused(identity, policy, value) {
			continue
		}
		leaf.clear(target.Lineage)
		merger.clearModelContributionEvidence(history, policy.Evidence(), nil)
	}
}

// acceptedRecordChild keeps a container required by a permitted child claim.
func acceptedRecordChild(history map[string]provenance.Field, prefix string) bool {
	for path, field := range history {
		if strings.HasPrefix(path, prefix+".") && path != prefix+".present" && field.Current.Source != "" {
			return true
		}
	}
	return false
}

// emptyCompositeRecord preserves an observed container without restoring refused leaves.
func emptyCompositeRecord(record catalogs.ModelRecord) (any, bool) {
	switch record {
	case catalogs.ModelRecordMetadata:
		return &catalogs.ModelMetadata{}, true
	case catalogs.ModelRecordLineage:
		return &catalogs.ModelLineage{}, true
	case catalogs.ModelRecordFeatures:
		return &catalogs.ModelFeatures{}, true
	case catalogs.ModelRecordLimits:
		return &catalogs.ModelLimits{}, true
	default:
		return nil, false
	}
}
