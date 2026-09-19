package starmap

import (
	"context"
	"reflect"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// retainMembershipEvidence keeps original receipts for an unchanged inventory.
// A custom update cannot use retained receipts to change provider membership.
func (c *Client) retainMembershipEvidence(ctx context.Context, published *catalogs.Catalog, evidence CandidateEvidence) (CandidateEvidence, error) {
	current := c.Catalog().MembershipScopes()
	proposed := published.MembershipScopes()
	if len(current) == 0 && len(proposed) == 0 {
		return evidence, nil
	}
	customUpdate := len(evidence.SourceObservations) == 0
	if customUpdate && !reflect.DeepEqual(current, proposed) {
		return CandidateEvidence{}, &errors.ValidationError{
			Field: "candidate.membership_scopes", Message: "membership changes require explicit source observations",
		}
	}
	if !customUpdate && catalogs.ValidateMembershipEvidence(proposed, evidence.SourceObservations) == nil {
		return evidence, nil
	}
	generation, err := c.CurrentGeneration(ctx)
	if err != nil {
		return CandidateEvidence{}, err
	}
	if !customUpdate {
		return retainUnchangedScopeLinks(current, proposed, generation.Manifest, evidence), nil
	}
	if err := catalogs.ValidateMembershipEvidence(proposed, generation.Manifest.SourceObservations); err != nil {
		return CandidateEvidence{}, err
	}
	reviewObservations := make(map[string]bool, len(generation.Manifest.ReviewCandidates))
	for _, review := range generation.Manifest.ReviewCandidates {
		reviewObservations[review.SourceObservationID] = true
	}
	observations := make([]catalogs.SourceObservationLink, 0, len(generation.Manifest.SourceObservations))
	for _, observation := range generation.Manifest.SourceObservations {
		if observation.Source != customUpdateSourceID || reviewObservations[observation.ObservationID] {
			observations = append(observations, observation)
		}
	}
	return CandidateEvidence{
		SourceObservations: observations,
		ReviewCandidates:   generation.Manifest.ReviewCandidates,
	}, nil
}

func retainUnchangedScopeLinks(current, proposed []catalogs.ProviderMembershipScope, retained catalogs.GenerationManifest, evidence CandidateEvidence) CandidateEvidence {
	present := make(map[string]bool, len(evidence.SourceObservations))
	for _, link := range evidence.SourceObservations {
		present[link.ObservationID] = true
	}
	needed := make(map[string]bool)
	for _, scope := range proposed {
		for _, previous := range current {
			if !reflect.DeepEqual(scope, previous) {
				continue
			}
			if scope.Inventory != nil {
				needed[scope.Inventory.ObservationID] = true
			}
			for _, addition := range scope.Additions {
				needed[addition.ObservationID] = true
			}
			break
		}
	}
	links := append([]catalogs.SourceObservationLink(nil), evidence.SourceObservations...)
	for _, link := range retained.SourceObservations {
		if needed[link.ObservationID] && !present[link.ObservationID] {
			links = append(links, link)
			present[link.ObservationID] = true
		}
	}
	evidence.SourceObservations = links
	return evidence
}
