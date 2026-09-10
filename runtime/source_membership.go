package runtime

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// decodeCatalog checks retained scope evidence against its original generation.
// Legacy payload-only layers remain readable when they contain no scope records.
func (s *sourceLayer) decodeCatalog() (*catalogs.Catalog, error) {
	if s.Manifest != nil {
		if s.Manifest.GenerationID != s.GenerationID || s.Manifest.Payload.Checksum != s.Checksum {
			return nil, &errors.ValidationError{Field: "source_layer.manifest", Message: "must match the retained generation identity and checksum"}
		}
		return catalogs.DecodeCatalogGeneration(catalogs.Generation{Manifest: *s.Manifest, Payload: s.Payload})
	}
	catalog, err := catalogs.DecodeCatalogPayload(s.Payload)
	if err != nil {
		return nil, err
	}
	if len(catalog.MembershipScopes()) != 0 {
		return nil, &errors.ValidationError{Field: "source_layer.manifest", Message: "scope records require the original generation manifest"}
	}
	return catalog, nil
}

func (l *layerSet) appendScopeSourceEvidence(catalog *catalogs.Catalog) error {
	if l.source == nil || l.source.Manifest == nil {
		return nil
	}
	referenced := make(map[string]bool)
	for _, scope := range catalog.MembershipScopes() {
		if scope.Inventory != nil {
			referenced[scope.Inventory.ObservationID] = true
		}
		for _, addition := range scope.Additions {
			referenced[addition.ObservationID] = true
		}
	}
	if len(referenced) == 0 {
		return nil
	}
	existing := make(map[string]catalogs.SourceObservationLink)
	for _, link := range l.buildEvidence.SourceObservations {
		existing[link.ObservationID] = link
	}
	for _, link := range l.source.Manifest.SourceObservations {
		if !referenced[link.ObservationID] {
			continue
		}
		if previous, found := existing[link.ObservationID]; found {
			if previous != link {
				return &errors.ConflictError{Resource: "scope observation", Message: "one observation identity has conflicting source evidence"}
			}
			continue
		}
		l.buildEvidence.SourceObservations = append(l.buildEvidence.SourceObservations, link)
		existing[link.ObservationID] = link
	}
	compactManualEvidence(&l.buildEvidence)
	return nil
}

// validateUpstreamScopePublishers reserves this runtime's identities for local evidence.
func (l *layerSet) validateUpstreamScopePublishers(catalog *catalogs.Catalog) error {
	for _, policy := range catalog.RemovalPolicies() {
		if namesInstance(policy.PublisherID, l.publisherID, l.publisherAliases) {
			return &errors.ConflictError{Resource: "removal publisher", Message: "an upstream catalog cannot claim this runtime operator identity"}
		}
	}
	for _, scope := range catalog.MembershipScopes() {
		if namesInstance(scope.PublisherID, l.publisherID, l.publisherAliases) {
			return &errors.ConflictError{
				Resource: "scope publisher", Expected: "an upstream publisher distinct from this runtime",
				Actual: scope.PublisherID, Message: "an upstream catalog cannot claim this runtime's scope identity",
			}
		}
	}
	return nil
}
