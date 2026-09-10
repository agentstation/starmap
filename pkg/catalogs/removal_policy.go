package catalogs

import (
	"cmp"
	"slices"
	"sync"
)

// CatalogRemovalPolicy contains one publisher's complete operator removal snapshot.
// Publisher identity requires authentication before a consumer enforces the policy.
type CatalogRemovalPolicy struct {
	PublisherID string                 `json:"publisher_id"`
	Targets     []CatalogRemovalTarget `json:"targets"`
}

type removalPolicyStore struct {
	mu       sync.RWMutex
	policies []CatalogRemovalPolicy
}

// RemovalPolicies returns caller-owned operator policy snapshots.
func (cat *Builder) RemovalPolicies() []CatalogRemovalPolicy {
	cat.removalPolicies.mu.RLock()
	defer cat.removalPolicies.mu.RUnlock()
	return copyRemovalPolicies(cat.removalPolicies.policies)
}

// SetRemovalPolicies validates and replaces operator snapshots as one unit.
// The caller must authorize replacement. Acquisition cannot supply these policies.
func (cat *Builder) SetRemovalPolicies(policies []CatalogRemovalPolicy) error {
	prepared, err := prepareRemovalPolicies(policies)
	if err != nil {
		return err
	}
	cat.removalPolicies.mu.Lock()
	cat.removalPolicies.policies = prepared
	cat.removalPolicies.mu.Unlock()
	return nil
}

// RemovalPolicies returns caller-owned operator policy snapshots.
func (cat *Catalog) RemovalPolicies() []CatalogRemovalPolicy {
	return cat.source.RemovalPolicies()
}

// Removals returns the immutable union of accepted operator removals.
// Scope links and publisher authority require validation before consumer queries.
func (cat *Catalog) Removals() *CatalogRemovalSet {
	return cat.removals
}

func copyRemovalPolicies(input []CatalogRemovalPolicy) []CatalogRemovalPolicy {
	result := slices.Clone(input)
	for i := range result {
		result[i].Targets = slices.Clone(result[i].Targets)
		for j := range result[i].Targets {
			if result[i].Targets[j].Scope != nil {
				scope := *result[i].Targets[j].Scope
				result[i].Targets[j].Scope = &scope
			}
		}
	}
	return result
}

func prepareRemovalPolicies(input []CatalogRemovalPolicy) ([]CatalogRemovalPolicy, error) {
	result := make([]CatalogRemovalPolicy, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, policy := range input {
		if !validMembershipIdentifier(policy.PublisherID) || seen[policy.PublisherID] {
			return nil, invalidRemovalTarget("publisher_id", "must be a unique bounded publisher identity")
		}
		seen[policy.PublisherID] = true
		set, err := NewCatalogRemovalSet(policy.Targets...)
		if err != nil {
			return nil, err
		}
		result = append(result, CatalogRemovalPolicy{PublisherID: policy.PublisherID, Targets: set.Targets()})
	}
	slices.SortFunc(result, func(a, b CatalogRemovalPolicy) int { return cmp.Compare(a.PublisherID, b.PublisherID) })
	return result, nil
}

func indexRemovalPolicies(policies []CatalogRemovalPolicy) (*CatalogRemovalSet, error) {
	var targets []CatalogRemovalTarget
	for _, policy := range policies {
		targets = append(targets, policy.Targets...)
	}
	return NewCatalogRemovalSet(targets...)
}

// mergeRemovalPolicies preserves every removal when generic inputs omit targets.
func mergeRemovalPolicies(left, right []CatalogRemovalPolicy) ([]CatalogRemovalPolicy, error) {
	combined, err := prepareRemovalPolicies(left)
	if err != nil {
		return nil, err
	}
	validated, err := prepareRemovalPolicies(right)
	if err != nil {
		return nil, err
	}
	for _, incoming := range validated {
		index := slices.IndexFunc(combined, func(policy CatalogRemovalPolicy) bool { return policy.PublisherID == incoming.PublisherID })
		if index < 0 {
			combined = append(combined, incoming)
		} else {
			combined[index].Targets = append(combined[index].Targets, incoming.Targets...)
		}
	}
	return prepareRemovalPolicies(combined)
}
