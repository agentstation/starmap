package app

import (
	"bytes"
	"encoding/json"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/productpaths"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/sources"
)

func TestConfigPathsCommandIsPassiveAndReportsOrigins(t *testing.T) {
	clearCatalogEnvironment(t)
	root := t.TempDir()
	t.Setenv("STARMAP_HOME", root)
	app := NewForCommand("test", "test", "test", "test")
	command := app.createRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	cache := filepath.Join(root, "selected-cache")
	command.SetArgs([]string{"config", "paths", "--output", "json", "--cache-dir", cache})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("path report command failed: %v", err)
	}
	var report struct {
		SchemaVersion int    `json:"schema_version"`
		Product       string `json:"product"`
		Files         []struct {
			ID       string                        `json:"id"`
			Location struct{ Path, Origin string } `json:"location"`
		} `json:"files"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 1 || report.Product != "starmap" {
		t.Fatalf("unexpected report: %s", output.String())
	}
	found := false
	for _, entry := range report.Files {
		if entry.ID == "source-http" {
			found = true
			if entry.Location.Path != filepath.Join(cache, "models.dev") || entry.Location.Origin != "options" {
				t.Fatalf("source cache origin or path lost: %+v", entry)
			}
		}
	}
	if !found {
		t.Fatal("source cache missing from file inventory")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("path report created product files")
	}
	if app.runtime != nil || app.starmap != nil {
		t.Fatal("path report initialized a catalog")
	}
}

func TestFileManifestCoversColdStartupAndPublication(t *testing.T) {
	clearCatalogEnvironment(t)
	root := t.TempDir()
	t.Setenv("STARMAP_HOME", root)
	app, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}}))
	if err != nil {
		t.Fatal(err)
	}
	report, err := app.FileManifest()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := app.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	connected, err := app.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	input, err := catalogs.NewObservationCatalog(catalogs.NewEmpty())
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.LocalCatalogID, input, sources.ObservationMetadata{
		ObservedAt: time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
		t.Fatal(err)
	}
	if err := app.closeRuntime(); err != nil {
		t.Fatal(err)
	}
	covered := 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !manifestCoversFile(t, report, path) {
			t.Errorf("persistent startup file missing from manifest: %s", path)
		}
		covered++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if covered < 8 {
		t.Fatalf("startup evidence is incomplete: %d files", covered)
	}
	if !manifestCoversFile(t, report, filepath.Join(paths.Runtime.Path, "catalog-runtime", "providers", "bindings", strings.Repeat("a", 64)+".json")) {
		t.Fatal("scoped provider record is absent from the manifest")
	}
	if manifestCoversFile(t, report, filepath.Join(paths.Runtime.Path, "catalog-runtime", "unexpected-file")) {
		t.Fatal("broad evidence entry hides an unknown managed file")
	}
	if manifestCoversFile(t, report, filepath.Join(paths.Runtime.Path, "unexpected-file")) {
		t.Fatal("broad runtime entry hides an unknown managed file")
	}
	t.Logf("covered %d persistent startup files", covered)
}

func manifestCoversFile(t *testing.T, report productpaths.FileManifest, path string) bool {
	t.Helper()
	for _, entry := range report.Files {
		if entry.Availability != "available" {
			continue
		}
		if entry.Kind == "file" {
			if entry.Location.Path == path {
				return true
			}
			continue
		}
		relative, err := filepath.Rel(entry.Location.Path, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		relative = filepath.ToSlash(relative)
		for _, pattern := range entry.Patterns {
			if pattern == "**" {
				return true
			}
			if strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(pattern, "/**")
				count := strings.Count(prefix, "/") + 1
				parts := strings.Split(relative, "/")
				if len(parts) > count {
					relativePrefix := strings.Join(parts[:count], "/")
					if matched, _ := pathpkg.Match(prefix, relativePrefix); matched {
						return true
					}
				}
			} else if matched, _ := pathpkg.Match(pattern, relative); matched {
				return true
			}
		}
	}
	return false
}

func TestFileManifestReportsDisabledWorkspaceAndPlannedFeatures(t *testing.T) {
	clearCatalogEnvironment(t)
	root := t.TempDir()
	t.Setenv("STARMAP_HOME", root)
	app, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.WorkspacePath: ""}}))
	if err != nil {
		t.Fatal(err)
	}
	report, err := app.FileManifest()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	planned := 0
	for _, entry := range report.Files {
		if seen[entry.ID] {
			t.Fatalf("duplicate file role: %s", entry.ID)
		}
		seen[entry.ID] = true
		if entry.Creation == "" || entry.Recovery == "" {
			t.Fatalf("file lifecycle missing: %s", entry.ID)
		}
		if entry.Availability == "planned" {
			planned++
		}
		if entry.ID == "workspace" {
			if entry.Availability != "disabled" || entry.Location.Path != "" {
				t.Fatal("disabled workspace received a file location")
			}
		} else if !filepath.IsAbs(entry.Location.Path) {
			t.Fatalf("file role has no absolute base: %s", entry.ID)
		}
	}
	if seen["workspace-lock"] || seen["workspace-receipt"] || seen["workspace-journal"] || seen["workspace-backup"] || seen["workspace-staging"] {
		t.Fatal("disabled workspace has active sibling files")
	}
	if planned != 6 || !seen["runtime-seed"] || !seen["migration-journal"] {
		t.Fatalf("inventory omits planned or persistent roles: %+v", seen)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("inventory created reserved paths")
	}
}

func TestFileManifestCoversWorkspaceRecoveryArtifacts(t *testing.T) {
	parent := t.TempDir()
	var report productpaths.FileManifest
	addWorkspaceFiles(&report, ProductPaths{Workspace: productpaths.Path{Path: filepath.Join(parent, "workspace[local]")}})
	for _, name := range []string{
		".workspace[local].starmap-replacement.json",
		"..workspace[local].starmap-replacement.json.temporary",
		".workspace[local].backup-123/notes.txt",
		".workspace[local].backup-123/providers/openai.yaml",
		".workspace[local].candidate-123/providers/openai.yaml",
		".workspace[local].preparing-123",
		".workspace[local].preparing-123/render/providers/openai.yaml",
	} {
		if !manifestCoversFile(t, report, filepath.Join(parent, filepath.FromSlash(name))) {
			t.Errorf("workspace recovery artifact is absent from the inventory: %s", name)
		}
	}
	if manifestCoversFile(t, report, filepath.Join(parent, ".workspace-other.backup-123", "notes.txt")) {
		t.Fatal("workspace inventory includes another workspace's backup")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("workspace inventory created files")
	}
}

func TestFileManifestPreservesLeafAnchorsWithoutCredentialValues(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv(relativePathBaseName, "config")
	root := t.TempDir()
	t.Setenv("STARMAP_HOME", root)
	secret := "file-manifest-secret-value"
	app, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{
		catalogconfig.WorkspacePath:  "workspace",
		catalogconfig.StateDirectory: "runtime",
		catalogconfig.Source:         "starmap",
		catalogconfig.SourceURL:      "https://catalog.example.test",
		catalogconfig.SourceAPIKey:   secret,
	}}))
	if err != nil {
		t.Fatal(err)
	}
	report, err := app.FileManifest()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range report.Files {
		if entry.ID == "runtime-owner" || entry.ID == "workspace" {
			if entry.Location.Anchor != filepath.Join(root, "config") {
				t.Fatalf("relative anchor missing: %+v", entry)
			}
			checked++
		}
	}
	if checked != 2 {
		t.Fatal("anchored file roles missing")
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(secret)) {
		t.Fatal("file report exposed a credential value")
	}
	t.Chdir(t.TempDir())
	after, err := app.FileManifest()
	if err != nil {
		t.Fatal(err)
	}
	afterBytes, err := json.Marshal(after)
	if err != nil || !bytes.Equal(encoded, afterBytes) {
		t.Fatal("file report changed with the working directory")
	}
}
