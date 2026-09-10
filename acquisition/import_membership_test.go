package acquisition

import (
	"context"
	stderrors "errors"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestArtifactImportRetainsOriginalMembershipEvidence(t *testing.T) {
	for name, changed := range map[string]bool{"scope only": false, "changed facts": true} {
		t.Run(name, func(t *testing.T) {
			baseline := buildImportCatalog(t, importCatalogBuilder(t, "Model", "", false, false))
			store := storage.NewMemory()
			client, err := starmap.New(starmap.WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
				return starmap.NewCandidate(baseline, starmap.CandidateEvidence{})
			}); err != nil {
				t.Fatal(err)
			}
			incoming, scope, originalLink := importMembershipCatalog(t, "artifact-publisher", changed)
			release := importReleaseFixture(t, incoming, originalLink)
			syncer, err := New(client)
			if err != nil {
				t.Fatal(err)
			}
			result, err := syncer.ImportRelease(t.Context(), release, &importPublisherVerifier{want: release.Archive})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Publication.Published || !reflect.DeepEqual(client.Catalog().MembershipScopes(), []catalogs.ProviderMembershipScope{scope}) {
				t.Fatal("artifact import lost scoped membership")
			}
			generation, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := catalogs.DecodeCatalogGeneration(generation); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, link := range generation.Manifest.SourceObservations {
				found = found || link == originalLink
			}
			if !found {
				t.Fatal("artifact import lost the original provider receipt")
			}
			restarted, err := starmap.New(starmap.WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(restarted.Catalog().MembershipScopes(), []catalogs.ProviderMembershipScope{scope}) {
				t.Fatal("restart lost imported scope")
			}
		})
	}
}

func TestArtifactImportPreservesExistingMembership(t *testing.T) {
	for _, kind := range []string{"fact only", "independent scope", "identical scope", "conflicting scope"} {
		t.Run(kind, func(t *testing.T) {
			prior, priorScope, priorLink := importMembershipCatalog(t, "a-local", false)
			store := storage.NewMemory()
			client, err := starmap.New(starmap.WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
				return starmap.NewCandidate(prior, starmap.CandidateEvidence{SourceObservations: []catalogs.SourceObservationLink{priorLink}})
			}); err != nil {
				t.Fatal(err)
			}
			before, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			incoming := buildImportCatalog(t, importCatalogBuilder(t, "Model", "", true, false))
			scopes := []catalogs.ProviderMembershipScope{priorScope}
			var links []catalogs.SourceObservationLink
			switch kind {
			case "independent scope":
				var scope catalogs.ProviderMembershipScope
				var link catalogs.SourceObservationLink
				incoming, scope, link = importMembershipCatalog(t, "b-release", true)
				scopes = append(scopes, scope)
				links = append(links, link)
			case "identical scope":
				builder := importCatalogBuilder(t, "Model", "", true, false)
				if err := builder.SetMembershipScopes(scopes); err != nil {
					t.Fatal(err)
				}
				incoming = buildImportCatalog(t, builder)
				links = append(links, priorLink)
			case "conflicting scope":
				var link catalogs.SourceObservationLink
				incoming, _, link = importMembershipCatalog(t, "a-local", true)
				links = append(links, link)
			}
			release := importReleaseFixture(t, incoming, links...)
			syncer, err := New(client)
			if err != nil {
				t.Fatal(err)
			}
			result, err := syncer.ImportRelease(t.Context(), release, &importPublisherVerifier{want: release.Archive})
			if kind == "conflicting scope" {
				var conflict *pkgerrors.ConflictError
				if !stderrors.As(err, &conflict) {
					t.Fatalf("error = %v, want scope conflict", err)
				}
				after, err := store.Current(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, after) || client.Catalog() != prior {
					t.Fatal("conflicting scope changed the store or active catalog")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !result.Publication.Published {
				t.Fatal("new model facts did not publish")
			}
			if _, err := client.Catalog().Offering("provider-a", "model-b"); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(client.Catalog().MembershipScopes(), scopes) {
				t.Fatal("import changed existing scope evidence or lost an independent scope")
			}
			generation, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := catalogs.DecodeCatalogGeneration(generation); err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, link := range generation.Manifest.SourceObservations {
				if link == priorLink {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("prior receipt count = %d, want 1", count)
			}
			restarted, err := starmap.New(starmap.WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(restarted.Catalog().MembershipScopes(), scopes) {
				t.Fatal("restart lost scope evidence")
			}
			replay, err := syncer.ImportRelease(t.Context(), release, &importPublisherVerifier{want: release.Archive})
			if err != nil {
				t.Fatal(err)
			}
			if replay.Publication.Published || client.CurrentGenerationID() != generation.Manifest.GenerationID {
				t.Fatal("identical import published another generation")
			}
		})
	}
}

func importMembershipCatalog(t testing.TB, publisher string, changed bool) (*catalogs.Catalog, catalogs.ProviderMembershipScope, catalogs.SourceObservationLink) {
	t.Helper()
	builder := importCatalogBuilder(t, "Model", "", changed, false)
	binding := sources.ProviderAcquisitionBinding{
		SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion,
		ID:            "account", Revision: "1", ProviderID: "provider-a", AccountID: "account",
		Region: "global", APISurface: "models.list", MembershipAuthority: sources.ProviderMembershipScope,
		CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog",
	}
	observation, err := sources.NewObservation(sources.ProvidersID, buildImportCatalog(t, builder), sources.ObservationMetadata{
		ProviderBinding: &binding, ObservedAt: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	models := []string{"model-a"}
	if changed {
		models = append(models, "model-b")
	}
	scope := catalogs.ProviderMembershipScope{
		PublisherID: publisher, BindingID: binding.ID, BindingRevision: binding.Revision,
		ProviderID: binding.ProviderID, AccountID: binding.AccountID, Region: binding.Region,
		APISurface: binding.APISurface, Authority: catalogs.MembershipScopeAuthority,
		Inventory: &catalogs.MembershipInventory{ObservationID: observation.ID, ObservedAt: observation.ObservedAt, ModelIDs: models},
	}
	if err := builder.SetMembershipScopes([]catalogs.ProviderMembershipScope{scope}); err != nil {
		t.Fatal(err)
	}
	return buildImportCatalog(t, builder), scope, observation.Link()
}
