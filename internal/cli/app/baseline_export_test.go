package app

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestPersistentColdStartExportsInstalledBaseline(t *testing.T) {
	clearCatalogEnvironment(t)
	home, productHome := t.TempDir(), t.TempDir()
	setTestHome(t, home)
	t.Setenv("STARMAP_HOME", productHome)
	application, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{
		catalogconfig.Source: "embedded", catalogconfig.AcquisitionEnabled: "false", catalogconfig.SourcePollInterval: "0s",
	}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := application.Starmap(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(productHome)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Fatal("passive application access created persistent files")
	}
	if _, err := application.Runtime(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := application.closeRuntime(); err != nil {
			t.Error(err)
		}
	})
	baseline, err := starmap.EmbeddedGeneration()
	if err != nil {
		t.Fatal(err)
	}
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(baseline.Manifest.GenerationID)))
	directory := filepath.Join(productHome, "data", "catalog", "baseline", identity)
	manifestBytes, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatalf("persistent startup did not export the installed baseline: %v", err)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(directory, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Payload.Verify(payload); err != nil {
		t.Fatal(err)
	}
	if manifest.GenerationID != baseline.Manifest.GenerationID || manifest.Payload.Checksum != baseline.Manifest.Payload.Checksum {
		t.Fatal("persistent export does not identify the installed embedded baseline")
	}
}

func TestBaselineExportPreservesExistingAcceptedHead(t *testing.T) {
	clearCatalogEnvironment(t)
	setTestHome(t, t.TempDir())
	t.Setenv("STARMAP_HOME", t.TempDir())
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.Source: "embedded", catalogconfig.AcquisitionEnabled: "false", catalogconfig.SourcePollInterval: "0s"}}))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewFilesystem(paths.CatalogStore.Path)
	if err != nil {
		t.Fatal(err)
	}
	accepted := validCatalogGeneration(t, "accepted-before-baseline-export")
	if err := store.Commit(t.Context(), accepted, ""); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := a.Runtime(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := a.closeRuntime(); err != nil {
			t.Fatal(err)
		}
		retained, err := store.Current(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if retained.Manifest.GenerationID != accepted.Manifest.GenerationID || !bytes.Equal(retained.Payload, accepted.Payload) {
			t.Fatal("baseline export replaced the accepted catalog")
		}
	}
}

func TestRequiredBaselineWriteFailurePreventsRuntimeStartup(t *testing.T) {
	clearCatalogEnvironment(t)
	setTestHome(t, t.TempDir())
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "data"), []byte("keep-this-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.Source: "embedded", catalogconfig.AcquisitionEnabled: "false"}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Runtime(t.Context()); err == nil {
		t.Fatal("persistent runtime started without its required baseline export")
	}
	if a.runtime != nil {
		t.Fatal("failed baseline export left a running runtime")
	}
	if _, err := os.Stat(filepath.Join(home, "state")); !os.IsNotExist(err) {
		t.Fatal("failed baseline export wrote process state")
	}
	data, err := os.ReadFile(filepath.Join(home, "data"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep-this-file" {
		t.Fatal("baseline export replaced an existing file")
	}
}
