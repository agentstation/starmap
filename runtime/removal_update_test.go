package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestOperatorRemovalSurvivesRefreshAndRestartUntilRestore(t *testing.T) {
	binding := sources.ProviderAcquisitionBinding{
		SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion,
		ID:            "inventory", Revision: "1", ProviderID: "provider", Public: true,
		Region: "global", APISurface: "models.list", MembershipAuthority: sources.ProviderMembershipProvider,
		CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "public",
	}
	connected, options, at := providerResetRuntime(t, storage.NewMemory(), WithProviderBindings(binding))
	baseline := connected.Catalog()
	if _, err := connected.PublishObservations(t.Context(), providerResetObservation(t, sources.ProvidersID, baseline, at, &binding)); err != nil {
		t.Fatal(err)
	}
	scope := connected.Catalog().MembershipScopes()[0]
	target, err := catalogs.NewScopedRemovalTarget(scope, "model")
	if err != nil {
		t.Fatal(err)
	}
	before := connected.State()
	removed, err := connected.ReplaceRemovalTargets(t.Context(), before, target)
	if err != nil {
		t.Fatal(err)
	}
	if removed.GenerationID == before.GenerationID || !removed.Catalog.Removals().ContainsScoped(scope, "model") {
		t.Fatal("removal was not published")
	}
	if _, err := connected.ReplaceRemovalTargets(t.Context(), before); !errors.IsConflict(err) {
		t.Fatalf("stale restore did not conflict: %v", err)
	}
	if _, err := connected.PublishObservations(t.Context(), providerResetObservation(t, sources.ProvidersID, baseline, at.Add(time.Minute), &binding)); err != nil {
		t.Fatal(err)
	}
	if !connected.Catalog().Removals().ContainsScoped(scope, "model") {
		t.Fatal("provider refresh restored operator removal")
	}
	accepted := connected.State()
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != accepted.GenerationID || !reopened.Catalog().Removals().ContainsScoped(scope, "model") {
		t.Fatal("restart lost operator removal")
	}
	restored, err := reopened.ReplaceRemovalTargets(t.Context(), reopened.State())
	if err != nil {
		t.Fatal(err)
	}
	if restored.Catalog.Removals().ContainsScoped(scope, "model") {
		t.Fatal("explicit restore kept local removal")
	}
	if _, err := restored.Catalog.Offering("provider", "model"); err != nil {
		t.Fatal("restore lost underlying catalog facts")
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	final := openTestRuntime(t, options...)
	if final.State().GenerationID != restored.GenerationID || final.Catalog().Removals().ContainsScoped(scope, "model") {
		t.Fatal("restart undid explicit restore")
	}
}

func TestOperatorCanonicalRemovalPreservesSourceFacts(t *testing.T) {
	connected, _, _ := providerResetRuntime(t, storage.NewMemory())
	offering, err := connected.Catalog().Offering("provider", "model")
	if err != nil {
		t.Fatal(err)
	}
	target, err := catalogs.NewCanonicalRemovalTarget(offering.DefinitionID)
	if err != nil {
		t.Fatal(err)
	}
	state, err := connected.ReplaceRemovalTargets(t.Context(), connected.State(), target)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Catalog.Removals().ContainsCanonical(offering.DefinitionID) {
		t.Fatal("canonical removal missing")
	}
	if _, err := state.Catalog.Definition(offering.DefinitionID); err != nil {
		t.Fatal("operator policy rewrote source facts")
	}
}
