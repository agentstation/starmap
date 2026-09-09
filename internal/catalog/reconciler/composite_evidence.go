package reconciler

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// selectModelContribution selects one independently merged field and its receipt.
// A present nil is an explicit value, distinct from an omitted field.
func (merger *merger) selectModelContribution(identity modelIdentity, policy authority.Policy, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field, path string, get func(*catalogs.Model) (any, bool), set func(any)) sources.ID {
	field := policy
	field.EvidencePath = policy.Evidence() + path
	selected := merger.modelSourcesForValue(identity.providerID, identity.modelID, field, models, func(model *catalogs.Model) any {
		if model == nil {
			return nil
		}
		value, present := get(model)
		if !present {
			return nil
		}
		if value == nil {
			var explicitNull *string
			return explicitNull
		}
		return value
	})
	for _, source := range policy.SourceOrder {
		model := selected[source]
		if model == nil {
			continue
		}
		value, present := get(model)
		if !present {
			continue
		}
		set(value)
		merger.recordModelHistory(identity, history, field, source, value, "selected composite field by authority with its original receipt")
		return source
	}
	if local := models[sources.LocalCatalogID]; local != nil {
		if _, present := get(local); present {
			merger.clearModelContributionEvidence(history, field.Evidence(), nil)
		}
	}
	return ""
}

// clearModelContributionEvidence replaces a refused current claim.
func (merger *merger) clearModelContributionEvidence(history *map[string]provenance.Field, field string, value any) {
	if history == nil {
		return
	}
	(*history)[field] = provenance.Field{Current: provenance.Entry{
		Field: field, Value: value, Timestamp: merger.changeTime(),
		Reason: "cleared by composite authority policy",
	}}
}

func compositeEvidenceField(prefix, field string) bool {
	return field == prefix || strings.HasPrefix(field, prefix+".") || strings.HasPrefix(field, prefix+"[")
}

// clearCompositeEvidence clears all current claims for a refused record.
func (merger *merger) clearCompositeEvidence(identity modelIdentity, history *map[string]provenance.Field, prefix string) {
	if merger.baseline != nil {
		for field := range merger.baseline.Provenance().FindModel(identity.providerID, identity.modelID) {
			if compositeEvidenceField(prefix, field) {
				merger.clearModelContributionEvidence(history, field, nil)
			}
		}
	}
	if history != nil {
		for field := range *history {
			if compositeEvidenceField(prefix, field) {
				merger.clearModelContributionEvidence(history, field, nil)
			}
		}
	}
	merger.clearModelContributionEvidence(history, prefix, nil)
}

func compositeMapKey(value string) string { return "[" + strconv.Quote(value) + "]" }

func (merger *merger) mergeModeContributions(identity modelIdentity, target *catalogs.Model, policy authority.Policy, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	names := make(map[string]struct{})
	for _, model := range models {
		if model == nil {
			continue
		}
		for name := range model.Modes {
			names[name] = struct{}{}
		}
	}
	result := make(map[string]catalogs.ModelMode, len(names))
	if models[sources.LocalCatalogID] == nil {
		if baseline := merger.baselineModel(identity.providerID, identity.modelID); baseline != nil {
			for name, mode := range catalogs.DeepCopyModel(catalogs.Model{Modes: baseline.Modes}).Modes {
				result[name] = mode
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		mode, baselinePresent := result[name]
		path := compositeMapKey(name)
		selected := merger.selectModelContribution(identity, policy, models, history, path+".present", func(model *catalogs.Model) (any, bool) {
			_, present := model.Modes[name]
			return true, present
		}, func(any) {})
		present := selected != "" || baselinePresent
		merger.selectModelContribution(identity, policy, models, history, path+".provider.present", func(model *catalogs.Model) (any, bool) {
			return true, model.Modes[name].Provider != nil
		}, func(any) {
			present = true
			if mode.Provider == nil {
				mode.Provider = &catalogs.ModelProviderMode{}
			}
		})
		merger.selectModelContribution(identity, policy, models, history, path+".pricing", func(model *catalogs.Model) (any, bool) {
			price := model.Modes[name].Pricing
			return price, price != nil
		}, func(value any) { present = true; mode.Pricing = copyModelPricing(value.(*catalogs.ModelPricing)) })
		for _, body := range []bool{false, true} {
			keys := make(map[string]struct{})
			for _, model := range models {
				if model == nil || model.Modes[name].Provider == nil {
					continue
				}
				provider := model.Modes[name].Provider
				if body {
					for key := range provider.Body {
						keys[key] = struct{}{}
					}
				} else {
					for key := range provider.Headers {
						keys[key] = struct{}{}
					}
				}
			}
			part := ".provider.headers"
			if body {
				part = ".provider.body"
			}
			for _, key := range slices.Sorted(maps.Keys(keys)) {
				merger.selectModelContribution(identity, policy, models, history, path+part+compositeMapKey(key), func(model *catalogs.Model) (any, bool) {
					provider := model.Modes[name].Provider
					if provider == nil {
						return nil, false
					}
					if body {
						value, present := provider.Body[key]
						return value, present
					}
					value, present := provider.Headers[key]
					return value, present
				}, func(value any) {
					present = true
					if mode.Provider == nil {
						mode.Provider = &catalogs.ModelProviderMode{}
					}
					if body {
						if mode.Provider.Body == nil {
							mode.Provider.Body = make(map[string]any)
						}
						mode.Provider.Body[key] = catalogs.SourceExtension{Fields: map[string]any{key: value}}.Copy().Fields[key]
					} else {
						if mode.Provider.Headers == nil {
							mode.Provider.Headers = make(map[string]string)
						}
						mode.Provider.Headers[key] = value.(string)
					}
				})
			}
		}
		if present {
			if mode.Provider != nil {
				completeCompositePresence(history, policy, policy.Evidence()+path+".provider")
			}
			completeCompositePresence(history, policy, policy.Evidence()+path)
			result[name] = mode
		}
	}
	target.Modes = result
}

func (merger *merger) mergeExtensionContributions(identity modelIdentity, target *catalogs.Model, policy authority.Policy, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	keys := make(map[string]map[string]struct{})
	for _, model := range models {
		if model == nil {
			continue
		}
		for namespace, extension := range model.Extensions {
			if keys[namespace] == nil {
				keys[namespace] = make(map[string]struct{})
			}
			for key := range extension.Fields {
				keys[namespace][key] = struct{}{}
			}
		}
	}
	if len(keys) == 0 {
		return
	}
	result := make(catalogs.SourceExtensions, len(keys))
	if models[sources.LocalCatalogID] == nil {
		if baseline := merger.baselineModel(identity.providerID, identity.modelID); baseline != nil {
			for namespace, extension := range baseline.Extensions.Copy() {
				result[namespace] = extension
			}
		}
	}
	for _, namespace := range slices.Sorted(maps.Keys(keys)) {
		selected := merger.selectModelContribution(identity, policy, models, history, compositeMapKey(namespace)+".present", func(model *catalogs.Model) (any, bool) {
			_, present := model.Extensions[namespace]
			return true, present
		}, func(any) {})
		extension, baselinePresent := result[namespace]
		present := selected != "" || baselinePresent
		if extension.Fields == nil {
			extension.Fields = make(map[string]any)
		}
		for _, key := range slices.Sorted(maps.Keys(keys[namespace])) {
			merger.selectModelContribution(identity, policy, models, history, compositeMapKey(namespace)+".fields"+compositeMapKey(key), func(model *catalogs.Model) (any, bool) {
				value, present := model.Extensions[namespace].Fields[key]
				return value, present
			}, func(value any) {
				present = true
				extension.Fields[key] = catalogs.SourceExtension{Fields: map[string]any{key: value}}.Copy().Fields[key]
			})
		}
		if present {
			completeCompositePresence(history, policy, policy.Evidence()+compositeMapKey(namespace))
			result[namespace] = extension
		} else {
			delete(result, namespace)
		}
	}
	target.Extensions = catalogs.NormalizeSourceExtensions(result)
}

// recordCompositeSummary identifies a leading accepted contribution.
// Individual entries retain the evidence for each independently selected field.
func (merger *merger) recordCompositeSummary(history *map[string]provenance.Field, policy authority.Policy, value any) {
	if history == nil {
		return
	}
	keys := slices.Sorted(maps.Keys(*history))
	prefix := policy.Evidence()
	if selected, found := leadingCompositeEvidence(*history, prefix, policy.SourceOrder); found {
		selected.Field, selected.Value = prefix, value
		selected.Confidence = merger.calculateConfidence(value)
		selected.Reason = "merged fields with separate original evidence; leading accepted contribution"
		(*history)[prefix] = provenance.Field{Current: selected}
		return
	}
	for _, field := range keys {
		if compositeEvidenceField(prefix, field) && (*history)[field].Current.Source == "" {
			merger.clearModelContributionEvidence(history, prefix, value)
			return
		}
	}
}

// completeCompositePresence derives record presence from an accepted child.
func completeCompositePresence(history *map[string]provenance.Field, policy authority.Policy, prefix string) {
	if history == nil {
		return
	}
	field := prefix + ".present"
	if (*history)[field].Current.Source != "" {
		return
	}
	if entry, found := leadingCompositeEvidence(*history, prefix, policy.SourceOrder); found {
		entry.Field, entry.Value = field, true
		entry.Reason = "record presence follows an accepted child contribution"
		(*history)[field] = provenance.Field{Current: entry}
	}
}

func leadingCompositeEvidence(history map[string]provenance.Field, prefix string, order []sources.ID) (provenance.Entry, bool) {
	keys := slices.Sorted(maps.Keys(history))
	for _, source := range order {
		var selected provenance.Entry
		found := false
		for _, key := range keys {
			if key == prefix || !compositeEvidenceField(prefix, key) {
				continue
			}
			entry := history[key].Current
			if entry.Source != source {
				continue
			}
			if !found || entry.ObservedAt.After(selected.ObservedAt) {
				selected, found = entry, true
			}
		}
		if found {
			return selected, true
		}
	}
	return provenance.Entry{}, false
}
