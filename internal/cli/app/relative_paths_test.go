package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/runtime"
)

func TestRelativeRuntimeRefusesUnknownLegacyAnchorBeforeWrites(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv(relativePathBaseName, "")
	oldWorkingDirectory := t.TempDir()
	oldPath := filepath.Join(oldWorkingDirectory, "runtime")
	old, err := runtime.Open(t.Context(), runtime.WithStateDirectory(oldPath), runtime.WithCatalogSource("embedded"), runtime.WithSourcePollInterval(0), runtime.WithAcquisitionEnabled(false))
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	seedPath := filepath.Join(oldPath, "instance-seed")
	seed, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	t.Setenv(catalogconfig.StateDirectory, "runtime")
	// A different working directory cannot reveal the old relative anchor.
	t.Chdir(t.TempDir())
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}}))
	if err != nil {
		t.Fatal(err)
	}
	connected, err := a.Runtime(t.Context())
	if connected != nil {
		if closeErr := a.closeRuntime(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if err == nil {
		t.Error("relative runtime selection abandoned existing state under an unknown old anchor")
	}
	entries, readErr := os.ReadDir(home)
	if readErr != nil || len(entries) != 0 {
		t.Error("ambiguous relative selection wrote new product files")
	}
	after, err := os.ReadFile(seedPath)
	if err != nil || !bytes.Equal(seed, after) {
		t.Fatal("relative selection changed the old runtime seed")
	}
}

func TestRelativePrimaryFileRefusesAmbiguousSelection(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv(relativePathBaseName, "")
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	configRoot := filepath.Join(home, "config")
	if err := os.Mkdir(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	oldWorkingDirectory := t.TempDir()
	for directory, contents := range map[string]string{oldWorkingDirectory: "catalog_source: embedded\n", configRoot: "catalog_source: github\n"} {
		if err := os.WriteFile(filepath.Join(directory, "selected.yaml"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(oldWorkingDirectory)
	if _, err := loadConfig("selected.yaml"); err == nil {
		t.Fatal("relative primary selection silently chose a different configuration file")
	}
}

func TestRelativeLeafSelectionRequiresExplicitIntent(t *testing.T) {
	for _, role := range []string{"runtime", "workspace", "legacy-workspace"} {
		t.Run(role, func(t *testing.T) {
			clearCatalogEnvironment(t)
			t.Setenv(relativePathBaseName, "")
			home := t.TempDir()
			t.Setenv("STARMAP_HOME", home)
			config := &Config{CatalogValues: map[string]string{catalogconfig.Source: "embedded"}}
			switch role {
			case "runtime":
				config.CatalogValues[catalogconfig.StateDirectory] = "selected"
			case "workspace":
				config.CatalogValues[catalogconfig.WorkspacePath] = "selected"
			case "legacy-workspace":
				config.CatalogPath = "selected"
			}
			a, err := New("test", "test", "test", "test", WithConfig(config))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.ResolvedPaths(); err == nil || !strings.Contains(err.Error(), "absolute path") {
				t.Fatalf("ambiguous %s has no migration refusal: %v", role, err)
			}
			t.Setenv(relativePathBaseName, "config")
			paths, err := a.ResolvedPaths()
			if err != nil {
				t.Fatal(err)
			}
			selected := paths.Workspace
			if role == "runtime" {
				selected = paths.Runtime
			}
			if selected.Path != filepath.Join(home, "config", "selected") || selected.Anchor != filepath.Join(home, "config") {
				t.Fatalf("explicit relative intent lost its anchor: %+v", selected)
			}
		})
	}
}

func TestRelativePrimaryIntentPrecedesFileSelection(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv(relativePathBaseName, "")
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	root := filepath.Join(home, "config")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "selected.yaml")
	if err := os.WriteFile(file, []byte("relative_path_base: config\ncatalog_source: embedded\nstate_dir: runtime\ncatalog_workspace_path: workspace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig("selected.yaml"); err == nil {
		t.Fatal("file contents authorized selection of their own ambiguous relative path")
	}
	for range 2 {
		t.Chdir(t.TempDir())
		a := NewForCommand("test", "test", "test", "test")
		command := a.createRootCommand()
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs([]string{"config", "paths", "--config", "selected.yaml", "--relative-path-base", "config", "--output", "json"})
		if err := command.ExecuteContext(t.Context()); err != nil {
			t.Fatal(err)
		}
		var report productpaths.FileManifest
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.RelativePathBase != "config" || report.RelativePathBaseOrigin != "options" {
			t.Fatalf("report lost explicit path intent: %+v", report)
		}
		for _, entry := range report.Files {
			if entry.ID == "configuration" && entry.Location.Path != file || entry.ID == "runtime-owner" && entry.Location.Path != filepath.Join(root, "runtime", "owner.json") {
				t.Fatalf("working directory changed selection: %+v", entry)
			}
		}
		if a.runtime != nil || a.starmap != nil {
			t.Fatal("relative path inspection opened a catalog")
		}
	}
}

func TestRelativePathDeclarationPrecedenceAndValidation(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv(relativePathBaseName, "")
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("relative_path_base: config\ncatalog_source: embedded\nstate_dir: runtime\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New("test", "test", "test", "test", WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResolvedPaths(); err == nil {
		t.Fatal("empty environment declaration did not override the file")
	}
	if err := os.Unsetenv(relativePathBaseName); err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil || paths.RelativePathBase != "config" || paths.RelativePathBaseOrigin != "configuration-file" {
		t.Fatalf("file declaration did not select the anchor: %+v %v", paths, err)
	}
	t.Setenv(relativePathBaseName, "private-invalid-sentinel")
	if _, err := a.ResolvedPaths(); err == nil || strings.Contains(err.Error(), "private-invalid-sentinel") {
		t.Fatalf("invalid path declaration was accepted or exposed: %v", err)
	}
}

func TestRelativeUpdatePathsRefuseBeforeCatalogAccess(t *testing.T) {
	for _, selection := range []string{"catalog-path", "sources-dir", "environment-sources"} {
		t.Run(selection, func(t *testing.T) {
			clearCatalogEnvironment(t)
			t.Setenv(relativePathBaseName, "")
			t.Setenv("STARMAP_SOURCES_DIR", "")
			home := t.TempDir()
			t.Setenv("STARMAP_HOME", home)
			a := NewForCommand("test", "test", "test", "test")
			command := a.createRootCommand()
			args := []string{"update", "--source", "local", "--dry-run"}
			if selection == "environment-sources" {
				t.Setenv("STARMAP_SOURCES_DIR", "old-sources")
			} else {
				args = append(args, "--"+selection, "old-selection")
			}
			command.SetArgs(args)
			if err := command.ExecuteContext(t.Context()); err == nil || !strings.Contains(err.Error(), "legacy relative anchor") {
				t.Fatalf("update has no path migration refusal: %v", err)
			}
			entries, err := os.ReadDir(home)
			if err != nil || len(entries) != 0 || a.runtime != nil || a.starmap != nil || a.credentialResolver != nil {
				t.Fatal("ambiguous update path initialized application state")
			}
		})
	}
}

func TestAbsoluteRuntimeSelectionPreservesExistingIdentity(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv(relativePathBaseName, "")
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	directory := filepath.Join(t.TempDir(), "selected-runtime")
	t.Setenv(catalogconfig.StateDirectory, directory)
	var identity string
	for range 2 {
		t.Chdir(t.TempDir())
		a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}}))
		if err != nil {
			t.Fatal(err)
		}
		connected, err := a.Runtime(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		actual := connected.Status().InstanceIdentity
		if err := a.closeRuntime(); err != nil {
			t.Fatal(err)
		}
		if actual == "" || identity != "" && actual != identity {
			t.Fatal("absolute runtime selection changed identity")
		}
		identity = actual
	}
	if _, err := os.Stat(filepath.Join(directory, "instance-seed")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "config", "selected-runtime")); !os.IsNotExist(err) {
		t.Fatal("absolute selection created an anchored replacement")
	}
}

func TestOperationPathsRetainSelectedAnchor(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	t.Setenv(relativePathBaseName, "config")
	a, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		t.Chdir(t.TempDir())
		for value, want := range map[string]string{"sources": filepath.Join(home, "config", "sources"), home: home} {
			got, err := a.ResolveOperationPath(value, "sources-dir")
			if err != nil || got != want {
				t.Fatalf("operation path changed anchor: got %q want %q, %v", got, want, err)
			}
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatal("operation path resolution created files")
	}
}

func TestDotenvDeclaresRelativePrimarySelection(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv(relativePathBaseName, "")
	if err := os.Unsetenv(relativePathBaseName); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	root := filepath.Join(home, "config")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "selected.yaml"), []byte("catalog_source: embedded\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(t.TempDir(), "paths.env")
	if err := os.WriteFile(envFile, []byte("STARMAP_RELATIVE_PATH_BASE=config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := NewForCommand("test", "test", "test", "test")
	command := a.createRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"config", "paths", "--config", "selected.yaml", "--env-file", envFile, "--output", "json"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	var report productpaths.FileManifest
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.RelativePathBase != "config" || report.RelativePathBaseOrigin != "dotenv:"+envFile {
		t.Fatal("dotenv declaration lost its origin")
	}
	if _, present := os.LookupEnv(relativePathBaseName); present {
		t.Fatal("dotenv declaration escaped into process environment")
	}
}

func TestRelativeFileSourceRefusesBeforeBaselineWrites(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv(relativePathBaseName, "")
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.Source: "file", catalogconfig.SourceURL: "catalog.json", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}}))
	if err != nil {
		t.Fatal(err)
	}
	connected, err := a.Runtime(t.Context())
	if connected != nil {
		if closeErr := a.closeRuntime(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if err == nil || !strings.Contains(err.Error(), "legacy relative anchor") {
		t.Errorf("relative file source has no anchor refusal: %v", err)
	}
	entries, readErr := os.ReadDir(home)
	if readErr != nil || len(entries) != 0 {
		t.Fatal("ambiguous file source created baseline or runtime files")
	}
}

func TestFileSourceReadsReportedAnchorAcrossWorkingDirectories(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv(relativePathBaseName, "config")
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	root := filepath.Join(home, "config")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	generation, err := starmap.EmbeddedGeneration()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "catalog.json")
	if err := os.WriteFile(file, generation.Payload, 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.Source: "file", catalogconfig.SourceURL: "catalog.json", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}}))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := a.FileManifest()
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, entry := range manifest.Files {
		if entry.ID == "source-file" {
			matched = entry.Location.Path == file && entry.Location.Anchor == root
		}
	}
	if !matched {
		t.Fatal("manifest omits the selected file source and anchor")
	}
	for range 2 {
		other := t.TempDir()
		if err := os.WriteFile(filepath.Join(other, "catalog.json"), []byte("invalid-catalog"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(other)
		connected, err := a.Runtime(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		report, err := connected.RefreshSource(t.Context())
		if err != nil || report.Health != runtime.HealthOK {
			t.Fatalf("source did not read the reported anchor: %+v %v", report, err)
		}
	}
	if err := a.closeRuntime(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(generation.Payload, after) {
		t.Fatal("source refresh changed operator-owned input")
	}
}
