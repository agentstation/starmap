package reconciler

import "github.com/agentstation/starmap/pkg/sources"

type fieldRule struct {
	resource       sources.ResourceType
	reflectPath    string
	authorityPath  string
	provenancePath string
}

func newFieldRule(resource sources.ResourceType, path string) fieldRule {
	return fieldRule{
		resource:    resource,
		reflectPath: path,
	}
}

func newProvenanceFieldRule(resource sources.ResourceType, reflectPath, provenancePath string) fieldRule {
	return fieldRule{
		resource:       resource,
		reflectPath:    reflectPath,
		authorityPath:  reflectPath,
		provenancePath: provenancePath,
	}
}

func (rule fieldRule) authority() string {
	if rule.authorityPath != "" {
		return rule.authorityPath
	}
	return rule.reflectPath
}

func (rule fieldRule) provenance() string {
	if rule.provenancePath != "" {
		return rule.provenancePath
	}
	return rule.reflectPath
}

func fieldRulesFor(resource sources.ResourceType) []fieldRule {
	switch resource {
	case sources.ResourceTypeModel:
		return cloneFieldRules(modelFieldRules)
	case sources.ResourceTypeProvider:
		return cloneFieldRules(providerFieldRules)
	case sources.ResourceTypeAuthor:
		return cloneFieldRules(authorFieldRules)
	default:
		return nil
	}
}

func cloneFieldRules(rules []fieldRule) []fieldRule {
	cloned := make([]fieldRule, len(rules))
	copy(cloned, rules)
	return cloned
}

func modelProvenanceRule(provenancePath string) fieldRule {
	if rule, ok := modelProvenanceFieldRules[provenancePath]; ok {
		return rule
	}
	return fieldRule{
		resource:       sources.ResourceTypeModel,
		reflectPath:    provenancePath,
		authorityPath:  provenancePath,
		provenancePath: provenancePath,
	}
}

const (
	modelProvenanceLimitsContextWindow = "limits.context_window"
	modelProvenanceLimitsInputTokens   = "limits.input_tokens"
	modelProvenanceLimitsOutputTokens  = "limits.output_tokens"
	modelProvenanceLineageFamily       = "lineage.family"
	modelProvenanceLineageRoot         = "lineage.root"
	modelProvenanceLineageParent       = "lineage.parent"
	modelProvenancePricing             = "pricing"
	modelProvenanceMetadata            = "metadata"
	modelProvenanceModes               = "modes"
)

var modelFieldRules = []fieldRule{
	newFieldRule(sources.ResourceTypeModel, "Name"),
	newFieldRule(sources.ResourceTypeModel, "Description"),
	newFieldRule(sources.ResourceTypeModel, "Status"),
	newFieldRule(sources.ResourceTypeModel, "Authors"),
	newFieldRule(sources.ResourceTypeModel, "Lineage"),
	newFieldRule(sources.ResourceTypeModel, "Limits"),
	newFieldRule(sources.ResourceTypeModel, "Metadata"),
	newFieldRule(sources.ResourceTypeModel, "Features"),
	newFieldRule(sources.ResourceTypeModel, "Attachments"),
	newFieldRule(sources.ResourceTypeModel, "Generation"),
	newFieldRule(sources.ResourceTypeModel, "Reasoning"),
	newFieldRule(sources.ResourceTypeModel, "ReasoningTokens"),
	newFieldRule(sources.ResourceTypeModel, "Verbosity"),
	newFieldRule(sources.ResourceTypeModel, "Tools"),
	newFieldRule(sources.ResourceTypeModel, "Delivery"),
	newProvenanceFieldRule(sources.ResourceTypeModel, "Modes", modelProvenanceModes),
}

var modelProvenanceFieldRules = map[string]fieldRule{
	modelProvenanceLimitsContextWindow: newProvenanceFieldRule(sources.ResourceTypeModel, "Limits", modelProvenanceLimitsContextWindow),
	modelProvenanceLimitsInputTokens:   newProvenanceFieldRule(sources.ResourceTypeModel, "Limits", modelProvenanceLimitsInputTokens),
	modelProvenanceLimitsOutputTokens:  newProvenanceFieldRule(sources.ResourceTypeModel, "Limits", modelProvenanceLimitsOutputTokens),
	modelProvenanceLineageFamily:       newProvenanceFieldRule(sources.ResourceTypeModel, "Lineage", modelProvenanceLineageFamily),
	modelProvenanceLineageRoot:         newProvenanceFieldRule(sources.ResourceTypeModel, "Lineage", modelProvenanceLineageRoot),
	modelProvenanceLineageParent:       newProvenanceFieldRule(sources.ResourceTypeModel, "Lineage", modelProvenanceLineageParent),
	modelProvenancePricing:             newProvenanceFieldRule(sources.ResourceTypeModel, "Pricing", modelProvenancePricing),
	modelProvenanceMetadata:            newProvenanceFieldRule(sources.ResourceTypeModel, "Metadata", modelProvenanceMetadata),
}

var providerFieldRules = []fieldRule{
	newFieldRule(sources.ResourceTypeProvider, "Name"),
	newFieldRule(sources.ResourceTypeProvider, "Headquarters"),
	newFieldRule(sources.ResourceTypeProvider, "IconURL"),
	newFieldRule(sources.ResourceTypeProvider, "StatusPageURL"),
	newFieldRule(sources.ResourceTypeProvider, "Models"),
	newFieldRule(sources.ResourceTypeProvider, "Aliases"),
	newFieldRule(sources.ResourceTypeProvider, "APIKey"),
	newFieldRule(sources.ResourceTypeProvider, "EnvVars"),
	newFieldRule(sources.ResourceTypeProvider, "Catalog"),
	newFieldRule(sources.ResourceTypeProvider, "ChatCompletions"),
	newFieldRule(sources.ResourceTypeProvider, "PrivacyPolicy"),
	newFieldRule(sources.ResourceTypeProvider, "RetentionPolicy"),
	newFieldRule(sources.ResourceTypeProvider, "GovernancePolicy"),
}

var authorFieldRules = []fieldRule{
	newFieldRule(sources.ResourceTypeAuthor, "Name"),
	newFieldRule(sources.ResourceTypeAuthor, "Aliases"),
	newFieldRule(sources.ResourceTypeAuthor, "Description"),
	newFieldRule(sources.ResourceTypeAuthor, "Headquarters"),
	newFieldRule(sources.ResourceTypeAuthor, "IconURL"),
	newFieldRule(sources.ResourceTypeAuthor, "Website"),
	newFieldRule(sources.ResourceTypeAuthor, "HuggingFace"),
	newFieldRule(sources.ResourceTypeAuthor, "GitHub"),
	newFieldRule(sources.ResourceTypeAuthor, "Twitter"),
	newFieldRule(sources.ResourceTypeAuthor, "Catalog"),
	newFieldRule(sources.ResourceTypeAuthor, "Models"),
}
