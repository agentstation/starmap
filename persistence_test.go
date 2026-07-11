package starmap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
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
