package reconcile

import (
	"path/filepath"
)

// AuthorityProvider determines which source is authoritative for each field
type AuthorityProvider interface {
	// GetAuthority returns the authority configuration for a specific field
	GetAuthority(fieldPath string, resourceType ResourceType) *FieldAuthority
	
	// GetAuthorities returns all authorities for a resource type
	GetAuthorities(resourceType ResourceType) []FieldAuthority
}

// FieldAuthority defines source priority for a specific field
type FieldAuthority struct {
	FieldPath string     `json:"field_path" yaml:"field_path"` // e.g., "pricing.input", "metadata.knowledge_cutoff"
	Source    SourceName `json:"source" yaml:"source"`         // Which source is authoritative
	Priority  int        `json:"priority" yaml:"priority"`     // Priority (higher = more authoritative)
}

// DefaultAuthorities provides standard field authorities
type DefaultAuthorities struct {
	modelAuthorities    []FieldAuthority
	providerAuthorities []FieldAuthority
	authorAuthorities   []FieldAuthority
}

// NewDefaultAuthorities creates a new DefaultAuthorities with standard configurations
func NewDefaultAuthorities() AuthorityProvider {
	return &DefaultAuthorities{
		modelAuthorities:    defaultModelFieldAuthorities(),
		providerAuthorities: defaultProviderFieldAuthorities(),
		authorAuthorities:   defaultAuthorFieldAuthorities(),
	}
}

// NewDefaultAuthorityProvider is an alias for NewDefaultAuthorities
func NewDefaultAuthorityProvider() AuthorityProvider {
	return NewDefaultAuthorities()
}

// GetAuthority returns the authority configuration for a specific field
func (da *DefaultAuthorities) GetAuthority(fieldPath string, resourceType ResourceType) *FieldAuthority {
	var authorities []FieldAuthority
	
	switch resourceType {
	case ResourceTypeModel:
		authorities = da.modelAuthorities
	case ResourceTypeProvider:
		authorities = da.providerAuthorities
	case ResourceTypeAuthor:
		authorities = da.authorAuthorities
	default:
		return nil
	}
	
	return AuthorityByField(fieldPath, authorities)
}

// GetAuthorities returns all authorities for a resource type
func (da *DefaultAuthorities) GetAuthorities(resourceType ResourceType) []FieldAuthority {
	switch resourceType {
	case ResourceTypeModel:
		return da.modelAuthorities
	case ResourceTypeProvider:
		return da.providerAuthorities
	case ResourceTypeAuthor:
		return da.authorAuthorities
	default:
		return nil
	}
}

// AuthorityByField returns the highest priority authority for a given field path
func AuthorityByField(fieldPath string, authorities []FieldAuthority) *FieldAuthority {
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
func FilterAuthoritiesBySource(authorities []FieldAuthority, sourceName SourceName) []FieldAuthority {
	var filtered []FieldAuthority
	for _, auth := range authorities {
		if auth.Source == sourceName {
			filtered = append(filtered, auth)
		}
	}
	return filtered
}

// defaultModelFieldAuthorities returns the default field authorities for models
func defaultModelFieldAuthorities() []FieldAuthority {
	return []FieldAuthority{
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
}

// defaultProviderFieldAuthorities returns the default field authorities for providers
func defaultProviderFieldAuthorities() []FieldAuthority {
	return []FieldAuthority{
		// API configuration - local catalog for stability (using Go field names)
		{FieldPath: "APIKey.*", Source: LocalCatalog, Priority: 100},
		{FieldPath: "Catalog.*", Source: LocalCatalog, Priority: 95},
		{FieldPath: "ChatCompletions.URL", Source: LocalCatalog, Priority: 95},
		{FieldPath: "ChatCompletions.HealthAPIURL", Source: LocalCatalog, Priority: 90},

		// Core info - prefer manual edits (using Go field names)
		{FieldPath: "Name", Source: LocalCatalog, Priority: 90},
		{FieldPath: "Headquarters", Source: LocalCatalog, Priority: 85},
		{FieldPath: "IconURL", Source: LocalCatalog, Priority: 85},

		// Policies - models.dev or manual (HTTP preferred, using Go field names)
		{FieldPath: "PrivacyPolicy.*", Source: ModelsDevHTTP, Priority: 90},
		{FieldPath: "RetentionPolicy.*", Source: ModelsDevHTTP, Priority: 90},
		{FieldPath: "GovernancePolicy.*", Source: ModelsDevHTTP, Priority: 90},
		{FieldPath: "GovernancePolicy.ModerationRequired", Source: ModelsDevHTTP, Priority: 85},
		{FieldPath: "PrivacyPolicy.*", Source: ModelsDevGit, Priority: 85},
		{FieldPath: "RetentionPolicy.*", Source: ModelsDevGit, Priority: 85},
		{FieldPath: "GovernancePolicy.*", Source: ModelsDevGit, Priority: 85},
		{FieldPath: "GovernancePolicy.ModerationRequired", Source: ModelsDevGit, Priority: 80},

		// Status page - prefer local catalog (using Go field name)
		{FieldPath: "StatusPageURL", Source: LocalCatalog, Priority: 85},
		
		// Aliases - prefer local catalog (using Go field name)
		{FieldPath: "Aliases", Source: LocalCatalog, Priority: 85},
	}
}

// defaultAuthorFieldAuthorities returns the default field authorities for authors
func defaultAuthorFieldAuthorities() []FieldAuthority {
	return []FieldAuthority{
		// Core author info - prefer local catalog for stability
		{FieldPath: "name", Source: LocalCatalog, Priority: 90},
		{FieldPath: "url", Source: LocalCatalog, Priority: 85},
		{FieldPath: "description", Source: LocalCatalog, Priority: 85},
		
		// Fallback to models.dev
		{FieldPath: "name", Source: ModelsDevHTTP, Priority: 80},
		{FieldPath: "url", Source: ModelsDevHTTP, Priority: 75},
		{FieldPath: "description", Source: ModelsDevHTTP, Priority: 75},
		{FieldPath: "name", Source: ModelsDevGit, Priority: 70},
		{FieldPath: "url", Source: ModelsDevGit, Priority: 65},
		{FieldPath: "description", Source: ModelsDevGit, Priority: 65},
	}
}

// CustomAuthorities allows for custom field authority configurations
type CustomAuthorities struct {
	authorities map[ResourceType][]FieldAuthority
}

// NewCustomAuthorities creates a new CustomAuthorities
func NewCustomAuthorities() *CustomAuthorities {
	return &CustomAuthorities{
		authorities: make(map[ResourceType][]FieldAuthority),
	}
}

// AddAuthority adds a field authority for a resource type
func (ca *CustomAuthorities) AddAuthority(resourceType ResourceType, authority FieldAuthority) {
	ca.authorities[resourceType] = append(ca.authorities[resourceType], authority)
}

// GetAuthority returns the authority configuration for a specific field
func (ca *CustomAuthorities) GetAuthority(fieldPath string, resourceType ResourceType) *FieldAuthority {
	authorities, ok := ca.authorities[resourceType]
	if !ok {
		return nil
	}
	return AuthorityByField(fieldPath, authorities)
}

// GetAuthorities returns all authorities for a resource type
func (ca *CustomAuthorities) GetAuthorities(resourceType ResourceType) []FieldAuthority {
	return ca.authorities[resourceType]
}