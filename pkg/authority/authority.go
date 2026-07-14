// Package authority manages source authority for catalog data reconciliation.
package authority

import (
	"path/filepath"

	"github.com/agentstation/starmap/pkg/sources"
)

const (
	authorityPathAliases         = "Aliases"
	authorityPathAttachments     = "Attachments"
	authorityPathAuthors         = "Authors"
	authorityPathCatalog         = "Catalog"
	authorityPathDelivery        = "Delivery"
	authorityPathDescription     = "Description"
	authorityPathFeatures        = "Features"
	authorityPathGeneration      = "Generation"
	authorityPathGitHub          = "GitHub"
	authorityPathHeadquarters    = "Headquarters"
	authorityPathHuggingFace     = "HuggingFace"
	authorityPathIconURL         = "IconURL"
	authorityPathLimits          = "Limits"
	authorityPathLineage         = "Lineage"
	authorityPathMetadata        = "Metadata"
	authorityPathModes           = "Modes"
	authorityPathModels          = "Models"
	authorityPathName            = "Name"
	authorityPathPricing         = "Pricing"
	authorityPathReasoning       = "Reasoning"
	authorityPathReasoningTokens = "ReasoningTokens"
	authorityPathStatus          = "Status"
	authorityPathTools           = "Tools"
	authorityPathTwitter         = "Twitter"
	authorityPathVerbosity       = "Verbosity"
	authorityPathWebsite         = "Website"
)

// Authority determines which source is authoritative for each field.
type Authority interface {
	// Find returns the authority configuration for a specific field
	Find(resourceType sources.ResourceType, fieldPath string) *Field

	// List returns all authorities for a resource type
	ModelFields() []Field
	ProviderFields() []Field
	AuthorFields() []Field
}

// Field defines source priority for a specific field.
type Field struct {
	Path     string     `json:"path" yaml:"path"`         // e.g., "pricing.input", "metadata.knowledge_cutoff"
	Source   sources.ID `json:"source" yaml:"source"`     // Which source is authoritative
	Priority int        `json:"priority" yaml:"priority"` // Priority (higher = more authoritative)
}

// authorities provides standard field authorities.
type authorities struct {
	modelFields    []Field
	providerFields []Field
	authorFields   []Field
}

// New creates a new DefaultAuthorities with standard configurations.
func New() Authority {
	return &authorities{
		modelFields:    defaultModelAuthorities(),
		providerFields: defaultProviderAuthorities(),
		authorFields:   defaultAuthorAuthorities(),
	}
}

// Find returns the authority configuration for a specific field.
func (a *authorities) Find(resourceType sources.ResourceType, fieldPath string) *Field {
	var authorities []Field

	switch resourceType {
	case sources.ResourceTypeModel:
		authorities = a.modelFields
	case sources.ResourceTypeProvider:
		authorities = a.providerFields
	case sources.ResourceTypeAuthor:
		authorities = a.authorFields
	default:
		return nil
	}

	return findByFieldPath(authorities, fieldPath)
}

func (a *authorities) ModelFields() []Field {
	return append([]Field(nil), a.modelFields...)
}

func (a *authorities) ProviderFields() []Field {
	return append([]Field(nil), a.providerFields...)
}

func (a *authorities) AuthorFields() []Field {
	return append([]Field(nil), a.authorFields...)
}

// ByField returns the highest priority authority for a given field path.
func findByFieldPath(authorities []Field, fieldPath string) *Field {
	var bestMatch *Field
	var bestPriority int
	var bestMatchLength int

	for i, auth := range authorities {
		if MatchesPattern(fieldPath, auth.Path) {
			// Prioritize by: 1) priority, 2) pattern specificity (length), 3) order
			patternLength := len(auth.Path)
			if auth.Priority > bestPriority ||
				(auth.Priority == bestPriority && patternLength > bestMatchLength) {
				bestMatch = &authorities[i]
				bestPriority = auth.Priority
				bestMatchLength = patternLength
			}
		}
	}

	return bestMatch
}

// MatchesPattern checks if a field path matches a pattern (supports * wildcards).
func MatchesPattern(fieldPath, pattern string) bool {
	// Handle exact matches
	if fieldPath == pattern {
		return true
	}

	// Handle simple wildcard at the end
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(fieldPath) >= len(prefix) && fieldPath[:len(prefix)] == prefix
	}

	// Handle filepath.Match patterns
	matched, err := filepath.Match(pattern, fieldPath)
	if err != nil {
		return false
	}
	return matched
}

// defaultModelAuthorities returns the default field authorities for models.
func defaultModelAuthorities() []Field {
	return []Field{
		// Pricing is provider-offering data. A semantically valid provider
		// observation wins atomically; models.dev is fallback evidence.
		{Path: authorityPathPricing, Source: sources.ProvidersID, Priority: 110},
		{Path: authorityPathPricing, Source: sources.ModelsDevHTTPID, Priority: 100},
		{Path: authorityPathPricing, Source: sources.ModelsDevGitID, Priority: 90},
		{Path: authorityPathPricing, Source: sources.LocalCatalogID, Priority: 80},

		// Availability - Provider API is truth
		{Path: authorityPathFeatures, Source: sources.ProvidersID, Priority: 95},
		{Path: authorityPathFeatures, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathFeatures, Source: sources.ModelsDevGitID, Priority: 85},

		// Capability substructures - Provider API wins when it has explicit data;
		// models.dev fills gaps for providers with sparse /models responses.
		{Path: authorityPathAttachments, Source: sources.ProvidersID, Priority: 95},
		{Path: authorityPathAttachments, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathAttachments, Source: sources.ModelsDevGitID, Priority: 85},
		{Path: authorityPathReasoning, Source: sources.ProvidersID, Priority: 95},
		{Path: authorityPathReasoning, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathReasoning, Source: sources.ModelsDevGitID, Priority: 85},
		{Path: authorityPathReasoningTokens, Source: sources.ProvidersID, Priority: 95},
		{Path: authorityPathReasoningTokens, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathReasoningTokens, Source: sources.ModelsDevGitID, Priority: 85},
		{Path: authorityPathVerbosity, Source: sources.ProvidersID, Priority: 95},
		{Path: authorityPathVerbosity, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathVerbosity, Source: sources.ModelsDevGitID, Priority: 85},
		{Path: authorityPathTools, Source: sources.ProvidersID, Priority: 95},
		{Path: authorityPathTools, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathTools, Source: sources.ModelsDevGitID, Priority: 85},
		{Path: authorityPathDelivery, Source: sources.ProvidersID, Priority: 95},
		{Path: authorityPathDelivery, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathDelivery, Source: sources.ModelsDevGitID, Priority: 85},

		// Limits - models.dev has better data (HTTP preferred)
		{Path: authorityPathLimits, Source: sources.ModelsDevHTTPID, Priority: 100},
		{Path: authorityPathLimits, Source: sources.ModelsDevGitID, Priority: 90},
		{Path: authorityPathLimits, Source: sources.ProvidersID, Priority: 85},

		// Metadata - models.dev is authoritative (HTTP preferred)
		{Path: authorityPathMetadata, Source: sources.ModelsDevHTTPID, Priority: 110},
		{Path: authorityPathMetadata, Source: sources.ModelsDevGitID, Priority: 100},
		{Path: authorityPathMetadata, Source: sources.ProvidersID, Priority: 80},

		// Generation parameters - Provider API for current settings
		{Path: authorityPathGeneration, Source: sources.ProvidersID, Priority: 85},
		{Path: authorityPathGeneration, Source: sources.ModelsDevHTTPID, Priority: 80},
		{Path: authorityPathGeneration, Source: sources.ModelsDevGitID, Priority: 75},

		// Descriptions - prefer manual edits, then models.dev
		{Path: authorityPathDescription, Source: sources.LocalCatalogID, Priority: 90},
		{Path: authorityPathDescription, Source: sources.ModelsDevHTTPID, Priority: 85},
		{Path: authorityPathDescription, Source: sources.ModelsDevGitID, Priority: 80},
		{Path: authorityPathDescription, Source: sources.ProvidersID, Priority: 70},

		// Lifecycle status - models.dev is authoritative until provider
		// availability fields are mapped into the canonical status enum.
		{Path: authorityPathStatus, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathStatus, Source: sources.ModelsDevGitID, Priority: 85},
		{Path: authorityPathStatus, Source: sources.ProvidersID, Priority: 70},

		// Lineage - models.dev is best for family; provider APIs can fill root/parent.
		{Path: authorityPathLineage, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathLineage, Source: sources.ModelsDevGitID, Priority: 85},
		{Path: authorityPathLineage, Source: sources.ProvidersID, Priority: 80},

		// Modes - models.dev currently provides mode-specific pricing and request overrides.
		{Path: authorityPathModes, Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: authorityPathModes, Source: sources.ModelsDevGitID, Priority: 85},
		{Path: authorityPathModes, Source: sources.ProvidersID, Priority: 70},

		// Core identity - Provider API is authoritative for names
		{Path: authorityPathName, Source: sources.ProvidersID, Priority: 90},
		{Path: authorityPathName, Source: sources.ModelsDevHTTPID, Priority: 85},
		{Path: authorityPathName, Source: sources.ModelsDevGitID, Priority: 80},
		{Path: authorityPathName, Source: sources.LocalCatalogID, Priority: 75},

		// Authors field
		{Path: authorityPathAuthors, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathAuthors, Source: sources.ModelsDevHTTPID, Priority: 80},
		{Path: authorityPathAuthors, Source: sources.ModelsDevGitID, Priority: 75},
		{Path: authorityPathAuthors, Source: sources.ProvidersID, Priority: 70},
	}
}

// defaultProviderAuthorities returns the default field authorities for providers.
func defaultProviderAuthorities() []Field {
	return []Field{
		// Executable acquisition configuration is owned by the local catalog.
		{Path: "Credentials", Source: sources.LocalCatalogID, Priority: 100},
		{Path: "Credentials.*", Source: sources.LocalCatalogID, Priority: 100},
		{Path: "Advisories", Source: sources.LocalCatalogID, Priority: 100},
		{Path: authorityPathCatalog, Source: sources.LocalCatalogID, Priority: 95},
		{Path: "Catalog.*", Source: sources.LocalCatalogID, Priority: 95},
		{Path: "Invocation", Source: sources.LocalCatalogID, Priority: 95},
		{Path: "Invocation.*", Source: sources.LocalCatalogID, Priority: 95},

		// Core info - prefer manual edits (using Go field names)
		{Path: authorityPathName, Source: sources.LocalCatalogID, Priority: 90},
		{Path: authorityPathHeadquarters, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathIconURL, Source: sources.LocalCatalogID, Priority: 85},

		// Policies - models.dev or manual (HTTP preferred, using Go field names)
		{Path: "PrivacyPolicy", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "PrivacyPolicy.*", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "RetentionPolicy", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "RetentionPolicy.*", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "GovernancePolicy", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "GovernancePolicy.*", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "GovernancePolicy.ModerationRequired", Source: sources.ModelsDevHTTPID, Priority: 85},
		{Path: "PrivacyPolicy", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "PrivacyPolicy.*", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "RetentionPolicy", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "RetentionPolicy.*", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "GovernancePolicy", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "GovernancePolicy.*", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "GovernancePolicy.ModerationRequired", Source: sources.ModelsDevGitID, Priority: 80},

		// Status page - prefer local catalog (using Go field name)
		{Path: "StatusPageURL", Source: sources.LocalCatalogID, Priority: 85},

		// Aliases - prefer local catalog (using Go field name)
		{Path: authorityPathAliases, Source: sources.LocalCatalogID, Priority: 85},

		// Runtime model maps are populated from provider/catalog sources.
		{Path: authorityPathModels, Source: sources.ProvidersID, Priority: 90},
		{Path: authorityPathModels, Source: sources.LocalCatalogID, Priority: 80},
	}
}

// defaultAuthorAuthorities returns the default field authorities for authors.
func defaultAuthorAuthorities() []Field {
	return []Field{
		// Core author info - prefer local catalog for stability
		// Using capitalized field names to match Go struct fields
		{Path: authorityPathName, Source: sources.LocalCatalogID, Priority: 90},
		{Path: authorityPathDescription, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathHeadquarters, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathIconURL, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathWebsite, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathHuggingFace, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathGitHub, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathTwitter, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathCatalog, Source: sources.LocalCatalogID, Priority: 85},
		{Path: authorityPathModels, Source: sources.LocalCatalogID, Priority: 85},

		// Aliases - prefer local catalog (using Go field name)
		{Path: authorityPathAliases, Source: sources.LocalCatalogID, Priority: 85},

		// Fallback to models.dev
		{Path: authorityPathName, Source: sources.ModelsDevHTTPID, Priority: 80},
		{Path: authorityPathDescription, Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: authorityPathHeadquarters, Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: authorityPathIconURL, Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: authorityPathWebsite, Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: authorityPathHuggingFace, Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: authorityPathGitHub, Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: authorityPathTwitter, Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: authorityPathCatalog, Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: authorityPathModels, Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: authorityPathName, Source: sources.ModelsDevGitID, Priority: 70},
		{Path: authorityPathDescription, Source: sources.ModelsDevGitID, Priority: 65},
		{Path: authorityPathHeadquarters, Source: sources.ModelsDevGitID, Priority: 65},
		{Path: authorityPathIconURL, Source: sources.ModelsDevGitID, Priority: 65},
		{Path: authorityPathWebsite, Source: sources.ModelsDevGitID, Priority: 65},
		{Path: authorityPathHuggingFace, Source: sources.ModelsDevGitID, Priority: 65},
		{Path: authorityPathGitHub, Source: sources.ModelsDevGitID, Priority: 65},
		{Path: authorityPathTwitter, Source: sources.ModelsDevGitID, Priority: 65},
		{Path: authorityPathCatalog, Source: sources.ModelsDevGitID, Priority: 65},
		{Path: authorityPathModels, Source: sources.ModelsDevGitID, Priority: 65},
	}
}
