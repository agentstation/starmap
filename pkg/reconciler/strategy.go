package reconciler

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// StrategyType represents the type of reconciliation strategy.
type StrategyType string

// String returns the string representation of a strategy type.
func (s StrategyType) String() string {
	return string(s)
}

// Name returns the name of the strategy type.
func (s StrategyType) Name() string {
	str := s.String()
	// Replace hyphens with spaces and title case each word
	words := strings.Split(str, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

const (
	// StrategyTypeFieldAuthority uses field-specific authority scores to resolve conflicts.
	StrategyTypeFieldAuthority StrategyType = "field-authority"
	// StrategyTypeSourceOrder uses source ordering to resolve conflicts.
	StrategyTypeSourceOrder StrategyType = "source-order"
)

// Strategy defines how reconciliation should be performed.
type Strategy interface {
	// Type returns the strategy type
	Type() StrategyType

	// Description returns a human-readable description
	Description() string

	// ShouldMerge determines if resources should be merged
	ShouldMerge(resourceType sources.ResourceType) bool

	// ResolveConflict determines how to resolve conflicts
	ResolveConflict(field string, values map[sources.ID]any) (any, sources.ID, string)

	// ValidateResult validates the reconciliation result
	ValidateResult(result *Result) error

	// ApplyStrategy returns how changes should be applied
	ApplyStrategy() differ.ApplyStrategy
}

// baseStrategy provides common strategy functionality.
type baseStrategy struct {
	typ            StrategyType
	description    string
	applyStrategy  differ.ApplyStrategy
	mergeResources map[sources.ResourceType]bool
}

// Type returns the strategy type.
func (s *baseStrategy) Type() StrategyType {
	return s.typ
}

// Description returns a human-readable description.
func (s *baseStrategy) Description() string {
	return s.description
}

// ShouldMerge determines if resources should be merged.
func (s *baseStrategy) ShouldMerge(resourceType sources.ResourceType) bool {
	return s.mergeResources[resourceType]
}

// ApplyStrategy returns how changes should be applied.
func (s *baseStrategy) ApplyStrategy() differ.ApplyStrategy {
	return s.applyStrategy
}

// ValidateResult validates the reconciliation result.
func (s *baseStrategy) ValidateResult(result *Result) error {
	if result == nil {
		return &errors.ValidationError{
			Field:   "result",
			Message: "cannot be nil",
		}
	}
	return nil
}

// AuthorityStrategy uses field authorities to resolve conflicts.
type AuthorityStrategy struct {
	baseStrategy
	authorities authority.Authority
}

// NewAuthorityStrategy creates a new authority-based strategy.
func NewAuthorityStrategy(authorities authority.Authority) Strategy {
	return &AuthorityStrategy{
		baseStrategy: baseStrategy{
			typ:           StrategyTypeFieldAuthority,
			description:   "Resolves conflicts using field authority priorities",
			applyStrategy: differ.ApplyAdditive,
			mergeResources: map[sources.ResourceType]bool{
				sources.ResourceTypeModel:    true,
				sources.ResourceTypeProvider: true,
				sources.ResourceTypeAuthor:   true,
			},
		},
		authorities: authorities,
	}
}

// ResolveConflict uses authorities to resolve conflicts.
func (s *AuthorityStrategy) ResolveConflict(field string, values map[sources.ID]any) (any, sources.ID, string) {
	// Get all authorities for this resource type
	authorities := s.authorities.ModelFields()

	// Find all authorities that match this field, sorted by priority
	var matchingAuthorities []authority.Field
	for _, auth := range authorities {
		if authority.MatchesPattern(field, auth.Path) {
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

	// Filter authorities to only those with available sources
	var availableAuthorities []authority.Field
	for _, auth := range matchingAuthorities {
		if _, exists := values[auth.Source]; exists {
			availableAuthorities = append(availableAuthorities, auth)
		}
	}

	// Try authorities in priority order
	for _, authority := range availableAuthorities {
		if value := values[authority.Source]; value != nil && value != "" {
			return value, authority.Source, fmt.Sprintf("selected by authority (priority: %d)", authority.Priority)
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

// SourceOrderStrategy resolves conflicts using a fixed source precedence order.
// Sources earlier in the priority slice have higher precedence than sources later in the slice.
type SourceOrderStrategy struct {
	baseStrategy
	sourcePriorityOrder []sources.ID // First element = highest priority
}

// NewSourceOrderStrategy creates a new source priority order strategy.
// The priorityOrder slice determines precedence: earlier elements have higher priority.
func NewSourceOrderStrategy(priorityOrder []sources.ID) Strategy {
	return &SourceOrderStrategy{
		baseStrategy: baseStrategy{
			typ:           StrategyTypeSourceOrder,
			description:   fmt.Sprintf("Resolves conflicts using source priority order: %v", priorityOrder),
			applyStrategy: differ.ApplyAdditive,
			mergeResources: map[sources.ResourceType]bool{
				sources.ResourceTypeModel:    true,
				sources.ResourceTypeProvider: true,
				sources.ResourceTypeAuthor:   true,
			},
		},
		sourcePriorityOrder: priorityOrder,
	}
}

// ResolveConflict uses source priority order to resolve conflicts.
func (s *SourceOrderStrategy) ResolveConflict(_ string, values map[sources.ID]any) (any, sources.ID, string) {
	// Check sources in priority order
	for _, source := range s.sourcePriorityOrder {
		if value, exists := values[source]; exists {
			if value != nil && value != "" {
				return value, source, fmt.Sprintf("selected by source priority order (%s)", source)
			}
		}
	}

	// No priority source found, use first available non-empty value
	for source, value := range values {
		if value != nil && value != "" {
			return value, source, "no priority source available, using first non-empty"
		}
	}

	// Return any value as last resort
	for source, value := range values {
		return value, source, "using first available value"
	}

	return nil, "", "no value available"
}
