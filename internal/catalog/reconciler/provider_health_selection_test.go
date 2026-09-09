package reconciler

import (
	"bytes"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderFailureKeepsConfigurationOutOfCanonicalFacts(t *testing.T) {
	for _, code := range []sources.ObservationIssueCode{
		sources.ObservationIssueCodeMissingCredentials,
		sources.ObservationIssueCodeConfiguration,
		sources.ObservationIssueCodeFetchFailed,
		sources.ObservationIssueCodeSchemaDrift,
	} {
		for _, existing := range []bool{false, true} {
			name := string(code) + "/new"
			if existing {
				name = string(code) + "/existing"
			}
			t.Run(name, func(t *testing.T) {
				baseline, original := providerSelectionFixture(t)
				if existing {
					builder, err := catalogs.NewBuilderFrom(baseline)
					if err != nil {
						t.Fatal(err)
					}
					if err := builder.SetProvider(catalogs.Provider{ID: "provider-b", Name: "Accepted B"}); err != nil {
						t.Fatal(err)
					}
					baseline, err = builder.Build()
					if err != nil {
						t.Fatal(err)
					}
				}
				builder, err := catalogs.NewBuilderFrom(original.Catalog)
				if err != nil {
					t.Fatal(err)
				}
				provider, err := builder.Provider("provider-b")
				if err != nil {
					t.Fatal(err)
				}
				provider.Name, provider.Models = "Unconfirmed B", nil
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				candidate, err := catalogs.NewObservationCatalog(builder)
				if err != nil {
					t.Fatal(err)
				}
				observation, err := sources.NewObservation(sources.ProvidersID, candidate, sources.ObservationMetadata{
					ObservedAt: original.ObservedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
					Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
					Records: sources.ObservationRecordCounts{Accepted: 1},
					Issues:  []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeProvider, Subject: "provider-b", Code: code, Message: "Provider did not return usable live evidence."}},
				})
				if err != nil {
					t.Fatal(err)
				}
				before, err := catalogs.EncodeCatalogPayload(observation.Catalog)
				if err != nil {
					t.Fatal(err)
				}
				receipt, err := observation.Receipt()
				if err != nil {
					t.Fatal(err)
				}
				result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{observation})
				if err != nil {
					t.Fatal(err)
				}
				published, err := result.Catalog.Build()
				if err != nil {
					t.Fatal(err)
				}
				failed, found := published.Providers().Get("provider-b")
				if found != existing || (existing && failed.Name != "Accepted B") {
					t.Fatal("unavailable provider configuration changed canonical facts")
				}
				healthy, err := published.Provider("provider-a")
				if err != nil || healthy.Models["shared"].Limits.ContextWindow != 200 {
					t.Fatal("unavailable provider discarded a healthy sibling")
				}
				after, err := catalogs.EncodeCatalogPayload(observation.Catalog)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("selection changed the original observation payload")
				}
				if _, err := receipt.Restore(observation.Catalog); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestProviderFailureSelectionKeepsUsableEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		issues []sources.ObservationIssue
	}{
		{name: "healthy-empty"},
		{name: "partial-records", issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeProvider, Subject: "provider-b", Code: sources.ObservationIssueCodePayloadLimit, Message: "Some records exceeded the bound."}}},
		{name: "record-scope", issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Subject: "provider-b", Code: sources.ObservationIssueCodeSchemaDrift, Message: "One record was rejected."}}},
		{name: "other-provider", issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeProvider, Subject: "unrelated", Code: sources.ObservationIssueCodeFetchFailed, Message: "Another provider failed."}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseline, original := providerSelectionFixture(t)
			builder, err := catalogs.NewBuilderFrom(original.Catalog)
			if err != nil {
				t.Fatal(err)
			}
			provider, err := builder.Provider("provider-b")
			if err != nil {
				t.Fatal(err)
			}
			provider.Models = nil
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			candidate, err := catalogs.NewObservationCatalog(builder)
			if err != nil {
				t.Fatal(err)
			}
			metadata := sources.ObservationMetadata{ObservedAt: original.ObservedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded, Records: sources.ObservationRecordCounts{Accepted: 1}, Issues: test.issues}
			if len(test.issues) != 0 {
				metadata.Completeness, metadata.Status = sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded
			}
			observation, err := sources.NewObservation(sources.ProvidersID, candidate, metadata)
			if err != nil {
				t.Fatal(err)
			}
			result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{observation})
			if err != nil {
				t.Fatal(err)
			}
			published, err := result.Catalog.Build()
			if err != nil {
				t.Fatal(err)
			}
			if provider, err := published.Provider("provider-b"); err != nil || len(provider.Models) != 0 {
				t.Fatal("selection discarded a usable empty provider inventory")
			}
		})
	}
}
