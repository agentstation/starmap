package app

import (
	"bytes"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
)

func TestSeparateApplicationInstancesRetainDistinctSeeds(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	paths := map[string]string{"STARMAP_HOME": home, "STARMAP_INSTANCE_ID": "one"}
	values := map[string]string{catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}
	first, err := New("test", "test", "test", "test", WithConfig(&Config{PathValues: paths, CatalogValues: values}))
	if err != nil {
		t.Fatal(err)
	}
	defer first.closeRuntime()
	firstRuntime, err := first.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	firstPaths, err := first.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	seedPath := filepath.Join(firstPaths.Runtime.Path, "instance-seed")
	original, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New("test", "test", "test", "test", WithConfig(&Config{PathValues: map[string]string{"STARMAP_HOME": home, "STARMAP_INSTANCE_ID": "two"}, CatalogValues: values}))
	if err != nil {
		t.Fatal(err)
	}
	defer second.closeRuntime()
	secondRuntime, err := second.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	secondPaths, err := second.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if firstPaths.Runtime.Path == secondPaths.Runtime.Path || firstRuntime.Status().InstanceIdentity == secondRuntime.Status().InstanceIdentity {
		t.Fatal("separate instances reused runtime ownership")
	}
	secondSeed, err := os.ReadFile(filepath.Join(secondPaths.Runtime.Path, "instance-seed"))
	if err != nil {
		t.Fatal(err)
	}
	if len(original) == 0 || len(secondSeed) == 0 || bytes.Equal(original, secondSeed) {
		t.Fatal("separate instances reused a seed")
	}
	duplicate, err := New("test", "test", "test", "test", WithConfig(&Config{PathValues: paths, CatalogValues: values}))
	if err != nil {
		t.Fatal(err)
	}
	defer duplicate.closeRuntime()
	if _, err := duplicate.Runtime(t.Context()); err == nil {
		t.Fatal("duplicate local owner started")
	} else {
		var conflict *pkgerrors.ConflictError
		if !stderrors.As(err, &conflict) {
			t.Fatalf("duplicate owner error = %v, want ownership conflict", err)
		}
	}
	after, err := os.ReadFile(seedPath)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("duplicate owner changed the live seed")
	}
	t.Log("Two applications retain distinct runtime directories and identities; a duplicate cannot start or alter the seed.")
}

func TestApplicationRefusesEmptyRequiredPathsBeforeWrites(t *testing.T) {
	for _, field := range []string{"STARMAP_HOME", "STARMAP_CONFIG_DIR", "STARMAP_DATA_DIR", "STARMAP_STATE_ROOT", "STARMAP_CACHE_DIR", "STARMAP_CATALOG_STORE_PATH", catalogconfig.StateDirectory} {
		t.Run(field, func(t *testing.T) {
			clearCatalogEnvironment(t)
			home := t.TempDir()
			paths := map[string]string{"STARMAP_HOME": home}
			values := map[string]string{catalogconfig.Source: "embedded", catalogconfig.AcquisitionEnabled: "false"}
			if field == catalogconfig.StateDirectory {
				values[field] = ""
			} else {
				paths[field] = ""
			}
			application, err := New("test", "test", "test", "test", WithConfig(&Config{PathValues: paths, CatalogValues: values}))
			if err == nil {
				defer application.closeRuntime()
				_, err = application.Runtime(t.Context())
			}
			if err == nil {
				t.Fatal("empty required path started persistent runtime")
			}
			var invalid *pkgerrors.ValidationError
			if !stderrors.As(err, &invalid) {
				t.Fatalf("empty path error = %v, want validation error", err)
			}
			entries, err := os.ReadDir(home)
			if err != nil || len(entries) != 0 {
				t.Fatal("invalid path startup wrote state")
			}
		})
	}
}

func TestApplicationUsesExplicitDeploymentRootsWithoutHome(t *testing.T) {
	clearCatalogEnvironment(t)
	setTestHome(t, "")
	root := t.TempDir()
	selected := map[string]string{"STARMAP_CONFIG_DIR": filepath.Join(root, "etc", "starmap"), "STARMAP_DATA_DIR": filepath.Join(root, "data"), "STARMAP_STATE_ROOT": filepath.Join(root, "state"), "STARMAP_CACHE_DIR": filepath.Join(root, "cache")}
	application, err := New("test", "test", "test", "test", WithConfig(&Config{PathValues: selected, CatalogValues: map[string]string{catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}}))
	if err != nil {
		t.Fatal(err)
	}
	defer application.closeRuntime()
	paths, err := application.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[productpaths.Root]string{productpaths.Config: selected["STARMAP_CONFIG_DIR"], productpaths.Data: selected["STARMAP_DATA_DIR"], productpaths.State: selected["STARMAP_STATE_ROOT"], productpaths.Cache: selected["STARMAP_CACHE_DIR"]}
	for role, want := range expected {
		if paths.Roots[role].Path != want {
			t.Fatalf("%s root = %s, want %s", role, paths.Roots[role].Path, want)
		}
	}
	if paths.Runtime.Path != filepath.Join(selected["STARMAP_STATE_ROOT"], "catalog", "runtime", "default") || paths.Baselines.Path != filepath.Join(selected["STARMAP_DATA_DIR"], "catalog", "baseline") {
		t.Fatal("application roots used a home directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("path inspection wrote deployment state")
	}
	if _, err := application.Runtime(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.Runtime.Path, "instance-seed")); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(paths.Baselines.Path)
	if err != nil || len(entries) == 0 {
		t.Fatal("deployment did not persist a baseline")
	}
}
