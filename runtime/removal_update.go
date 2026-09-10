package runtime

import (
	"context"
	"slices"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type removalUpdate struct {
	expected starmap.CatalogState
	set      *catalogs.CatalogRemovalSet
}

// ReplaceRemovalTargets replaces this runtime's operator removal snapshot.
// Expected generation identity and checksum prevent stale edits. An empty target list restores local removals.
// The caller authorizes the operator action. Other publishers retain their own policies.
func (r *Runtime) ReplaceRemovalTargets(ctx context.Context, expected starmap.CatalogState, targets ...catalogs.CatalogRemovalTarget) (starmap.CatalogState, error) {
	if expected.GenerationID == "" || expected.PayloadChecksum == "" {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "removal.expected", Message: "generation identity and payload checksum are required"}
	}
	set, err := catalogs.NewCatalogRemovalSet(targets...)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	var state starmap.CatalogState
	_, err = r.execute(ctx, runKindManual, func(runCtx context.Context, _ *RefreshReport, epoch uint64) error {
		var err error
		state, err = r.publishInputsWithRemovals(runCtx, nil, nil, nil, epoch, nil, &removalUpdate{expected: expected, set: set})
		return err
	})
	return state, err
}

func (l *layerSet) prepareRemovalUpdate(current starmap.CatalogState, update *removalUpdate) error {
	if current.GenerationID != update.expected.GenerationID || current.PayloadChecksum != update.expected.PayloadChecksum {
		return &errors.ConflictError{Resource: "operator removal", Expected: update.expected.GenerationID, Actual: current.GenerationID, Message: "catalog changed before the operator edit"}
	}
	var retained *catalogs.CatalogRemovalSet
	if l.removals != nil {
		var err error
		retained, err = catalogs.NewCatalogRemovalSet(l.removals.Targets...)
		if err != nil {
			return err
		}
	}
	scopes := current.Catalog.MembershipScopes()
	for _, target := range update.set.Targets() {
		if target.Kind == catalogs.CatalogRemovalCanonical {
			if retained.ContainsCanonical(target.DefinitionID) {
				continue
			}
			if _, err := current.Catalog.Definition(target.DefinitionID); err != nil {
				return err
			}
			continue
		}
		if l.removals != nil && slices.ContainsFunc(l.removals.Targets, func(prior catalogs.CatalogRemovalTarget) bool {
			return prior.Kind == target.Kind && prior.ProviderModelID == target.ProviderModelID && prior.Scope != nil && *prior.Scope == *target.Scope
		}) {
			continue
		}
		if !slices.ContainsFunc(scopes, func(scope catalogs.ProviderMembershipScope) bool {
			return target.MatchesScope(scope, target.ProviderModelID)
		}) {
			return &errors.ConflictError{Resource: "operator removal scope", Message: "target has no matching accepted account scope"}
		}
		if _, err := current.Catalog.Offering(target.Scope.ProviderID, target.ProviderModelID); err != nil {
			return err
		}
	}
	l.removals = &catalogs.CatalogRemovalPolicy{PublisherID: l.publisherID, Targets: update.set.Targets()}
	return nil
}

func (l *layerSet) applyRemovalPolicy(catalog *catalogs.Catalog) (*catalogs.Catalog, error) {
	if l.removals == nil {
		return catalog, nil
	}
	if l.removals.PublisherID != l.publisherID {
		return nil, &errors.ConflictError{Resource: "operator removal publisher", Message: "retained policy belongs to another runtime identity"}
	}
	policies := catalog.RemovalPolicies()
	policies = slices.DeleteFunc(policies, func(policy catalogs.CatalogRemovalPolicy) bool { return policy.PublisherID == l.publisherID })
	policies = append(policies, *l.removals)
	builder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		return nil, err
	}
	if err := builder.SetRemovalPolicies(policies); err != nil {
		return nil, err
	}
	return builder.Build()
}
