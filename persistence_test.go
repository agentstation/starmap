package starmap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSave(t *testing.T) {
	// Create a starmap instance
	sm, err := New()
	if err != nil {
		t.Fatalf("Failed to create starmap: %v", err)
	}

	// Test Save method - this should work with embedded catalog
	err = sm.Save()
	if err != nil {
		t.Logf("Save failed (expected for embedded catalog): %v", err)
		// This is expected to fail for embedded catalogs that don't support saving
	}
}

func TestSaveReturnsNilAfterSuccessfulCatalogSave(t *testing.T) {
	sm, err := New()
	if err != nil {
		t.Fatalf("Failed to create starmap: %v", err)
	}

	path := t.TempDir()
	if err := sm.SaveTo(path); err != nil {
		t.Fatalf("Save returned error after successful catalog save: %v", err)
	}
	projected, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("Load projected catalog: %v", err)
	}
	projectedCatalog, err := projected.Build()
	if err != nil {
		t.Fatalf("Build projected catalog: %v", err)
	}
	wantPayload, err := catalogs.EncodeCatalogPayload(sm.Catalog())
	if err != nil {
		t.Fatalf("Encode active catalog: %v", err)
	}
	gotPayload, err := catalogs.EncodeCatalogPayload(projectedCatalog)
	if err != nil {
		t.Fatalf("Encode projected catalog: %v", err)
	}
	if got, want := catalogs.DescribeCatalogPayload(gotPayload).Checksum,
		catalogs.DescribeCatalogPayload(wantPayload).Checksum; got != want {
		t.Fatalf("projected checksum = %q, want committed %q", got, want)
	}
}

func TestProjectionFailureLeavesCommittedGenerationActiveAndReportsRepair(t *testing.T) {
	newCatalog := catalogs.NewEmpty()
	newProvider := catalogs.Provider{ID: "new", Name: "New Provider"}
	if err := newCatalog.SetProvider(newProvider); err != nil {
		t.Fatalf("Failed to seed new catalog: %v", err)
	}

	blockingFile := filepath.Join(t.TempDir(), "catalog-file")
	if err := os.WriteFile(blockingFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	store := catalogstore.NewMemory()
	opts := defaults()
	opts.catalogStore = store
	c := newWritableStoreTestClient(t, opts)

	published := mustTestCatalog(t, newCatalog)
	observation := persistenceObservation(t, newCatalog)
	publication, err := c.Update(context.Background(), func(
		context.Context,
		*catalogs.Catalog,
	) (*Candidate, error) {
		return NewCandidate(published, observation.Link())
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	projection := projectRollbackCatalog(
		context.Background(),
		published,
		blockingFile,
		workspace.Identity{
			GenerationID:    publication.GenerationID,
			PayloadChecksum: publication.PayloadChecksum,
		},
		workspace.InputExpectation{},
	)
	if projection.Status != catalogmeta.ProjectionStatusPendingRepair ||
		projection.IssueCode != catalogmeta.ProjectionIssueWorkspaceFailed {
		t.Fatalf("projection = %#v, want pending repair", projection)
	}
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Manifest.GenerationID != publication.GenerationID {
		t.Fatalf("store generation = %q, want %q", current.Manifest.GenerationID, publication.GenerationID)
	}

	if _, err := c.Catalog().Provider("new"); err != nil {
		t.Fatalf("committed catalog was not published after projection failure: %v", err)
	}
}

func TestSuccessfulProjectionReportsCommittedGenerationAndWorkspaceDigest(t *testing.T) {
	candidate := catalogs.NewEmpty()
	if err := candidate.SetProvider(catalogs.Provider{ID: "projected", Name: "Projected Provider"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	store := catalogstore.NewMemory()
	opts := defaults()
	opts.catalogStore = store
	client := newWritableStoreTestClient(t, opts)
	path := filepath.Join(t.TempDir(), "catalog")

	published := mustTestCatalog(t, candidate)
	observation := persistenceObservation(t, candidate)
	publication, err := client.Update(context.Background(), func(
		context.Context,
		*catalogs.Catalog,
	) (*Candidate, error) {
		return NewCandidate(published, observation.Link())
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	projection := projectRollbackCatalog(
		context.Background(),
		published,
		path,
		workspace.Identity{
			GenerationID:    publication.GenerationID,
			PayloadChecksum: publication.PayloadChecksum,
		},
		workspace.InputExpectation{},
	)
	if projection.Status != catalogmeta.ProjectionStatusApplied {
		t.Fatalf("projection = %#v, want applied", projection)
	}
	if projection.GenerationID != publication.GenerationID {
		t.Fatalf("projection generation = %q, want %q", projection.GenerationID, publication.GenerationID)
	}
	if projection.WorkspaceChecksum == "" || projection.IssueCode != "" {
		t.Fatalf("projection = %#v, want checksum without issue", projection)
	}
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if projection.WorkspaceChecksum != current.Manifest.Payload.Checksum {
		t.Fatalf(
			"workspace checksum = %q, want committed payload %q",
			projection.WorkspaceChecksum,
			current.Manifest.Payload.Checksum,
		)
	}
	projected, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("Load projected workspace: %v", err)
	}
	if _, err := projected.Provider("projected"); err != nil {
		t.Fatalf("Projected provider missing: %v", err)
	}
}

func TestStoreOnlyApplyCommitsWithoutWorkspaceAccess(t *testing.T) {
	store := catalogstore.NewMemory()
	opts := defaults()
	opts.catalogStore = store
	client := newWritableStoreTestClient(t, opts)

	candidate := catalogs.NewEmpty()
	if err := candidate.SetProvider(catalogs.Provider{ID: "store-only", Name: "Store Only"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	published := mustTestCatalog(t, candidate)
	observation := persistenceObservation(t, candidate)
	publication, err := client.Update(context.Background(), func(
		context.Context,
		*catalogs.Catalog,
	) (*Candidate, error) {
		return NewCandidate(published, observation.Link())
	})
	if err != nil {
		t.Fatalf("store-only Update: %v", err)
	}
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Manifest.GenerationID != publication.GenerationID {
		t.Fatalf("store generation = %q, want %q", current.Manifest.GenerationID, publication.GenerationID)
	}
	if _, err := client.Catalog().Provider("store-only"); err != nil {
		t.Fatalf("store-only candidate was not published: %v", err)
	}
}

func TestNewRepairsStaleProjectionFromDurableCurrentWithoutRepublishing(t *testing.T) {
	oldBuilder := catalogs.NewEmpty()
	if err := oldBuilder.SetProvider(catalogs.Provider{ID: "old", Name: "Old Provider"}); err != nil {
		t.Fatalf("SetProvider old: %v", err)
	}
	oldCatalog, err := oldBuilder.Build()
	if err != nil {
		t.Fatalf("Build old catalog: %v", err)
	}
	oldPayload, err := catalogs.EncodeCatalogPayload(oldCatalog)
	if err != nil {
		t.Fatalf("Encode old catalog: %v", err)
	}
	path := filepath.Join(t.TempDir(), "catalog")
	if _, err := workspace.Project(
		context.Background(),
		path,
		oldCatalog,
		workspace.Identity{
			GenerationID:    "old-generation",
			PayloadChecksum: catalogs.DescribeCatalogPayload(oldPayload).Checksum,
		},
	); err != nil {
		t.Fatalf("Project old workspace: %v", err)
	}

	store := catalogstore.NewMemory()
	current := rootRemoteGeneration(t)
	if err := store.Commit(context.Background(), current, ""); err != nil {
		t.Fatalf("Commit durable current: %v", err)
	}

	client, err := New(WithCatalogStore(store), WithCatalogPath(path))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if client.CurrentGenerationID() != current.Manifest.GenerationID {
		t.Fatalf("generation = %q, want %q", client.CurrentGenerationID(), current.Manifest.GenerationID)
	}
	if client.CurrentCatalogState().Sequence != 1 {
		t.Fatalf("startup repair republished catalog; sequence = %d, want 1", client.CurrentCatalogState().Sequence)
	}
	if _, err := client.Catalog().Provider("remote-root"); err != nil {
		t.Fatalf("durable catalog is not active: %v", err)
	}

	repaired, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("Load repaired workspace: %v", err)
	}
	if _, err := repaired.Provider("remote-root"); err != nil {
		t.Fatalf("repaired workspace does not match durable current: %v", err)
	}
	if _, err := repaired.Provider("old"); err == nil {
		t.Fatal("stale projected provider survived startup repair")
	}
	stillCurrent, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current after repair: %v", err)
	}
	if stillCurrent.Manifest.GenerationID != current.Manifest.GenerationID {
		t.Fatalf("store changed to %q, want %q", stillCurrent.Manifest.GenerationID, current.Manifest.GenerationID)
	}
}

func persistenceObservation(t testing.TB, builder *catalogs.Builder) sources.Observation {
	t.Helper()
	catalog := mustTestCatalog(t, builder)
	observation, err := sources.NewObservation(sources.LocalCatalogID, catalog, sources.ObservationMetadata{
		ObservedAt:   time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	return observation
}
