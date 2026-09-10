package catalogs

import (
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

// ValidateMembershipEvidence checks scope facts against original provider observations.
// It does not authenticate publishers or prove a source's declared scope authority.
func ValidateMembershipEvidence(scopes []ProviderMembershipScope, links []SourceObservationLink) error {
	if len(scopes) == 0 {
		return nil
	}
	if err := validateObservationLinks(links); err != nil {
		return err
	}
	observations := make(map[string]SourceObservationLink, len(links))
	ambiguous := make(map[string]bool)
	for _, link := range links {
		if _, exists := observations[link.ObservationID]; exists {
			ambiguous[link.ObservationID] = true
		}
		observations[link.ObservationID] = link
	}
	for _, scope := range scopes {
		if err := scope.Validate(); err != nil {
			return err
		}
		if scope.Inventory != nil {
			if err := validateMembershipReceipt(scope.Inventory.ObservationID, scope.Inventory.ObservedAt, true, observations, ambiguous); err != nil {
				return err
			}
		}
		for _, addition := range scope.Additions {
			if err := validateMembershipReceipt(addition.ObservationID, addition.ObservedAt, false, observations, ambiguous); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMembershipReceipt(id string, at time.Time, complete bool, observations map[string]SourceObservationLink, ambiguous map[string]bool) error {
	link, found := observations[id]
	if !found || ambiguous[id] {
		return invalidMembershipScope("observation_id", "must resolve to exactly one generation source observation")
	}
	if link.Source != evidence.ProvidersID {
		return invalidMembershipScope("observation_id", "requires an original provider source observation")
	}
	if !link.ObservedAt.Equal(at) {
		return invalidMembershipScope("observed_at", "must match the original source observation time")
	}
	if complete && (link.Completeness != evidence.ObservationCompletenessComplete || link.Status != evidence.ObservationStatusSucceeded) {
		return invalidMembershipScope("inventory", "requires a complete successful source observation")
	}
	return nil
}
