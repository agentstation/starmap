//go:build darwin || linux

package app

import (
	"encoding/json"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
	"github.com/agentstation/starmap/runtime"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceConfigurationExplicitSelection(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	t.Setenv("STARMAP_CONFIG_ACCESS", "service-managed")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("catalog_source: embedded\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(path)
	if err != nil {
		t.Fatalf("explicit service configuration: %v", err)
	}
	if config.CatalogValues["STARMAP_CATALOG_SOURCE"] != "embedded" {
		t.Fatalf("configuration was not parsed: %v", config.CatalogValues)
	}
}

func TestServiceConfigurationSelectionAndPrivateBoundaries(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	t.Setenv("CONFIG", "")
	t.Setenv("STARMAP_CONFIG_ACCESS", "service-managed")
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte("# service-private-sentinel\ncatalog_source: embedded\n")
	if err := os.WriteFile(path, contents, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(""); err == nil {
		t.Error("service policy accepted an implicit path")
	}
	if _, err := readPrivateInput(path, "dotenv"); err == nil {
		t.Error("service policy widened dotenv access")
	}
	t.Setenv("STARMAP_CONFIG_ACCESS", "owner-only")
	if _, err := loadConfig(path); err == nil {
		t.Error("owner-only mode accepted shared input")
	}
	t.Setenv("STARMAP_CONFIG_ACCESS", "service-managed")
	for _, mode := range []os.FileMode{0o660, 0o642} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		if _, err := loadConfig(path); err == nil {
			t.Errorf("accepted writable mode %o", mode)
		} else if strings.Contains(err.Error(), "service-private-sentinel") {
			t.Fatal("error disclosed contents")
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatal("reader changed permissions")
		}
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG", path)
	config, err := loadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	app, err := New("test", "", "", "", WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := app.InspectFiles(t.Context(), 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range manifest.Files {
		if item.ID == "configuration" {
			found = true
			if item.Policy.Access != policy.ServiceManaged {
				t.Fatalf("wrong reported policy: %+v", item.Policy)
			}
		}
		if item.ID == "catalog-store" && item.Policy.Access != policy.OwnerOnly {
			t.Fatal("service policy widened catalog state")
		}
	}
	if !found {
		t.Fatal("configuration omitted from manifest")
	}
	for _, item := range manifest.Inspection.Observations {
		if item.ID == "configuration" && item.AccessStatus == "conflict" {
			t.Fatalf("inspection rejected accepted mode: %+v", item)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(contents) {
		t.Fatal("reader changed contents")
	}
}

func TestServiceConfigurationFlagPrecedenceAndTargetChecks(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	t.Setenv("STARMAP_CONFIG_ACCESS", "owner-only")
	dir := t.TempDir()
	path, link := filepath.Join(dir, "target.yaml"), filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("catalog_source: embedded\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	app := NewForCommand("test", "", "", "")
	if err := app.Execute(t.Context(), []string{"--config", link, "--config-access", "service-managed", "config", "paths", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if app.config.ConfigAccess != policy.ServiceManaged {
		t.Fatal("flag did not override environment")
	}
	if _, err := readSelectedConfiguration(link, policy.ServiceManaged); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := readSelectedConfiguration(link, policy.ServiceManaged); err == nil {
		t.Error("service file bypassed ancestor protection")
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := readSelectedConfiguration(link, policy.ServiceManaged); err == nil || os.IsNotExist(err) {
		t.Fatalf("dangling target became optional absence: %v", err)
	}
}

func TestServiceConfigurationMigrationRereadPreservesPolicyAndDigest(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	t.Setenv("STARMAP_CONFIG_ACCESS", "service-managed")
	target, path := filepath.Join(home, "target"), filepath.Join(home, "selected.yaml")
	contents, err := json.Marshal(map[string]string{"state_dir": target, "scheduler_identity": "service-identity", "catalog_source": "embedded"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	app, err := New("test", "", "", "", WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := app.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	request := runtime.DirectoryMigrationRequest{TargetDirectory: target, SourceIdentity: "service-identity", Owner: runtime.DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "default"}}
	if err := app.verifySavedMigrationSelection(request, paths); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(contents, '\n'), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := app.verifySavedMigrationSelection(request, paths); err == nil {
		t.Fatal("changed service configuration bypassed migration digest check")
	}
}
