package catalogs

import (
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/agentstation/starmap/pkg/errors"
)

// MembershipScopeKey names a binding revision within one publisher's namespace.
type MembershipScopeKey struct {
	PublisherID     string
	BindingID       string
	BindingRevision string
}

type membershipScopeStore struct {
	mu     sync.RWMutex
	scopes []ProviderMembershipScope
}

type membershipScopeIndex struct {
	provider ProviderID
	complete bool
	models   map[string]bool
}

// MembershipScopes returns independent copies of effective scope records.
func (cat *Builder) MembershipScopes() []ProviderMembershipScope {
	cat.membership.mu.RLock()
	defer cat.membership.mu.RUnlock()
	return copyMembershipScopes(cat.membership.scopes)
}

// SetMembershipScopes validates and replaces effective scope records as one unit.
func (cat *Builder) SetMembershipScopes(scopes []ProviderMembershipScope) error {
	copied, err := prepareMembershipScopes(scopes)
	if err != nil {
		return err
	}
	cat.membership.mu.Lock()
	cat.membership.scopes = copied
	cat.membership.mu.Unlock()
	return nil
}

// MembershipScopes returns caller-owned effective scope records.
func (cat *Catalog) MembershipScopes() []ProviderMembershipScope {
	return cat.source.MembershipScopes()
}

// ScopeMembership reads a precomputed scope without allocation or storage I/O.
// Missing scopes and incomplete absence return known=false. The provider must match.
func (cat *Catalog) ScopeMembership(key MembershipScopeKey, provider ProviderID, model string) (present, known bool) {
	scope, exists := cat.membership[key]
	if !exists || scope.provider != provider {
		return false, false
	}
	if scope.models[model] {
		return true, true
	}
	return false, scope.complete
}

func membershipKey(scope ProviderMembershipScope) MembershipScopeKey {
	return MembershipScopeKey{scope.PublisherID, scope.BindingID, scope.BindingRevision}
}

func copyMembershipScopes(input []ProviderMembershipScope) []ProviderMembershipScope {
	copied := slices.Clone(input)
	for index := range copied {
		scope := &copied[index]
		if scope.Inventory != nil {
			inventory := *scope.Inventory
			inventory.ModelIDs = slices.Clone(inventory.ModelIDs)
			scope.Inventory = &inventory
		}
		scope.Additions = slices.Clone(scope.Additions)
	}
	return copied
}

func prepareMembershipScopes(input []ProviderMembershipScope) ([]ProviderMembershipScope, error) {
	copied := copyMembershipScopes(input)
	seen := make(map[MembershipScopeKey]bool, len(copied))
	for index := range copied {
		scope := &copied[index]
		if err := scope.Validate(); err != nil {
			return nil, err
		}
		key := membershipKey(*scope)
		if seen[key] {
			return nil, &errors.ConflictError{Resource: "membership scope", Message: "publisher and binding revision must be unique"}
		}
		seen[key] = true
		if scope.Inventory != nil {
			scope.Inventory.ObservedAt = scope.Inventory.ObservedAt.UTC()
			slices.Sort(scope.Inventory.ModelIDs)
		}
		for i := range scope.Additions {
			scope.Additions[i].ObservedAt = scope.Additions[i].ObservedAt.UTC()
		}
		slices.SortFunc(scope.Additions, func(a, b MembershipPresence) int { return strings.Compare(a.ModelID, b.ModelID) })
	}
	slices.SortFunc(copied, func(a, b ProviderMembershipScope) int {
		if order := strings.Compare(a.PublisherID, b.PublisherID); order != 0 {
			return order
		}
		if order := strings.Compare(a.BindingID, b.BindingID); order != 0 {
			return order
		}
		return strings.Compare(a.BindingRevision, b.BindingRevision)
	})
	return copied, nil
}

func indexMembershipScopes(scopes []ProviderMembershipScope) map[MembershipScopeKey]membershipScopeIndex {
	index := make(map[MembershipScopeKey]membershipScopeIndex, len(scopes))
	for _, scope := range scopes {
		entry := membershipScopeIndex{provider: scope.ProviderID, complete: scope.Inventory != nil, models: make(map[string]bool)}
		if scope.Inventory != nil {
			for _, model := range scope.Inventory.ModelIDs {
				entry.models[model] = true
			}
		}
		for _, addition := range scope.Additions {
			entry.models[addition.ModelID] = true
		}
		index[membershipKey(scope)] = entry
	}
	return index
}

func mergeMembershipScopes(left, right []ProviderMembershipScope) ([]ProviderMembershipScope, error) {
	combined, err := prepareMembershipScopes(left)
	if err != nil {
		return nil, err
	}
	incoming, err := prepareMembershipScopes(right)
	if err != nil {
		return nil, err
	}
	existing := make(map[MembershipScopeKey]ProviderMembershipScope, len(combined))
	for _, scope := range combined {
		existing[membershipKey(scope)] = scope
	}
	for _, scope := range incoming {
		if prior, found := existing[membershipKey(scope)]; found {
			if !reflect.DeepEqual(prior, scope) {
				return nil, &errors.ConflictError{Resource: "membership scope", Message: "generic catalog merge cannot resolve different scope inventories"}
			}
			continue
		}
		combined = append(combined, scope)
	}
	return combined, nil
}
