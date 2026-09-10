package reconciler

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestEmptyProviderInventoryRequiresExplicitAuthority(t *testing.T) {
	seed := sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared"})
	builder, err := catalogs.NewBuilderFrom(seed)
	if err != nil {
		t.Fatal(err)
	}
	peer, err := seed.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	peer.ID, peer.Name = "provider-b", "Provider B"
	if err := builder.SetProvider(peer); err != nil {
		t.Fatal(err)
	}
	baseline, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	provider, err := seed.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models = nil
	empty := catalogs.NewEmpty()
	if err := empty.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	inventory, err := catalogs.NewObservationCatalog(empty)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name         string
		public       bool
		unscoped     bool
		completeness sources.ObservationCompleteness
		status       sources.ObservationStatus
		wantOffering bool
		authority    sources.ProviderMembershipAuthority
	}{
		{"complete-public", true, false, sources.ObservationCompletenessComplete, sources.ObservationStatusSucceeded, false, sources.ProviderMembershipProvider},
		{"partial-public", true, false, sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded, true, sources.ProviderMembershipProvider},
		{"failed-public", true, false, sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded, true, sources.ProviderMembershipProvider},
		{"complete-account", false, false, sources.ObservationCompletenessComplete, sources.ObservationStatusSucceeded, true, sources.ProviderMembershipScope},
		{"complete-unscoped", false, true, sources.ObservationCompletenessComplete, sources.ObservationStatusSucceeded, true, sources.ProviderMembershipEvidenceOnly},
		{"complete-public-evidence", true, false, sources.ObservationCompletenessComplete, sources.ObservationStatusSucceeded, true, sources.ProviderMembershipEvidenceOnly},
		{"complete-public-scope", true, false, sources.ObservationCompletenessComplete, sources.ObservationStatusSucceeded, true, sources.ProviderMembershipScope},
	} {
		t.Run(test.name, func(t *testing.T) {
			binding := &sources.ProviderAcquisitionBinding{
				SchemaVersion:       sources.ProviderAcquisitionBindingSchemaVersion,
				MembershipAuthority: test.authority,
				ID:                  "inventory", Revision: "1", ProviderID: "provider-a",
				Public: test.public, Region: "global", APISurface: "models.list",
				CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "unauthenticated",
			}
			if !test.public {
				binding.AccountID = "account-a"
			}
			if test.unscoped {
				binding = nil
			}
			var issues []sources.ObservationIssue
			if test.name == "partial-public" {
				issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeSchemaDrift, Message: "Source inventory is incomplete."}}
			}
			if test.name == "failed-public" {
				issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeFetchFailed, Message: "Source fetch failed."}}
			}
			observation, err := sources.NewObservation(sources.ProvidersID, inventory, sources.ObservationMetadata{
				ProviderBinding: binding, ObservedAt: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
				Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
				Completeness: test.completeness, Status: test.status, Issues: issues,
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{
				{SourceID: sources.EmbeddedCatalogID, Catalog: baseline}, observation,
			})
			if err != nil {
				t.Fatal(err)
			}
			got, ok := result.Catalog.Providers().Get("provider-a")
			if !ok {
				t.Fatal("inventory removal deleted provider metadata")
			}
			if _, exists := got.Models["shared"]; exists != test.wantOffering {
				t.Errorf("provider-a offering present = %v, want %v", exists, test.wantOffering)
			}
			other, ok := result.Catalog.Providers().Get("provider-b")
			if !ok || other.Models["shared"] == nil {
				t.Fatal("provider-a inventory removed unrelated provider-b offering")
			}
			if _, err := result.Catalog.Build(); err != nil {
				t.Fatalf("scope handling damaged canonical catalog: %v", err)
			}
		})
	}
}
