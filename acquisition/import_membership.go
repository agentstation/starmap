package acquisition

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// importMembershipEvidence retains only original receipts that effective scopes reference.
func importMembershipEvidence(
	scopes []catalogs.ProviderMembershipScope,
	links []catalogs.SourceObservationLink,
	manifests ...[]catalogs.SourceObservationLink,
) ([]catalogs.SourceObservationLink, error) {
	referenced := make(map[string]bool)
	for _, scope := range scopes {
		if scope.Inventory != nil {
			referenced[scope.Inventory.ObservationID] = true
		}
		for _, addition := range scope.Additions {
			referenced[addition.ObservationID] = true
		}
	}
	existing := make(map[string]catalogs.SourceObservationLink, len(links))
	for _, link := range links {
		existing[link.ObservationID] = link
	}
	for _, manifest := range manifests {
		for _, link := range manifest {
			if !referenced[link.ObservationID] {
				continue
			}
			if previous, found := existing[link.ObservationID]; found {
				if previous != link {
					return nil, &errors.ConflictError{Resource: "scope observation", Message: "one observation identity has conflicting source evidence"}
				}
				continue
			}
			links = append(links, link)
			existing[link.ObservationID] = link
		}
	}
	if err := catalogs.ValidateMembershipEvidence(scopes, links); err != nil {
		return nil, err
	}
	return links, nil
}
