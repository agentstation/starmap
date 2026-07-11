// Package authority manages source authority for catalog data reconciliation.
package authority

import (
	"path/filepath"

	"github.com/agentstation/starmap/pkg/sources"
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
		{Path: "Pricing", Source: sources.ProvidersID, Priority: 110},
		{Path: "Pricing", Source: sources.ModelsDevHTTPID, Priority: 100},
		{Path: "Pricing", Source: sources.ModelsDevGitID, Priority: 90},
		{Path: "Pricing", Source: sources.LocalCatalogID, Priority: 80},

		// Availability - Provider API is truth
		{Path: "Features", Source: sources.ProvidersID, Priority: 95},
		{Path: "Features", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "Features", Source: sources.ModelsDevGitID, Priority: 85},

		// Capability substructures - Provider API wins when it has explicit data;
		// models.dev fills gaps for providers with sparse /models responses.
		{Path: "Attachments", Source: sources.ProvidersID, Priority: 95},
		{Path: "Attachments", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "Attachments", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "Reasoning", Source: sources.ProvidersID, Priority: 95},
		{Path: "Reasoning", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "Reasoning", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "ReasoningTokens", Source: sources.ProvidersID, Priority: 95},
		{Path: "ReasoningTokens", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "ReasoningTokens", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "Verbosity", Source: sources.ProvidersID, Priority: 95},
		{Path: "Verbosity", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "Verbosity", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "Tools", Source: sources.ProvidersID, Priority: 95},
		{Path: "Tools", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "Tools", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "Delivery", Source: sources.ProvidersID, Priority: 95},
		{Path: "Delivery", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "Delivery", Source: sources.ModelsDevGitID, Priority: 85},

		// Limits - models.dev has better data (HTTP preferred)
		{Path: "Limits", Source: sources.ModelsDevHTTPID, Priority: 100},
		{Path: "Limits", Source: sources.ModelsDevGitID, Priority: 90},
		{Path: "Limits", Source: sources.ProvidersID, Priority: 85},

		// Metadata - models.dev is authoritative (HTTP preferred)
		{Path: "Metadata", Source: sources.ModelsDevHTTPID, Priority: 110},
		{Path: "Metadata", Source: sources.ModelsDevGitID, Priority: 100},
		{Path: "Metadata", Source: sources.ProvidersID, Priority: 80},

		// Generation parameters - Provider API for current settings
		{Path: "Generation", Source: sources.ProvidersID, Priority: 85},
		{Path: "Generation", Source: sources.ModelsDevHTTPID, Priority: 80},
		{Path: "Generation", Source: sources.ModelsDevGitID, Priority: 75},

		// Descriptions - prefer manual edits, then models.dev
		{Path: "Description", Source: sources.LocalCatalogID, Priority: 90},
		{Path: "Description", Source: sources.ModelsDevHTTPID, Priority: 85},
		{Path: "Description", Source: sources.ModelsDevGitID, Priority: 80},
		{Path: "Description", Source: sources.ProvidersID, Priority: 70},

		// Lifecycle status - models.dev is authoritative until provider
		// availability fields are mapped into the canonical status enum.
		{Path: "Status", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "Status", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "Status", Source: sources.ProvidersID, Priority: 70},

		// Lineage - models.dev is best for family; provider APIs can fill root/parent.
		{Path: "Lineage", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "Lineage", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "Lineage", Source: sources.ProvidersID, Priority: 80},

		// Modes - models.dev currently provides mode-specific pricing and request overrides.
		{Path: "Modes", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "Modes", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "Modes", Source: sources.ProvidersID, Priority: 70},

		// Core identity - Provider API is authoritative for names
		{Path: "Name", Source: sources.ProvidersID, Priority: 90},
		{Path: "Name", Source: sources.ModelsDevHTTPID, Priority: 85},
		{Path: "Name", Source: sources.ModelsDevGitID, Priority: 80},
		{Path: "Name", Source: sources.LocalCatalogID, Priority: 75},

		// Authors field
		{Path: "Authors", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "Authors", Source: sources.ModelsDevHTTPID, Priority: 80},
		{Path: "Authors", Source: sources.ModelsDevGitID, Priority: 75},
		{Path: "Authors", Source: sources.ProvidersID, Priority: 70},
	}
}

// defaultProviderAuthorities returns the default field authorities for providers.
func defaultProviderAuthorities() []Field {
	return []Field{
		// API configuration - local catalog for stability (using Go field names)
		{Path: "APIKey", Source: sources.LocalCatalogID, Priority: 100},
		{Path: "APIKey.*", Source: sources.LocalCatalogID, Priority: 100},
		{Path: "EnvVars", Source: sources.LocalCatalogID, Priority: 100},
		{Path: "EnvVars", Source: sources.ModelsDevHTTPID, Priority: 90},
		{Path: "EnvVars", Source: sources.ModelsDevGitID, Priority: 85},
		{Path: "EnvVars", Source: sources.ProvidersID, Priority: 80},
		{Path: "Catalog", Source: sources.LocalCatalogID, Priority: 95},
		{Path: "Catalog.*", Source: sources.LocalCatalogID, Priority: 95},
		{Path: "ChatCompletions", Source: sources.LocalCatalogID, Priority: 95},
		{Path: "ChatCompletions.URL", Source: sources.LocalCatalogID, Priority: 95},
		{Path: "ChatCompletions.HealthAPIURL", Source: sources.LocalCatalogID, Priority: 90},

		// Core info - prefer manual edits (using Go field names)
		{Path: "Name", Source: sources.LocalCatalogID, Priority: 90},
		{Path: "Headquarters", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "IconURL", Source: sources.LocalCatalogID, Priority: 85},

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
		{Path: "Aliases", Source: sources.LocalCatalogID, Priority: 85},

		// Runtime model maps are populated from provider/catalog sources.
		{Path: "Models", Source: sources.ProvidersID, Priority: 90},
		{Path: "Models", Source: sources.LocalCatalogID, Priority: 80},
	}
}

// defaultAuthorAuthorities returns the default field authorities for authors.
func defaultAuthorAuthorities() []Field {
	return []Field{
		// Core author info - prefer local catalog for stability
		// Using capitalized field names to match Go struct fields
		{Path: "Name", Source: sources.LocalCatalogID, Priority: 90},
		{Path: "Description", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "Headquarters", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "IconURL", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "Website", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "HuggingFace", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "GitHub", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "Twitter", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "Catalog", Source: sources.LocalCatalogID, Priority: 85},
		{Path: "Models", Source: sources.LocalCatalogID, Priority: 85},

		// Aliases - prefer local catalog (using Go field name)
		{Path: "Aliases", Source: sources.LocalCatalogID, Priority: 85},

		// Fallback to models.dev
		{Path: "Name", Source: sources.ModelsDevHTTPID, Priority: 80},
		{Path: "Description", Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: "Headquarters", Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: "IconURL", Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: "Website", Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: "HuggingFace", Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: "GitHub", Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: "Twitter", Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: "Catalog", Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: "Models", Source: sources.ModelsDevHTTPID, Priority: 75},
		{Path: "Name", Source: sources.ModelsDevGitID, Priority: 70},
		{Path: "Description", Source: sources.ModelsDevGitID, Priority: 65},
		{Path: "Headquarters", Source: sources.ModelsDevGitID, Priority: 65},
		{Path: "IconURL", Source: sources.ModelsDevGitID, Priority: 65},
		{Path: "Website", Source: sources.ModelsDevGitID, Priority: 65},
		{Path: "HuggingFace", Source: sources.ModelsDevGitID, Priority: 65},
		{Path: "GitHub", Source: sources.ModelsDevGitID, Priority: 65},
		{Path: "Twitter", Source: sources.ModelsDevGitID, Priority: 65},
		{Path: "Catalog", Source: sources.ModelsDevGitID, Priority: 65},
		{Path: "Models", Source: sources.ModelsDevGitID, Priority: 65},
	}
}
