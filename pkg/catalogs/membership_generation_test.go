package catalogs

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

func TestGenerationRejectsUnboundMembershipEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ProviderMembershipScope, *GenerationManifest)
		reject bool
	}{
		{"complete inventory", func(*ProviderMembershipScope, *GenerationManifest) {}, false},
		{"non-provider inventory receipt", func(_ *ProviderMembershipScope, m *GenerationManifest) {
			m.SourceObservations[0].Source = evidence.ModelsDevHTTPID
		}, true},
		{"non-provider positive receipt", func(_ *ProviderMembershipScope, m *GenerationManifest) {
			m.SourceObservations[1].Source = evidence.ModelsDevHTTPID
		}, true},
		{"missing inventory receipt", func(s *ProviderMembershipScope, _ *GenerationManifest) { s.Inventory.ObservationID = "missing" }, true},
		{"inventory time differs", func(s *ProviderMembershipScope, _ *GenerationManifest) {
			s.Inventory.ObservedAt = s.Inventory.ObservedAt.Add(-time.Second)
		}, true},
		{"degraded inventory", func(_ *ProviderMembershipScope, m *GenerationManifest) {
			m.SourceObservations[0].Status = evidence.ObservationStatusDegraded
			m.Degraded = true
		}, true},
		{"partial inventory", func(_ *ProviderMembershipScope, m *GenerationManifest) {
			m.SourceObservations[0].Completeness = evidence.ObservationCompletenessPartial
			m.SourceObservations[0].Status = evidence.ObservationStatusDegraded
			m.Degraded = true
		}, true},
		{"ambiguous inventory receipt", func(_ *ProviderMembershipScope, m *GenerationManifest) {
			m.SourceObservations[1].ObservationID = m.SourceObservations[0].ObservationID
			m.SourceObservations[1].Source = evidence.ModelsDevHTTPID
		}, true},
		{"missing positive receipt", func(s *ProviderMembershipScope, _ *GenerationManifest) { s.Additions[0].ObservationID = "missing" }, true},
		{"positive time differs", func(s *ProviderMembershipScope, _ *GenerationManifest) {
			s.Additions[0].ObservedAt = s.Additions[0].ObservedAt.Add(time.Second)
		}, true},
		{"partial positive accepted", func(_ *ProviderMembershipScope, m *GenerationManifest) {
			m.SourceObservations[1].Completeness = evidence.ObservationCompletenessPartial
			m.SourceObservations[1].Status = evidence.ObservationStatusDegraded
			m.Degraded = true
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := loadGenerationManifestFixture(t)
			manifest.SourceObservations[1].Source = evidence.ProvidersID
			manifest.SchemaVersion = CurrentCatalogSchemaVersion
			manifest.ConsumerCompatibility = ConsumerCompatibility{MinSchemaVersion: CurrentCatalogSchemaVersion, MaxSchemaVersion: CurrentCatalogSchemaVersion}
			scope := testMembershipScope()
			inventory, positive := manifest.SourceObservations[0], manifest.SourceObservations[1]
			scope.Inventory = &MembershipInventory{ObservationID: inventory.ObservationID, ObservedAt: inventory.ObservedAt, ModelIDs: []string{}}
			scope.Additions = []MembershipPresence{{ModelID: "model", ObservationID: positive.ObservationID, ObservedAt: positive.ObservedAt}}
			tc.mutate(&scope, &manifest)
			if manifest.Degraded {
				manifest.DegradationReasons = []string{"partial source evidence"}
			}
			for _, link := range manifest.SourceObservations {
				if link.Completeness == evidence.ObservationCompletenessPartial {
					manifest.Completeness = GenerationCompletenessPartial
				}
			}
			builder := NewEmpty()
			if err := builder.SetMembershipScopes([]ProviderMembershipScope{scope}); err != nil {
				t.Fatal(err)
			}
			payload, err := EncodeCatalogPayload(builder)
			if err != nil {
				t.Fatal(err)
			}
			manifest.Payload = DescribeCatalogPayload(payload)
			generation := Generation{Manifest: manifest, Payload: payload}
			if err := generation.Validate(); err != nil {
				t.Fatalf("invalid fixture: %v", err)
			}
			catalog, err := DecodeCatalogGeneration(generation)
			if tc.reject {
				if err == nil || catalog != nil {
					t.Fatal("activation accepted unbound membership evidence")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
