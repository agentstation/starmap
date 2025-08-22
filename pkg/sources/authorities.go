package sources

import (
	"path/filepath"
)

// FieldAuthority represents which source is authoritative for a specific field
type FieldAuthority struct {
	FieldPath string `json:"field_path" yaml:"field_path"` // e.g., "pricing.input", "metadata.knowledge_cutoff"
	Source    Type   `json:"source" yaml:"source"`         // Source type that is authoritative
	Priority  int    `json:"priority" yaml:"priority"`     // Priority for this field (higher = more authoritative)
}

// DefaultModelFieldAuthorities defines the default field authorities for models
var DefaultModelFieldAuthorities = []FieldAuthority{
	// Pricing - models.dev is most reliable (HTTP preferred for speed)
	{FieldPath: "pricing.input", Source: ModelsDevHTTP, Priority: 110},
	{FieldPath: "pricing.output", Source: ModelsDevHTTP, Priority: 110},
	{FieldPath: "pricing.caching.*", Source: ModelsDevHTTP, Priority: 105},
	{FieldPath: "pricing.batch", Source: ModelsDevHTTP, Priority: 105},
	{FieldPath: "pricing.image", Source: ModelsDevHTTP, Priority: 105},
	// Fallback to git version
	{FieldPath: "pricing.input", Source: ModelsDevGit, Priority: 100},
	{FieldPath: "pricing.output", Source: ModelsDevGit, Priority: 100},
	{FieldPath: "pricing.caching.*", Source: ModelsDevGit, Priority: 95},
	{FieldPath: "pricing.batch", Source: ModelsDevGit, Priority: 95},
	{FieldPath: "pricing.image", Source: ModelsDevGit, Priority: 95},

	// Availability - Provider API is truth
	{FieldPath: "features.available", Source: ProviderAPI, Priority: 100},
	{FieldPath: "features.deprecated", Source: ProviderAPI, Priority: 100},

	// Current capabilities - Provider API knows best
	{FieldPath: "features.chat", Source: ProviderAPI, Priority: 95},
	{FieldPath: "features.completion", Source: ProviderAPI, Priority: 95},
	{FieldPath: "features.embedding", Source: ProviderAPI, Priority: 95},
	{FieldPath: "features.vision", Source: ProviderAPI, Priority: 95},
	{FieldPath: "features.audio", Source: ProviderAPI, Priority: 95},
	{FieldPath: "features.reasoning", Source: ProviderAPI, Priority: 95},
	{FieldPath: "features.function_calling", Source: ProviderAPI, Priority: 95},

	// Limits - models.dev has better data (HTTP preferred)
	{FieldPath: "limits.context_window", Source: ModelsDevHTTP, Priority: 100},
	{FieldPath: "limits.max_output", Source: ModelsDevHTTP, Priority: 100},
	{FieldPath: "limits.context_window", Source: ModelsDevGit, Priority: 90},
	{FieldPath: "limits.max_output", Source: ModelsDevGit, Priority: 90},
	{FieldPath: "limits.request_timeout", Source: ProviderAPI, Priority: 85},

	// Metadata - models.dev is authoritative (HTTP preferred)
	{FieldPath: "metadata.knowledge_cutoff", Source: ModelsDevHTTP, Priority: 110},
	{FieldPath: "metadata.release_date", Source: ModelsDevHTTP, Priority: 105},
	{FieldPath: "metadata.open_weights", Source: ModelsDevHTTP, Priority: 100},
	{FieldPath: "metadata.tags", Source: ModelsDevHTTP, Priority: 95},
	{FieldPath: "metadata.architecture.*", Source: ModelsDevHTTP, Priority: 100},
	{FieldPath: "metadata.knowledge_cutoff", Source: ModelsDevGit, Priority: 100},
	{FieldPath: "metadata.release_date", Source: ModelsDevGit, Priority: 95},
	{FieldPath: "metadata.open_weights", Source: ModelsDevGit, Priority: 90},
	{FieldPath: "metadata.tags", Source: ModelsDevGit, Priority: 85},
	{FieldPath: "metadata.architecture.*", Source: ModelsDevGit, Priority: 90},

	// Generation parameters - Provider API for current settings
	{FieldPath: "generation.*", Source: ProviderAPI, Priority: 85},

	// Descriptions - prefer manual edits, then models.dev
	{FieldPath: "description", Source: LocalCatalog, Priority: 90},
	{FieldPath: "description", Source: ModelsDevHTTP, Priority: 85},
	{FieldPath: "description", Source: ModelsDevGit, Priority: 80},
	{FieldPath: "description", Source: ProviderAPI, Priority: 70},

	// Core identity - local catalog preserves manual corrections
	{FieldPath: "name", Source: LocalCatalog, Priority: 85},
	{FieldPath: "name", Source: ModelsDevHTTP, Priority: 83},
	{FieldPath: "name", Source: ModelsDevGit, Priority: 80},
	{FieldPath: "name", Source: ProviderAPI, Priority: 75},
}

// DefaultProviderFieldAuthorities defines the default field authorities for providers
var DefaultProviderFieldAuthorities = []FieldAuthority{
	// API configuration - local catalog for stability
	{FieldPath: "api_key.*", Source: LocalCatalog, Priority: 100},
	{FieldPath: "catalog.*", Source: LocalCatalog, Priority: 95},
	{FieldPath: "chat_completions.url", Source: LocalCatalog, Priority: 95},
	{FieldPath: "chat_completions.health_api_url", Source: LocalCatalog, Priority: 90},

	// Core info - prefer manual edits
	{FieldPath: "name", Source: LocalCatalog, Priority: 90},
	{FieldPath: "headquarters", Source: LocalCatalog, Priority: 85},
	{FieldPath: "icon_url", Source: LocalCatalog, Priority: 85},

	// Policies - models.dev or manual (HTTP preferred)
	{FieldPath: "privacy_policy.*", Source: ModelsDevHTTP, Priority: 90},
	{FieldPath: "retention_policy.*", Source: ModelsDevHTTP, Priority: 90},
	{FieldPath: "governance_policy.*", Source: ModelsDevHTTP, Priority: 90},
	{FieldPath: "governance_policy.moderation_required", Source: ModelsDevHTTP, Priority: 85},
	{FieldPath: "privacy_policy.*", Source: ModelsDevGit, Priority: 85},
	{FieldPath: "retention_policy.*", Source: ModelsDevGit, Priority: 85},
	{FieldPath: "governance_policy.*", Source: ModelsDevGit, Priority: 85},
	{FieldPath: "governance_policy.moderation_required", Source: ModelsDevGit, Priority: 80},

	// Status page - prefer local catalog
	{FieldPath: "status_page_url", Source: LocalCatalog, Priority: 85},
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

// GetAuthorityForField returns the highest priority authority for a given field path
func GetAuthorityForField(fieldPath string, authorities []FieldAuthority) *FieldAuthority {
	var bestMatch *FieldAuthority
	var bestPriority int
	var bestMatchLength int

	for i, auth := range authorities {
		if MatchesPattern(fieldPath, auth.FieldPath) {
			// Prioritize by: 1) priority, 2) pattern specificity (length), 3) order
			patternLength := len(auth.FieldPath)
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

// FilterAuthoritiesBySource returns only the authorities for a specific source
func FilterAuthoritiesBySource(authorities []FieldAuthority, sourceType Type) []FieldAuthority {
	var filtered []FieldAuthority
	for _, auth := range authorities {
		if auth.Source == sourceType {
			filtered = append(filtered, auth)
		}
	}
	return filtered
}
