package acquisition

import (
	"context"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestImportReleaseCannotReplaceOperatorPolicy(t *testing.T) {
	t.Parallel()
	client, err := starmap.New(starmap.WithCatalogStore(storage.NewMemory()))
	if err != nil {
		t.Fatal(err)
	}
	baseline := importCatalogBuilder(t, "Local Model", "", false, true)
	target, err := catalogs.NewCanonicalRemovalTarget("release-lab/model-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := baseline.SetRemovalPolicies([]catalogs.CatalogRemovalPolicy{{PublisherID: "local-operator", Targets: []catalogs.CatalogRemovalTarget{target}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
		return starmap.NewCandidate(buildImportCatalog(t, baseline), starmap.CandidateEvidence{})
	}); err != nil {
		t.Fatal(err)
	}
	syncer, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	incoming := importCatalogBuilder(t, "Release Model", "Description", true, false)
	release := importReleaseFixture(t, buildImportCatalog(t, incoming))
	result, err := syncer.ImportRelease(t.Context(), release, &importPublisherVerifier{want: release.Archive})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Publication.Published || !client.Catalog().Removals().ContainsCanonical(target.DefinitionID) {
		t.Fatal("ordinary release import lost accepted operator removal")
	}
	before := client.CurrentGenerationID()
	if err := incoming.SetRemovalPolicies([]catalogs.CatalogRemovalPolicy{{PublisherID: "local-operator", Targets: nil}}); err != nil {
		t.Fatal(err)
	}
	untrustedPolicy := importReleaseFixture(t, buildImportCatalog(t, incoming))
	if _, err := syncer.ImportRelease(t.Context(), untrustedPolicy, &importPublisherVerifier{want: untrustedPolicy.Archive}); err == nil {
		t.Fatal("fact import accepted operator policy")
	}
	if client.CurrentGenerationID() != before || !client.Catalog().Removals().ContainsCanonical(target.DefinitionID) {
		t.Fatal("refused policy import changed the active catalog")
	}
}
