package reconciler

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAuthorshipSelectsDetailsFromAcceptedInput(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	input := authorshipCatalog(t, catalogs.Author{ID: "first-author", Name: "Accepted"})
	original := sourceIdentityObservation(t, sources.ProvidersID, input, at)
	generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
	local := sourceIdentityObservation(t, sources.LocalCatalogID, snapshotForTest(t, generated.Catalog), at.Add(time.Minute))
	stale, err := sources.NewObservation(sources.ProvidersID,
		authorshipCatalog(t, catalogs.Author{ID: "first-author", Name: "Rejected"}),
		sources.ObservationMetadata{ObservedAt: at.Add(2 * time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded, Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback, Message: "A source returned stale fallback data."}}})
	if err != nil {
		t.Fatal(err)
	}
	result := sourceIdentityReconcile(t, sources.LocalCatalogID, local, stale)
	provider, err := result.Catalog.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	model := provider.Models["shared"]
	if len(model.Authors) != 1 || model.Authors[0].Name != "Accepted" {
		t.Errorf("authors=%+v, want accepted details", model.Authors)
	}
	assertModelEvidenceSource(t, result.Catalog, `Authors["first-author"].present`, sources.ProvidersID, original.ID)
}

func TestAuthorshipProjectionRejectsRenamedCarryAndAllowsNewMembership(t *testing.T) {
	for _, add := range []bool{false, true} {
		name := "renamed-carry"
		if add {
			name = "new-membership"
		}
		t.Run(name, func(t *testing.T) {
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			original := sourceIdentityObservation(t, sources.ProvidersID, authorshipCatalog(t, catalogs.Author{ID: "first-author", Name: "Original"}), at)
			generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
			builder, err := catalogs.NewBuilderFrom(snapshotForTest(t, generated.Catalog))
			if err != nil {
				t.Fatal(err)
			}
			provider, err := builder.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			provider.Models["shared"].Authors[0].Name = "Edited details"
			if add {
				author := catalogs.Author{ID: "new-author", Name: "Local author"}
				if err := builder.SetAuthor(author); err != nil {
					t.Fatal(err)
				}
				provider.Models["shared"].Authors = append(provider.Models["shared"].Authors, author)
			}
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			catalog, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			local := sourceIdentityObservation(t, sources.LocalCatalogID, catalog, at.Add(time.Minute))
			reconcile, err := New(WithBaseline(catalog), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
				return !strings.HasPrefix(entry.Field, "Authors") || entry.ObservationID != original.ID
			}))
			if err != nil {
				t.Fatal(err)
			}
			result, err := reconcile.Sources(t.Context(), sources.LocalCatalogID, []sources.Observation{local})
			if err != nil {
				t.Fatal(err)
			}
			provider, err = result.Catalog.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			model := provider.Models["shared"]
			if slices.ContainsFunc(model.Authors, func(author catalogs.Author) bool { return author.ID == "first-author" }) {
				t.Error("an author-detail edit restored rejected membership")
			}
			entries := result.Catalog.Provenance().FindModelField("provider-a", "shared", `Authors["first-author"].present`)
			if len(entries) != 1 || entries[0].Value != false || entries[0].ObservationID != "" {
				t.Error("rejected authorship kept its positive source receipt")
			}
			if !add {
				for _, entry := range result.Catalog.Provenance().FindModelField("provider-a", "shared", "Authors") {
					if entry.ObservationID == original.ID {
						t.Error("empty authorship kept its positive aggregate receipt")
					}
				}
			}
			if model.ModelRef != "test-author/shared" {
				t.Error("authorship policy changed the serving identity")
			}
			if add {
				if len(model.Authors) != 1 || model.Authors[0].ID != "new-author" {
					t.Error("explicit local membership did not survive")
				}
				assertModelEvidenceSource(t, result.Catalog, `Authors["new-author"].present`, sources.LocalCatalogID, local.ID)
			}
		})
	}
}

func TestAuthorshipPreservesBaselineAndCallerOwnership(t *testing.T) {
	baseline := authorshipCatalog(t, catalogs.Author{ID: "first-author", Name: "First"})
	input := authorshipCatalog(t, catalogs.Author{ID: "second-author", Name: "Second"})
	observation := sourceIdentityObservation(t, sources.ProvidersID, input, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{observation})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := result.Catalog.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.Models["shared"].Authors) != 2 {
		t.Error("a new author erased omitted baseline membership")
	}
	description := "original"
	author := catalogs.Author{ID: "first-author", Name: "First", Description: &description, Aliases: []catalogs.AuthorID{"first"}, Logo: []byte("original")}
	policy := authority.New()
	merger := newMerger(policy, NewAuthorityStrategy(policy), nil)
	models, _, err := merger.Models(map[sources.ID][]*catalogs.Model{sources.ProvidersID: {{ID: "shared", Name: "Shared", Authors: []catalogs.Author{author}}}})
	if err != nil {
		t.Fatal(err)
	}
	*models[0].Authors[0].Description = "changed"
	models[0].Authors[0].Aliases[0] = "changed"
	models[0].Authors[0].Logo[0] = 'X'
	if description != "original" || author.Aliases[0] != "first" || string(author.Logo) != "original" {
		t.Error("merged author details alias source data")
	}
}

func TestAuthorshipMissingLocalClaimKeepsBaseline(t *testing.T) {
	baseline := authorshipCatalog(t, catalogs.Author{ID: "first-author", Name: "First"})
	local := sourceIdentityObservation(t, sources.LocalCatalogID, authorshipCatalog(t), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{local})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := result.Catalog.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.Models["shared"].Authors) != 1 || provider.Models["shared"].Authors[0].ID != "first-author" {
		t.Error("missing local authorship erased the baseline")
	}
}

func TestAuthorshipWorkspaceRetainsEachSourceReceipt(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	first := sourceIdentityObservation(t, sources.ProvidersID, authorshipCatalog(t, catalogs.Author{ID: "first-author", Name: "First"}), at)
	second := sourceIdentityObservation(t, sources.ModelsDevHTTPID, authorshipCatalog(t, catalogs.Author{ID: "second-author", Name: "Second"}), at.Add(time.Minute))
	generated := sourceIdentityReconcile(t, sources.ProvidersID, first, second)
	workspace := filepath.Join(t.TempDir(), "catalog")
	if err := generated.Catalog.SaveTo(workspace); err != nil {
		t.Fatal(err)
	}
	local := sourceIdentityObservation(t, sources.LocalCatalogID, sourceIdentityLoad(t, workspace), at.Add(2*time.Minute))
	result := sourceIdentityReconcile(t, sources.LocalCatalogID, local)
	assertModelEvidenceSource(t, result.Catalog, `Authors["first-author"].present`, sources.ProvidersID, first.ID)
	assertModelEvidenceSource(t, result.Catalog, `Authors["second-author"].present`, sources.ModelsDevHTTPID, second.ID)
}

func authorshipCatalog(t *testing.T, authors ...catalogs.Author) *catalogs.Catalog {
	t.Helper()
	seed := sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared"})
	builder, err := catalogs.NewBuilderFrom(seed)
	if err != nil {
		t.Fatal(err)
	}
	for _, author := range authors {
		if err := builder.SetAuthor(author); err != nil {
			t.Fatal(err)
		}
	}
	provider, err := builder.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models["shared"].Authors = authors
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
