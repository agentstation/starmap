package reconcile

import (
	"fmt"
	"strings"
)

// Strategy defines how reconciliation should be performed
type Strategy interface {
	// Name returns the strategy name
	Name() string

	// Description returns a human-readable description
	Description() string

	// ShouldMerge determines if resources should be merged
	ShouldMerge(resourceType ResourceType) bool

	// ResolveConflict determines how to resolve conflicts
	ResolveConflict(field string, values map[SourceName]interface{}) (interface{}, SourceName, string)

	// ValidateResult validates the reconciliation result
	ValidateResult(result *Result) error

	// GetApplyStrategy returns how changes should be applied
	GetApplyStrategy() ApplyStrategy
}

// baseStrategy provides common strategy functionality
type baseStrategy struct {
	name           string
	description    string
	applyStrategy  ApplyStrategy
	mergeResources map[ResourceType]bool
}

// Name returns the strategy name
func (s *baseStrategy) Name() string {
	return s.name
}

// Description returns a human-readable description
func (s *baseStrategy) Description() string {
	return s.description
}

// ShouldMerge determines if resources should be merged
func (s *baseStrategy) ShouldMerge(resourceType ResourceType) bool {
	return s.mergeResources[resourceType]
}

// GetApplyStrategy returns how changes should be applied
func (s *baseStrategy) GetApplyStrategy() ApplyStrategy {
	return s.applyStrategy
}

// ValidateResult validates the reconciliation result
func (s *baseStrategy) ValidateResult(result *Result) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}
	return nil
}

// AuthorityBasedStrategy uses field authorities to resolve conflicts
type AuthorityBasedStrategy struct {
	baseStrategy
	authorities AuthorityProvider
}

// NewAuthorityBasedStrategy creates a new authority-based strategy
func NewAuthorityBasedStrategy(authorities AuthorityProvider) Strategy {
	return &AuthorityBasedStrategy{
		baseStrategy: baseStrategy{
			name:        "authority-based",
			description: "Resolves conflicts using field authority priorities",
			applyStrategy: ApplyAdditive,
			mergeResources: map[ResourceType]bool{
				ResourceTypeModel:    true,
				ResourceTypeProvider: true,
				ResourceTypeAuthor:   true,
			},
		},
		authorities: authorities,
	}
}

// ResolveConflict uses authorities to resolve conflicts
func (s *AuthorityBasedStrategy) ResolveConflict(field string, values map[SourceName]interface{}) (interface{}, SourceName, string) {
	// Get all authorities for this resource type
	allAuthorities := s.authorities.GetAuthorities(ResourceTypeModel)
	
	// Find all authorities that match this field, sorted by priority
	var matchingAuthorities []FieldAuthority
	for _, auth := range allAuthorities {
		if MatchesPattern(field, auth.FieldPath) {
			matchingAuthorities = append(matchingAuthorities, auth)
		}
	}
	
	// Sort by priority (highest first)
	for i := 0; i < len(matchingAuthorities)-1; i++ {
		for j := i + 1; j < len(matchingAuthorities); j++ {
			if matchingAuthorities[j].Priority > matchingAuthorities[i].Priority {
				matchingAuthorities[i], matchingAuthorities[j] = matchingAuthorities[j], matchingAuthorities[i]
			}
		}
	}
	
	// Try authorities in priority order
	for _, authority := range matchingAuthorities {
		if value, exists := values[authority.Source]; exists {
			if value != nil && value != "" {
				return value, authority.Source, fmt.Sprintf("selected by authority (priority: %d)", authority.Priority)
			}
		}
	}
	
	// No matching authority had a value, fallback to first non-empty value
	for source, value := range values {
		if value != nil && value != "" {
			return value, source, "using first non-empty value (no authority match)"
		}
	}

	// Return any value
	for source, value := range values {
		return value, source, "using first available value"
	}

	return nil, "", "no value available"
}

// UnionStrategy combines all values without conflict resolution
type UnionStrategy struct {
	baseStrategy
}

// NewUnionStrategy creates a new union strategy
func NewUnionStrategy() Strategy {
	return &UnionStrategy{
		baseStrategy: baseStrategy{
			name:        "union",
			description: "Combines all resources without conflict resolution",
			applyStrategy: ApplyAll,
			mergeResources: map[ResourceType]bool{
				ResourceTypeModel:    true,
				ResourceTypeProvider: true,
				ResourceTypeAuthor:   true,
			},
		},
	}
}

// ResolveConflict in union strategy returns the first non-nil value in a deterministic order
func (s *UnionStrategy) ResolveConflict(field string, values map[SourceName]interface{}) (interface{}, SourceName, string) {
	// Check sources in a deterministic order to ensure consistent results
	// Priority order: LocalCatalog > ModelsDevHTTP > ModelsDevGit > ProviderAPI > others alphabetically
	sourceOrder := []SourceName{
		LocalCatalog,
		ModelsDevHTTP,
		ModelsDevGit,
		ProviderAPI,
	}
	
	// First check known sources in order
	for _, source := range sourceOrder {
		if value, exists := values[source]; exists && value != nil && value != "" {
			return value, source, fmt.Sprintf("union strategy - first non-empty value (source: %s)", source)
		}
	}
	
	// Then check any other sources alphabetically for determinism
	var otherSources []SourceName
	for source := range values {
		found := false
		for _, knownSource := range sourceOrder {
			if source == knownSource {
				found = true
				break
			}
		}
		if !found {
			otherSources = append(otherSources, source)
		}
	}
	
	// Sort other sources alphabetically for determinism
	for i := 0; i < len(otherSources)-1; i++ {
		for j := i + 1; j < len(otherSources); j++ {
			if otherSources[j] < otherSources[i] {
				otherSources[i], otherSources[j] = otherSources[j], otherSources[i]
			}
		}
	}
	
	for _, source := range otherSources {
		if value := values[source]; value != nil && value != "" {
			return value, source, fmt.Sprintf("union strategy - first non-empty value (source: %s)", source)
		}
	}
	
	// If all values are nil/empty, return the first one we find
	for _, source := range sourceOrder {
		if value, exists := values[source]; exists {
			return value, source, "union strategy - all values empty, returning first"
		}
	}
	
	return nil, "", "no value available"
}

// SourcePriorityStrategy uses a fixed source priority order
type SourcePriorityStrategy struct {
	baseStrategy
	sourcePriority []SourceName
}

// NewSourcePriorityStrategy creates a new source priority strategy
func NewSourcePriorityStrategy(priority []SourceName) Strategy {
	return &SourcePriorityStrategy{
		baseStrategy: baseStrategy{
			name:        "source-priority",
			description: fmt.Sprintf("Resolves conflicts using source priority: %v", priority),
			applyStrategy: ApplyAdditive,
			mergeResources: map[ResourceType]bool{
				ResourceTypeModel:    true,
				ResourceTypeProvider: true,
				ResourceTypeAuthor:   true,
			},
		},
		sourcePriority: priority,
	}
}

// ResolveConflict uses source priority to resolve conflicts
func (s *SourcePriorityStrategy) ResolveConflict(field string, values map[SourceName]interface{}) (interface{}, SourceName, string) {
	// Check sources in priority order
	for _, source := range s.sourcePriority {
		if value, exists := values[source]; exists {
			return value, source, fmt.Sprintf("selected by source priority (%s)", source)
		}
	}

	// No priority source found, use first available
	for source, value := range values {
		return value, source, "no priority source available, using first"
	}

	return nil, "", "no value available"
}

// CustomStrategy allows custom conflict resolution logic
type CustomStrategy struct {
	baseStrategy
	resolver ConflictResolver
}

// ConflictResolver is a function that resolves conflicts
type ConflictResolver func(field string, values map[SourceName]interface{}) (interface{}, SourceName, string)

// NewCustomStrategy creates a new custom strategy
func NewCustomStrategy(name, description string, resolver ConflictResolver) Strategy {
	return &CustomStrategy{
		baseStrategy: baseStrategy{
			name:        name,
			description: description,
			applyStrategy: ApplyAdditive,
			mergeResources: map[ResourceType]bool{
				ResourceTypeModel:    true,
				ResourceTypeProvider: true,
				ResourceTypeAuthor:   true,
			},
		},
		resolver: resolver,
	}
}

// ResolveConflict uses custom resolver
func (s *CustomStrategy) ResolveConflict(field string, values map[SourceName]interface{}) (interface{}, SourceName, string) {
	if s.resolver != nil {
		return s.resolver(field, values)
	}
	// Fallback to first value
	for source, value := range values {
		return value, source, "custom resolver not defined, using first"
	}
	return nil, "", "no value available"
}

// StrategyChain combines multiple strategies with fallback
type StrategyChain struct {
	strategies []Strategy
}

// NewStrategyChain creates a new strategy chain
func NewStrategyChain(strategies ...Strategy) Strategy {
	return &StrategyChain{
		strategies: strategies,
	}
}

// Name returns the strategy name
func (s *StrategyChain) Name() string {
	names := []string{}
	for _, strategy := range s.strategies {
		names = append(names, strategy.Name())
	}
	return fmt.Sprintf("chain(%s)", strings.Join(names, " -> "))
}

// Description returns a human-readable description
func (s *StrategyChain) Description() string {
	return "Tries multiple strategies in order until one succeeds"
}

// ShouldMerge checks all strategies
func (s *StrategyChain) ShouldMerge(resourceType ResourceType) bool {
	for _, strategy := range s.strategies {
		if strategy.ShouldMerge(resourceType) {
			return true
		}
	}
	return false
}

// ResolveConflict tries each strategy in order
func (s *StrategyChain) ResolveConflict(field string, values map[SourceName]interface{}) (interface{}, SourceName, string) {
	for _, strategy := range s.strategies {
		value, source, reason := strategy.ResolveConflict(field, values)
		if value != nil || source != "" {
			return value, source, fmt.Sprintf("%s (via %s)", reason, strategy.Name())
		}
	}
	return nil, "", "no strategy could resolve conflict"
}

// ValidateResult validates using all strategies
func (s *StrategyChain) ValidateResult(result *Result) error {
	for _, strategy := range s.strategies {
		if err := strategy.ValidateResult(result); err != nil {
			return fmt.Errorf("%s validation failed: %w", strategy.Name(), err)
		}
	}
	return nil
}

// GetApplyStrategy returns the first strategy's apply strategy
func (s *StrategyChain) GetApplyStrategy() ApplyStrategy {
	if len(s.strategies) > 0 {
		return s.strategies[0].GetApplyStrategy()
	}
	return ApplyAdditive
}