package starmap

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/differ"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
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

func TestSaveDoesNotPublishCatalogWhenPersistenceFails(t *testing.T) {
	oldCatalog := catalogs.NewEmpty()
	oldProvider := catalogs.Provider{ID: "old", Name: "Old Provider"}
	if err := oldCatalog.SetProvider(oldProvider); err != nil {
		t.Fatalf("Failed to seed old catalog: %v", err)
	}

	newCatalog := catalogs.NewEmpty()
	newProvider := catalogs.Provider{ID: "new", Name: "New Provider"}
	if err := newCatalog.SetProvider(newProvider); err != nil {
		t.Fatalf("Failed to seed new catalog: %v", err)
	}

	blockingFile := filepath.Join(t.TempDir(), "catalog-file")
	if err := os.WriteFile(blockingFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	c := &Client{
		catalog: mustTestCatalog(t, oldCatalog),
		hooks:   newHooks(),
	}

	_, err := c.save(context.Background(), newCatalog, &pkgsync.Options{OutputPath: blockingFile}, &differ.Changeset{}, nil)
	if err == nil {
		t.Fatal("Expected save to fail")
	}

	current := c.Catalog()
	if _, err := current.Provider("old"); err != nil {
		t.Fatalf("Expected old catalog to remain published after failed save: %v", err)
	}
	if _, err := current.Provider("new"); err == nil {
		t.Fatal("New catalog was published even though persistence failed")
	}
}

// TestF002CharacterizationStoreOnlyApplyFailsBeforeGenerationCommit pins the
// current F-002 defect. P3.8 must invert this assertion: a configured catalog
// store with no YAML workspace succeeds and performs no workspace filesystem
// operation.
func TestF002CharacterizationStoreOnlyApplyFailsBeforeGenerationCommit(t *testing.T) {
	store := catalogstore.NewMemory()
	opts := defaults()
	opts.catalogStore = store
	client := newWritableStoreTestClient(t, opts)

	candidate := catalogs.NewEmpty()
	if err := candidate.SetProvider(catalogs.Provider{ID: "store-only", Name: "Store Only"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	_, err := (pipelineStore{client: client}).Apply(
		context.Background(),
		candidate,
		&pkgsync.Options{},
		&differ.Changeset{},
		nil,
	)
	if err == nil {
		t.Fatal("F-002 characterization changed: store-only apply unexpectedly succeeded")
	}
	var ioErr *pkgerrors.IOError
	if !stderrors.As(err, &ioErr) {
		t.Fatalf("store-only apply error = %T: %v, want *errors.IOError", err, err)
	}
	if _, currentErr := store.Current(context.Background()); !pkgerrors.IsNotFound(currentErr) {
		t.Fatalf("generation store changed before YAML failure: %v", currentErr)
	}
	if _, currentErr := client.Catalog().Provider("store-only"); currentErr == nil {
		t.Fatal("failed store-only apply published the candidate in memory")
	}
}
