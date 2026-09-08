package reconciler

import (
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// mergeMetadataContributions selects independent metadata facts before publication.
func (merger *merger) mergeMetadataContributions(identity modelIdentity, target *catalogs.Model, policy authority.Policy, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	result := catalogs.Model{Metadata: &catalogs.ModelMetadata{}}
	if models[sources.LocalCatalogID] == nil {
		if baseline := merger.baselineModel(identity.providerID, identity.modelID); baseline != nil && baseline.Metadata != nil {
			result.Metadata = copyModelMetadata(baseline.Metadata)
		}
	}
	selected := merger.selectModelContribution(identity, policy, models, history, ".present", func(model *catalogs.Model) (any, bool) {
		return true, model.Metadata != nil
	}, func(any) {})
	present := selected != ""
	merger.selectModelContribution(identity, policy, models, history, ".architecture.present", func(model *catalogs.Model) (any, bool) {
		return true, model.Metadata != nil && model.Metadata.Architecture != nil
	}, func(any) {
		present = true
		if result.Metadata.Architecture == nil {
			result.Metadata.Architecture = &catalogs.ModelArchitecture{}
		}
	})
	for _, field := range []struct{ path, evidence string }{
		{"Metadata.ReleaseDate", ".release_date"},
		{"Metadata.KnowledgeCutoff", ".knowledge_cutoff"},
		{"Metadata.Architecture.ParameterCount", ".architecture.parameter_count"},
		{"Metadata.Architecture.Type", ".architecture.type"},
		{"Metadata.Architecture.Tokenizer", ".architecture.tokenizer"},
		{"Metadata.Architecture.Quantization", ".architecture.quantization"},
		{"Metadata.Architecture.BaseModel", ".architecture.base_model"},
	} {
		merger.selectModelContribution(identity, policy, models, history, field.evidence, func(model *catalogs.Model) (any, bool) {
			value := merger.modelFieldValue(model, field.path)
			return value, value != nil
		}, func(value any) { present = true; merger.setModelFieldValue(&result, field.path, value) })
	}
	for _, flag := range []struct {
		path    string
		value   func(*catalogs.ModelArchitecture) (bool, catalogs.ValuePresence)
		set     func(*catalogs.ModelArchitecture, bool)
		unknown func(*catalogs.ModelArchitecture)
	}{
		{".architecture.quantized", (*catalogs.ModelArchitecture).QuantizedValue, (*catalogs.ModelArchitecture).SetQuantized, (*catalogs.ModelArchitecture).SetQuantizedUnknown},
		{".architecture.fine_tuned", (*catalogs.ModelArchitecture).FineTunedValue, (*catalogs.ModelArchitecture).SetFineTuned, (*catalogs.ModelArchitecture).SetFineTunedUnknown},
	} {
		for _, presence := range []catalogs.ValuePresence{catalogs.ValueKnown, catalogs.ValueUnknown} {
			selected := merger.selectModelContribution(identity, policy, models, history, flag.path, func(model *catalogs.Model) (any, bool) {
				if model.Metadata == nil {
					return nil, false
				}
				value, state := flag.value(model.Metadata.Architecture)
				if state != presence {
					return nil, false
				}
				if state == catalogs.ValueUnknown {
					return nil, true
				}
				return value, true
			}, func(value any) {
				present = true
				if result.Metadata.Architecture == nil {
					result.Metadata.Architecture = &catalogs.ModelArchitecture{}
				}
				if value == nil {
					flag.unknown(result.Metadata.Architecture)
				} else {
					flag.set(result.Metadata.Architecture, value.(bool))
				}
			})
			if selected != "" {
				break
			}
		}
	}
	for _, presence := range []catalogs.ValuePresence{catalogs.ValueKnown, catalogs.ValueUnknown} {
		selected := merger.selectModelContribution(identity, policy, models, history, ".open_weights", func(model *catalogs.Model) (any, bool) {
			value, state := model.Metadata.OpenWeightsValue()
			if state != presence {
				return nil, false
			}
			if state == catalogs.ValueUnknown {
				return nil, true
			}
			return value, true
		}, func(value any) {
			present = true
			if value == nil {
				result.Metadata.SetOpenWeightsUnknown()
			} else {
				result.Metadata.SetOpenWeights(value.(bool))
			}
		})
		if selected != "" {
			break
		}
	}
	tags := make([]catalogs.ModelTag, 0)
	for _, source := range policy.SourceOrder {
		model := models[source]
		if model != nil && model.Metadata != nil {
			tags = mergeModelTags(tags, model.Metadata.Tags)
		}
	}
	for _, tag := range tags {
		merger.selectModelContribution(identity, policy, models, history, ".tags"+compositeMapKey(string(tag)), func(model *catalogs.Model) (any, bool) {
			return true, model.Metadata != nil && slices.Contains(model.Metadata.Tags, tag)
		}, func(any) {
			present = true
			result.Metadata.Tags = mergeModelTags(result.Metadata.Tags, []catalogs.ModelTag{tag})
		})
	}
	if present {
		if result.Metadata.Architecture != nil {
			completeCompositePresence(history, policy, policy.Evidence()+".architecture")
		}
		completeCompositePresence(history, policy, policy.Evidence())
		target.Metadata = copyModelMetadata(result.Metadata)
	} else if models[sources.LocalCatalogID] != nil {
		target.Metadata = nil
	}
}
