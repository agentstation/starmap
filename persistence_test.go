package starmap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/save"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
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

	if err := sm.Save(save.WithPath(t.TempDir())); err != nil {
		t.Fatalf("Save returned error after successful catalog save: %v", err)
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

	publication, err := c.save(
		context.Background(),
		newCatalog,
		&pkgsync.Options{OutputPath: blockingFile},
		&differ.Changeset{},
		[]sources.Observation{persistenceObservation(t, newCatalog)},
	)
	if err != nil {
		t.Fatalf("save returned a pre-commit error after projection failure: %v", err)
	}
	if publication.Projection == nil ||
		publication.Projection.Status != pkgsync.ProjectionStatusPendingRepair ||
		publication.Projection.IssueCode != workspaceProjectionFailureIssue {
		t.Fatalf("projection = %#v, want pending repair", publication.Projection)
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

func TestStoreOnlyApplyCommitsWithoutWorkspaceAccess(t *testing.T) {
	store := catalogstore.NewMemory()
	opts := defaults()
	opts.catalogStore = store
	client := newWritableStoreTestClient(t, opts)

	candidate := catalogs.NewEmpty()
	if err := candidate.SetProvider(catalogs.Provider{ID: "store-only", Name: "Store Only"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	publication, err := (pipelineStore{client: client}).Apply(
		context.Background(),
		candidate,
		&pkgsync.Options{},
		&differ.Changeset{},
		[]sources.Observation{persistenceObservation(t, candidate)},
	)
	if err != nil {
		t.Fatalf("store-only Apply: %v", err)
	}
	if publication.Projection != nil {
		t.Fatalf("store-only projection = %#v, want nil", publication.Projection)
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

func persistenceObservation(t testing.TB, builder *catalogs.Builder) sources.Observation {
	t.Helper()
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build observation catalog: %v", err)
	}
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
