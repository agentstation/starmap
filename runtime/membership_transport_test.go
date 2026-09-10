package runtime

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestRuntimePublishesScopedInventoryAndOriginalReceipts(t *testing.T) {
	binding := sources.ProviderAcquisitionBinding{
		SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion,
		ID:            "account-a", Revision: "1", ProviderID: "provider", AccountID: "account-a",
		Region: "global", APISurface: "models.list", MembershipAuthority: sources.ProviderMembershipScope,
		CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog",
	}
	store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	connected, options, at := providerResetRuntime(t, store, WithProviderBindings(binding))
	baseline := connected.State().Catalog
	builder, err := catalogs.NewBuilderFrom(baseline)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models = nil
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	empty, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	complete := providerResetObservation(t, sources.ProvidersID, empty, at, &binding)
	if _, err := connected.PublishObservations(t.Context(), complete); err != nil {
		t.Fatal(err)
	}
	publisher := connected.Status().InstanceIdentity
	check := func(runtime *Runtime, positive bool) {
		t.Helper()
		scopes := runtime.State().Catalog.MembershipScopes()
		if len(scopes) != 1 {
			t.Fatalf("published scopes = %d, want one account scope", len(scopes))
		}
		scope := scopes[0]
		if scope.PublisherID != publisher || scope.BindingID != binding.ID || scope.BindingRevision != binding.Revision || scope.AccountID != binding.AccountID {
			t.Fatal("scope identity changed")
		}
		if scope.Inventory == nil || scope.Inventory.ObservationID != complete.ID || len(scope.Inventory.ModelIDs) != 0 {
			t.Fatal("complete empty inventory lost its receipt")
		}
		key := catalogs.MembershipScopeKey{PublisherID: publisher, BindingID: binding.ID, BindingRevision: binding.Revision}
		present, known := runtime.State().Catalog.ScopeMembership(key, "provider", "model")
		if !known || present != positive {
			t.Fatalf("scope membership = %t/%t, want %t/true", present, known, positive)
		}
		provider, err := runtime.State().Catalog.Provider("provider")
		if err != nil || provider.Models["model"] == nil {
			t.Fatal("account absence changed canonical discovery", err)
		}
		generation, err := store.Current(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := catalogs.DecodeCatalogGeneration(generation); err != nil {
			t.Fatalf("published scope evidence: %v", err)
		}
	}
	check(connected, false)
	partial, err := sources.NewObservation(sources.ProvidersID, baseline, sources.ObservationMetadata{
		ProviderBinding: &binding, ObservedAt: at.Add(time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
		Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeSchemaDrift, Message: "Inventory is incomplete."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.PublishObservations(t.Context(), partial); err != nil {
		t.Fatal(err)
	}
	check(connected, true)
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, options...)
	check(restarted, true)
	request := DirectoryMigrationRequest{
		OperationID: "scope-move", SourceDirectory: restarted.config.stateDirectory,
		TargetDirectory: filepath.Join(t.TempDir(), "moved"), JournalRoot: privateRuntimeDirectory(t),
		SourceIdentity: publisher, Owner: restarted.config.directoryOwner,
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	movedOptions := append([]Option{}, options...)
	movedOptions = append(movedOptions, WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(publisher), WithDirectoryOwner(request.Owner), WithPublishedDirectoryMigration(request))
	moved := openTestRuntime(t, movedOptions...)
	check(moved, true)
	if _, err := moved.CompleteDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
}
