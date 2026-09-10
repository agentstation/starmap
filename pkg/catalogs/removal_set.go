package catalogs

import (
	"cmp"
	"slices"
)

// CatalogRemovalSet retains immutable operator targets and their lookup indexes.
// Construction validates target shape. The caller authorizes targets before construction.
type CatalogRemovalSet struct {
	targets   []CatalogRemovalTarget
	scoped    map[scopedRemovalKey]struct{}
	canonical map[ModelDefinitionID]struct{}
	aliases   map[ModelDefinitionID]struct{}
}

type scopedRemovalKey struct {
	scope CatalogRemovalScope
	model ProviderModelID
}

// NewCatalogRemovalSet copies valid targets and removes identical duplicates.
// The resulting set has deterministic order and is safe for concurrent reads.
func NewCatalogRemovalSet(targets ...CatalogRemovalTarget) (*CatalogRemovalSet, error) {
	set := &CatalogRemovalSet{
		scoped:    make(map[scopedRemovalKey]struct{}),
		canonical: make(map[ModelDefinitionID]struct{}),
		aliases:   make(map[ModelDefinitionID]struct{}),
	}
	for _, target := range targets {
		if err := target.Validate(); err != nil {
			return nil, err
		}
		if target.Kind == CatalogRemovalCanonical {
			if _, found := set.canonical[target.DefinitionID]; found {
				continue
			}
			set.canonical[target.DefinitionID] = struct{}{}
		} else if target.Kind == CatalogRemovalAlias {
			if _, found := set.aliases[target.AliasID]; found {
				continue
			}
			set.aliases[target.AliasID] = struct{}{}
		} else {
			key := scopedRemovalKey{scope: *target.Scope, model: target.ProviderModelID}
			if _, found := set.scoped[key]; found {
				continue
			}
			set.scoped[key] = struct{}{}
			scope := *target.Scope
			target.Scope = &scope
		}
		set.targets = append(set.targets, target)
	}
	slices.SortFunc(set.targets, compareRemovalTargets)
	return set, nil
}

// Targets returns caller-owned targets in deterministic order.
func (s *CatalogRemovalSet) Targets() []CatalogRemovalTarget {
	if s == nil {
		return nil
	}
	targets := slices.Clone(s.targets)
	for i := range targets {
		if targets[i].Scope != nil {
			scope := *targets[i].Scope
			targets[i].Scope = &scope
		}
	}
	return targets
}

// ContainsCanonical reports an explicit removal of the canonical model.
// A scoped removal never changes this result.
func (s *CatalogRemovalSet) ContainsCanonical(id ModelDefinitionID) bool {
	if s == nil {
		return false
	}
	_, found := s.canonical[id]
	return found
}

// indexCanonicalRenames preserves model removals across retained rename edges.
// Catalog construction calls this before publication. Original operator targets remain unchanged.
func (s *CatalogRemovalSet) indexCanonicalRenames(aliases *CanonicalAliasIndex) {
	for _, target := range s.targets {
		if target.Kind != CatalogRemovalCanonical {
			continue
		}
		if terminal, _, found := aliases.Lookup(target.DefinitionID); found {
			s.canonical[terminal] = struct{}{}
		}
	}
}

// ContainsAlias reports an explicit removal of one former canonical ID.
// It does not remove the target model or other aliases.
func (s *CatalogRemovalSet) ContainsAlias(id ModelDefinitionID) bool {
	if s == nil {
		return false
	}
	_, found := s.aliases[id]
	return found
}

// ContainsScoped reports an exact account or public entry removal without allocating memory.
// The caller authenticates and validates the source scope before this query.
func (s *CatalogRemovalSet) ContainsScoped(scope ProviderMembershipScope, model ProviderModelID) bool {
	if s == nil {
		return false
	}
	key := scopedRemovalKey{model: model, scope: CatalogRemovalScope{
		PublisherID: scope.PublisherID, ProviderID: scope.ProviderID, AccountID: scope.AccountID,
		ProjectID: scope.ProjectID, Region: scope.Region, APISurface: scope.APISurface, Public: scope.Public,
	}}
	_, found := s.scoped[key]
	return found
}

func compareRemovalTargets(left, right CatalogRemovalTarget) int {
	if result := cmp.Compare(left.Kind, right.Kind); result != 0 {
		return result
	}
	if left.Kind == CatalogRemovalCanonical {
		return cmp.Compare(left.DefinitionID, right.DefinitionID)
	}
	if left.Kind == CatalogRemovalAlias {
		return cmp.Compare(left.AliasID, right.AliasID)
	}
	l, r := left.Scope, right.Scope
	result := cmp.Or(
		cmp.Compare(l.PublisherID, r.PublisherID), cmp.Compare(l.ProviderID, r.ProviderID),
		cmp.Compare(l.AccountID, r.AccountID), cmp.Compare(l.ProjectID, r.ProjectID),
		cmp.Compare(l.Region, r.Region), cmp.Compare(l.APISurface, r.APISurface),
	)
	if result != 0 {
		return result
	}
	if l.Public != r.Public {
		if l.Public {
			return 1
		}
		return -1
	}
	return cmp.Compare(left.ProviderModelID, right.ProviderModelID)
}
