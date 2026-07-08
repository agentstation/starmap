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
	modelProvenanceLimitsOutputTokens  = "limits.output_tokens"
	modelProvenancePricing             = "pricing"
	modelProvenanceMetadata            = "metadata"
)

var modelFieldRules = []fieldRule{
	newFieldRule(sources.ResourceTypeModel, "Name"),
	newFieldRule(sources.ResourceTypeModel, "Description"),
	newFieldRule(sources.ResourceTypeModel, "Authors"),
	newFieldRule(sources.ResourceTypeModel, "Pricing"),
	newFieldRule(sources.ResourceTypeModel, "Limits"),
	newFieldRule(sources.ResourceTypeModel, "Metadata"),
	newFieldRule(sources.ResourceTypeModel, "Features"),
	newFieldRule(sources.ResourceTypeModel, "Generation"),
}

var modelProvenanceFieldRules = map[string]fieldRule{
	modelProvenanceLimitsContextWindow: newProvenanceFieldRule(sources.ResourceTypeModel, "Limits", modelProvenanceLimitsContextWindow),
	modelProvenanceLimitsOutputTokens:  newProvenanceFieldRule(sources.ResourceTypeModel, "Limits", modelProvenanceLimitsOutputTokens),
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
