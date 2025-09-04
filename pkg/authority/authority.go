package authority

import (
	"path/filepath"

	"github.com/agentstation/starmap/pkg/sources"
)

// Authority determines which source is authoritative for each field
type Authority interface {
	// Find returns the authority configuration for a specific field
	Find(fieldPath string, resourceType sources.ResourceType) *Field

	// List returns all authorities for a resource type
	List(resourceType sources.ResourceType) []Field
}

// Field defines source priority for a specific field
type Field struct {
	Path     string       `json:"path" yaml:"path"`         // e.g., "pricing.input", "metadata.knowledge_cutoff"
	Source   sources.Type `json:"source" yaml:"source"`     // Which source is authoritative
	Priority int          `json:"priority" yaml:"priority"` // Priority (higher = more authoritative)
}

// authorities provides standard field authorities
type authorities struct {
	modelAuthorities    []Field
	providerAuthorities []Field
	authorAuthorities   []Field
}

// New creates a new DefaultAuthorities with standard configurations
func New() Authority {
	return &authorities{
		modelAuthorities:    defaultModelAuthorities(),
		providerAuthorities: defaultProviderAuthorities(),
		authorAuthorities:   defaultAuthorAuthorities(),
	}
}

// Find returns the authority configuration for a specific field
func (da *authorities) Find(fieldPath string, resourceType sources.ResourceType) *Field {
	var authorities []Field

	switch resourceType {
	case sources.ResourceTypeModel:
		authorities = da.modelAuthorities
	case sources.ResourceTypeProvider:
		authorities = da.providerAuthorities
	case sources.ResourceTypeAuthor:
		authorities = da.authorAuthorities
	default:
		return nil
	}

	return ByField(fieldPath, authorities)
}

// List returns all authorities for a resource type
func (da *authorities) List(resourceType sources.ResourceType) []Field {
	switch resourceType {
	case sources.ResourceTypeModel:
		return da.modelAuthorities
	case sources.ResourceTypeProvider:
		return da.providerAuthorities
	case sources.ResourceTypeAuthor:
		return da.authorAuthorities
	default:
		return nil
	}
}

// ByField returns the highest priority authority for a given field path
func ByField(fieldPath string, authorities []Field) *Field {
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

// MatchesPattern checks if a field path matches a pattern (supports * wildcards)
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

// FilterAuthoritiesBySource returns only the authorities for a specific source
func FilterAuthoritiesBySource(authorities []Field, sourceType sources.Type) []Field {
	var filtered []Field
	for _, auth := range authorities {
		if auth.Source == sourceType {
			filtered = append(filtered, auth)
		}
	}
	return filtered
}

// defaultModelAuthorities returns the default field authorities for models
func defaultModelAuthorities() []Field {
	return []Field{
		// Pricing - models.dev is most reliable (HTTP preferred for speed)
		// Using capitalized field names to match Go struct fields
		{Path: "Pricing", Source: sources.ModelsDevHTTP, Priority: 110},
		{Path: "Pricing", Source: sources.ModelsDevGit, Priority: 100},

		// Availability - Provider API is truth
		{Path: "Features", Source: sources.ProviderAPI, Priority: 95},
		{Path: "Features", Source: sources.ModelsDevHTTP, Priority: 90},
		{Path: "Features", Source: sources.ModelsDevGit, Priority: 85},

		// Limits - models.dev has better data (HTTP preferred)
		{Path: "Limits", Source: sources.ModelsDevHTTP, Priority: 100},
		{Path: "Limits", Source: sources.ModelsDevGit, Priority: 90},
		{Path: "Limits", Source: sources.ProviderAPI, Priority: 85},

		// Metadata - models.dev is authoritative (HTTP preferred)
		{Path: "Metadata", Source: sources.ModelsDevHTTP, Priority: 110},
		{Path: "Metadata", Source: sources.ModelsDevGit, Priority: 100},
		{Path: "Metadata", Source: sources.ProviderAPI, Priority: 80},

		// Generation parameters - Provider API for current settings
		{Path: "Generation", Source: sources.ProviderAPI, Priority: 85},
		{Path: "Generation", Source: sources.ModelsDevHTTP, Priority: 80},
		{Path: "Generation", Source: sources.ModelsDevGit, Priority: 75},

		// Descriptions - prefer manual edits, then models.dev
		{Path: "Description", Source: sources.LocalCatalog, Priority: 90},
		{Path: "Description", Source: sources.ModelsDevHTTP, Priority: 85},
		{Path: "Description", Source: sources.ModelsDevGit, Priority: 80},
		{Path: "Description", Source: sources.ProviderAPI, Priority: 70},

		// Core identity - Provider API is authoritative for names
		{Path: "Name", Source: sources.ProviderAPI, Priority: 90},
		{Path: "Name", Source: sources.ModelsDevHTTP, Priority: 85},
		{Path: "Name", Source: sources.ModelsDevGit, Priority: 80},
		{Path: "Name", Source: sources.LocalCatalog, Priority: 75},

		// Authors field
		{Path: "Authors", Source: sources.LocalCatalog, Priority: 85},
		{Path: "Authors", Source: sources.ModelsDevHTTP, Priority: 80},
		{Path: "Authors", Source: sources.ModelsDevGit, Priority: 75},
		{Path: "Authors", Source: sources.ProviderAPI, Priority: 70},
	}
}

// defaultProviderAuthorities returns the default field authorities for providers
func defaultProviderAuthorities() []Field {
	return []Field{
		// API configuration - local catalog for stability (using Go field names)
		{Path: "APIKey.*", Source: sources.LocalCatalog, Priority: 100},
		{Path: "Catalog.*", Source: sources.LocalCatalog, Priority: 95},
		{Path: "ChatCompletions.URL", Source: sources.LocalCatalog, Priority: 95},
		{Path: "ChatCompletions.HealthAPIURL", Source: sources.LocalCatalog, Priority: 90},

		// Core info - prefer manual edits (using Go field names)
		{Path: "Name", Source: sources.LocalCatalog, Priority: 90},
		{Path: "Headquarters", Source: sources.LocalCatalog, Priority: 85},
		{Path: "IconURL", Source: sources.LocalCatalog, Priority: 85},

		// Policies - models.dev or manual (HTTP preferred, using Go field names)
		{Path: "PrivacyPolicy.*", Source: sources.ModelsDevHTTP, Priority: 90},
		{Path: "RetentionPolicy.*", Source: sources.ModelsDevHTTP, Priority: 90},
		{Path: "GovernancePolicy.*", Source: sources.ModelsDevHTTP, Priority: 90},
		{Path: "GovernancePolicy.ModerationRequired", Source: sources.ModelsDevHTTP, Priority: 85},
		{Path: "PrivacyPolicy.*", Source: sources.ModelsDevGit, Priority: 85},
		{Path: "RetentionPolicy.*", Source: sources.ModelsDevGit, Priority: 85},
		{Path: "GovernancePolicy.*", Source: sources.ModelsDevGit, Priority: 85},
		{Path: "GovernancePolicy.ModerationRequired", Source: sources.ModelsDevGit, Priority: 80},

		// Status page - prefer local catalog (using Go field name)
		{Path: "StatusPageURL", Source: sources.LocalCatalog, Priority: 85},

		// Aliases - prefer local catalog (using Go field name)
		{Path: "Aliases", Source: sources.LocalCatalog, Priority: 85},
	}
}

// defaultAuthorAuthorities returns the default field authorities for authors
func defaultAuthorAuthorities() []Field {
	return []Field{
		// Core author info - prefer local catalog for stability
		// Using capitalized field names to match Go struct fields
		{Path: "Name", Source: sources.LocalCatalog, Priority: 90},
		{Path: "URL", Source: sources.LocalCatalog, Priority: 85},
		{Path: "Description", Source: sources.LocalCatalog, Priority: 85},

		// Fallback to models.dev
		{Path: "Name", Source: sources.ModelsDevHTTP, Priority: 80},
		{Path: "URL", Source: sources.ModelsDevHTTP, Priority: 75},
		{Path: "Description", Source: sources.ModelsDevHTTP, Priority: 75},
		{Path: "Name", Source: sources.ModelsDevGit, Priority: 70},
		{Path: "URL", Source: sources.ModelsDevGit, Priority: 65},
		{Path: "Description", Source: sources.ModelsDevGit, Priority: 65},
	}
}
