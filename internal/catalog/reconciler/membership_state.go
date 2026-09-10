package reconciler

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// MembershipState resolves membership from a deployment's selected observation history.
// Its maps are private and remain immutable after construction.
type MembershipState struct {
	providers map[catalogs.ProviderID]*providerMembership
}

type membershipFact struct {
	at       time.Time
	fallback bool
}

type membershipInventory struct {
	fact   membershipFact
	models map[string]bool
}

type membershipScopeKey struct{ id, revision string }

type membershipScope struct {
	binding  sources.ProviderAcquisitionBinding
	positive map[string]membershipFact
	latest   membershipFact
	latestID string
}

type providerMembership struct {
	inventory *membershipInventory
	scopes    map[membershipScopeKey]*membershipScope
}

// ResolveMembership reads verified, selected history without changing its observations.
// Callers must exclude revoked bindings and reset observations before construction.
func ResolveMembership(ctx context.Context, observations []sources.Observation) (*MembershipState, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "membership.context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ordered := slices.Clone(observations)
	slices.SortStableFunc(ordered, CompareProviderObservations)
	state := &MembershipState{providers: make(map[catalogs.ProviderID]*providerMembership)}
	seen := make(map[string]bool)
	for _, observation := range ordered {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if observation.ProviderBinding == nil {
			continue
		}
		if err := observation.Validate(); err != nil {
			return nil, err
		}
		if seen[observation.ID] {
			continue
		}
		seen[observation.ID] = true
		binding := *observation.ProviderBinding
		if !providerObservationSelection(nil).permits(observation, binding.ProviderID) {
			continue
		}
		provider, _ := observation.Catalog.Providers().Get(binding.ProviderID)
		membership := state.providers[binding.ProviderID]
		if membership == nil {
			membership = &providerMembership{scopes: make(map[membershipScopeKey]*membershipScope)}
			state.providers[binding.ProviderID] = membership
		}
		key := membershipScopeKey{binding.ID, binding.Revision}
		scope := membership.scopes[key]
		fact := membershipFact{at: observation.ObservedAt, fallback: scopedFallback(observation)}
		if scope == nil {
			scope = &membershipScope{binding: binding, positive: make(map[string]membershipFact)}
			membership.scopes[key] = scope
		} else {
			if scope.binding != binding {
				return nil, membershipConflict("one binding revision declares different membership scopes")
			}
			if compareMembershipFacts(scope.latest, fact) == 0 && scope.latestID != observation.ID {
				return nil, membershipConflict("different inventories have the same binding and observation time")
			}
		}
		scope.latest, scope.latestID = fact, observation.ID
		complete := observation.Status == sources.ObservationStatusSucceeded && observation.Completeness == sources.ObservationCompletenessComplete && !fact.fallback
		if complete && binding.MembershipAuthority != sources.ProviderMembershipEvidenceOnly {
			clear(scope.positive)
		}
		inventory := make(map[string]bool, len(provider.Models))
		for model := range provider.Models {
			scope.positive[model] = fact
			inventory[model] = true
		}
		if complete && binding.MembershipAuthority == sources.ProviderMembershipProvider {
			if previous := membership.inventory; previous != nil && compareMembershipFacts(previous.fact, fact) == 0 && !maps.Equal(previous.models, inventory) {
				return nil, membershipConflict("provider-wide inventories disagree at the same observation time")
			}
			membership.inventory = &membershipInventory{fact: fact, models: inventory}
		}
	}
	return state, nil
}

// Permits reports whether retained evidence keeps an offering in canonical discovery.
// Account and scoped-public evidence remain separate from provider-wide public absence.
func (s *MembershipState) Permits(provider catalogs.ProviderID, model string) bool {
	if s == nil {
		return true
	}
	membership := s.providers[provider]
	if membership == nil || membership.inventory == nil || membership.inventory.models[model] {
		return true
	}
	for _, scope := range membership.scopes {
		fact, exists := scope.positive[model]
		if !exists {
			continue
		}
		if !scope.binding.Public || scope.binding.MembershipAuthority == sources.ProviderMembershipScope || compareMembershipFacts(fact, membership.inventory.fact) > 0 {
			return true
		}
	}
	return false
}

func compareMembershipFacts(left, right membershipFact) int {
	if left.fallback != right.fallback {
		if left.fallback {
			return -1
		}
		return 1
	}
	return left.at.Compare(right.at)
}

func membershipConflict(message string) error {
	return &errors.ConflictError{Resource: "provider membership", Message: message}
}

// WithMembershipState shares one resolved history across intermediate field-replay batches.
func WithMembershipState(state *MembershipState) Option {
	return func(options *options) error {
		if state == nil {
			return &errors.ValidationError{Field: "membership.state", Message: "is required"}
		}
		options.membership = state
		return nil
	}
}

func (s *MembershipState) apply(ctx context.Context, catalog *catalogs.Builder) error {
	for providerID, membership := range s.providers {
		if membership.inventory == nil {
			continue
		}
		provider, exists := catalog.Providers().Get(providerID)
		if !exists {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		changed := false
		for model := range provider.Models {
			if !s.Permits(provider.ID, model) {
				delete(provider.Models, model)
				changed = true
			}
		}
		if changed {
			if err := catalog.SetProvider(*provider); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *MembershipState) pruneProvenance(entries provenance.Map) {
	for key := range entries {
		resource, found := strings.CutPrefix(key, "model:")
		if !found {
			continue
		}
		identity, _, found := strings.Cut(resource, ":")
		if !found {
			continue
		}
		provider, model, valid := provenance.ParseModelResourceID(identity)
		if valid && !s.Permits(catalogs.ProviderID(provider), model) {
			delete(entries, key)
		}
	}
}
