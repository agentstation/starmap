package reconciler

import (
	"context"
	stderrors "errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func membershipObservation(t *testing.T, id string, authority sources.ProviderMembershipAuthority, public, present, complete, fallback bool, minute int) sources.Observation {
	t.Helper()
	catalog := sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared"})
	if !present {
		builder, err := catalogs.NewBuilderFrom(catalog)
		if err != nil {
			t.Fatal(err)
		}
		provider, err := builder.Provider("provider-a")
		if err != nil {
			t.Fatal(err)
		}
		provider.Models = nil
		if err := builder.SetProvider(provider); err != nil {
			t.Fatal(err)
		}
		catalog, err = catalogs.NewObservationCatalog(builder)
		if err != nil {
			t.Fatal(err)
		}
	}
	binding := sources.ProviderAcquisitionBinding{SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion, ID: id, Revision: "1", ProviderID: "provider-a", Public: public, Region: "global", APISurface: "models.list", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog", MembershipAuthority: authority}
	if !public {
		binding.AccountID = id
	}
	metadata := sources.ObservationMetadata{ProviderBinding: &binding, ObservedAt: time.Date(2026, 9, 9, 0, minute, 0, 0, time.UTC), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded}
	if !complete {
		metadata.Completeness = sources.ObservationCompletenessPartial
		metadata.Status = sources.ObservationStatusDegraded
		metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeSchemaDrift, Message: "Inventory is incomplete."}}
	}
	if fallback {
		metadata.Status = sources.ObservationStatusDegraded
		metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback, Message: "Inventory uses retained evidence."}}
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, metadata)
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestMembershipHistoryPreservesScopeAndPartialEvidence(t *testing.T) {
	global := func(present, complete, fallback bool, minute int) sources.Observation {
		return membershipObservation(t, "global", sources.ProviderMembershipProvider, true, present, complete, fallback, minute)
	}
	scoped := func(id string, public, present bool, minute int) sources.Observation {
		return membershipObservation(t, id, sources.ProviderMembershipScope, public, present, true, false, minute)
	}
	evidence := func(minute int) sources.Observation {
		return membershipObservation(t, "evidence", sources.ProviderMembershipEvidenceOnly, true, true, true, false, minute)
	}
	for _, test := range []struct {
		name         string
		observations []sources.Observation
		want         map[string]bool
	}{
		{"complete absence", []sources.Observation{global(true, true, false, 1), global(false, true, false, 2)}, map[string]bool{"global": false}},
		{"reverse input order", []sources.Observation{global(false, true, false, 2), global(true, true, false, 1)}, map[string]bool{"global": false}},
		{"partial absence", []sources.Observation{global(false, true, false, 1), global(false, false, false, 2)}, map[string]bool{"global": false}},
		{"new partial presence", []sources.Observation{global(false, true, false, 1), global(true, false, false, 2)}, map[string]bool{"global": true}},
		{"stale presence", []sources.Observation{global(false, true, false, 1), global(true, true, true, 2)}, map[string]bool{"global": false}},
		{"unrelated account", []sources.Observation{scoped("account", false, true, 1), global(false, true, false, 2)}, map[string]bool{"account": true, "global": false}},
		{"unrelated public scope", []sources.Observation{scoped("region", true, true, 1), global(false, true, false, 2)}, map[string]bool{"region": true, "global": false}},
		{"account later withdraws", []sources.Observation{scoped("account", false, true, 1), global(false, true, false, 2), scoped("account", false, false, 3)}, map[string]bool{"account": false, "global": false}},
		{"older public evidence", []sources.Observation{evidence(1), global(false, true, false, 2)}, map[string]bool{"evidence": true, "global": false}},
		{"new public evidence", []sources.Observation{global(false, true, false, 1), evidence(2)}, map[string]bool{"evidence": true, "global": false}},
		{"account cannot remove baseline", []sources.Observation{scoped("account", false, false, 1)}, map[string]bool{"account": false}},
		{"public scope cannot remove baseline", []sources.Observation{scoped("region", true, false, 1)}, map[string]bool{"region": false}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, err := ResolveMembership(t.Context(), test.observations)
			if err != nil {
				t.Fatal(err)
			}
			scopes, err := state.ExportScopes("publisher")
			if err != nil {
				t.Fatal(err)
			}
			got := make(map[string]bool)
			for _, scope := range scopes {
				present, known := scope.Membership("shared")
				if !known {
					t.Fatalf("scope %s lost known availability", scope.BindingID)
				}
				got[scope.BindingID] = present
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("scope availability = %v, want %v", got, test.want)
			}
		})
	}
}

func TestMembershipHistoryRejectsConflictsAndCancellation(t *testing.T) {
	one := membershipObservation(t, "one", sources.ProviderMembershipProvider, true, true, true, false, 1)
	two := membershipObservation(t, "two", sources.ProviderMembershipProvider, true, false, true, false, 1)
	if _, err := ResolveMembership(t.Context(), []sources.Observation{one, two}); !errors.IsConflict(err) {
		t.Fatalf("same-time disagreement: %v", err)
	}
	changed := membershipObservation(t, "one", sources.ProviderMembershipScope, true, true, true, false, 2)
	if _, err := ResolveMembership(t.Context(), []sources.Observation{one, changed}); !errors.IsConflict(err) {
		t.Fatalf("changed declaration: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ResolveMembership(ctx, nil); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled empty history: %v", err)
	}
	if _, err := ResolveMembership(nil, nil); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := New(WithMembershipState(nil)); err == nil {
		t.Fatal("nil membership state accepted")
	}
}

func TestMembershipHistoryExcludesUnhealthyProviderEvidence(t *testing.T) {
	removal := membershipObservation(t, "global", sources.ProviderMembershipProvider, true, false, true, false, 1)
	positive := membershipObservation(t, "account", sources.ProviderMembershipScope, false, true, true, false, 2)
	for _, subject := range []string{"provider-a", "unrelated"} {
		t.Run(subject, func(t *testing.T) {
			observation, err := sources.NewObservation(sources.ProvidersID, positive.Catalog, sources.ObservationMetadata{
				ProviderBinding: positive.ProviderBinding, ObservedAt: positive.ObservedAt,
				Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
				Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
				Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeProvider, Subject: subject, Code: sources.ObservationIssueCodeFetchFailed, Message: "Provider fetch failed."}},
			})
			if err != nil {
				t.Fatal(err)
			}
			state, err := ResolveMembership(t.Context(), []sources.Observation{removal, observation})
			if err != nil {
				t.Fatal(err)
			}
			scopes, err := state.ExportScopes("publisher")
			if err != nil {
				t.Fatal(err)
			}
			accountPresent := false
			for _, scope := range scopes {
				if scope.BindingID == "account" {
					accountPresent, _ = scope.Membership("shared")
				}
				if scope.BindingID == "global" {
					present, known := scope.Membership("shared")
					if present || !known {
						t.Fatal("account evidence changed public availability")
					}
				}
			}
			if accountPresent != (subject != "provider-a") {
				t.Fatalf("account availability = %t", accountPresent)
			}
		})
	}
}

func TestMembershipStateOwnsInputAndSupportsConcurrentReads(t *testing.T) {
	account := membershipObservation(t, "account", sources.ProviderMembershipScope, false, true, true, false, 1)
	removal := membershipObservation(t, "public", sources.ProviderMembershipProvider, true, false, true, false, 2)
	observations := []sources.Observation{account, removal}
	state, err := ResolveMembership(t.Context(), observations)
	if err != nil {
		t.Fatal(err)
	}
	account.ProviderBinding.Public = true
	account.ProviderBinding.AccountID = ""
	account.ProviderBinding.MembershipAuthority = sources.ProviderMembershipProvider
	clear(observations)
	var readers sync.WaitGroup
	for range 8 {
		readers.Go(func() {
			for range 100 {
				scopes, err := state.ExportScopes("publisher")
				if err != nil {
					t.Error(err)
					return
				}
				if len(scopes) != 2 {
					t.Error("caller mutation changed scope count")
					return
				}
				for _, scope := range scopes {
					present, known := scope.Membership("shared")
					if !known || present != (scope.BindingID == "account") {
						t.Error("caller mutation changed independent availability")
						return
					}
					present, known = scope.Membership("absent")
					if present || !known {
						t.Error("complete inventory lost known absence")
						return
					}
				}
			}
		})
	}
	readers.Wait()
}
