package reconciler

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/agentstation/starmap/internal/catalog/authority"
	"github.com/agentstation/starmap/pkg/sources"
)

// AuthorityStrategy resolves every conflict through the canonical field
// authority table and deterministic fallbacks.
type AuthorityStrategy struct {
	authorities authority.Reader
}

// NewAuthorityStrategy creates the canonical authority strategy.
func NewAuthorityStrategy(authorities authority.Reader) *AuthorityStrategy {
	return &AuthorityStrategy{authorities: authorities}
}

// ResolveConflict resolves a model field conflict.
func (s *AuthorityStrategy) ResolveConflict(
	field string,
	values map[sources.ID]any,
) (any, sources.ID, string) {
	return s.ResolveResourceConflict(sources.ResourceTypeModel, field, values)
}

// ResolveResourceConflict resolves a resource field conflict.
func (s *AuthorityStrategy) ResolveResourceConflict(
	resourceType sources.ResourceType,
	field string,
	values map[sources.ID]any,
) (any, sources.ID, string) {
	if policy, found := s.authorities.Find(resourceType, field); found {
		for index, source := range policy.SourceOrder {
			if value, exists := values[source]; exists && policyAccepts(policy, value) {
				return value, source, fmt.Sprintf(
					"selected by %s policy (source rank: %d)",
					policy.Path,
					index+1,
				)
			}
		}
	}

	for _, source := range sortedValueSources(values) {
		value := values[source]
		if value != nil && value != "" {
			return value, source, "using deterministic non-empty fallback (no authority match)"
		}
	}
	for _, source := range sortedValueSources(values) {
		return values[source], source, "using deterministic available fallback"
	}
	return nil, "", "no value available"
}

func policyAccepts(policy authority.Policy, value any) bool {
	if value == nil {
		return false
	}
	if policy.Empty == authority.EmptyAuthoritative {
		return true
	}
	if value == "" {
		return false
	}
	return !reflect.ValueOf(value).IsZero()
}

func sortedValueSources(values map[sources.ID]any) []sources.ID {
	ids := make([]sources.ID, 0, len(values))
	for source := range values {
		ids = append(ids, source)
	}
	sort.Slice(ids, func(i, j int) bool { return string(ids[i]) < string(ids[j]) })
	return ids
}
