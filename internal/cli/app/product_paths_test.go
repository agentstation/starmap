package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/productpaths"
)

func TestProductHomeAndLeafOverridesUseOneAnchor(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	t.Setenv(relativePathBaseName, "config")
	t.Setenv("STARMAP_CATALOG_STORE_PATH", "stores/catalog")
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.WorkspacePath: "workspace", catalogconfig.StateDirectory: "runtime"}}))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	configRoot := filepath.Join(home, "config")
	if paths.CatalogStore.Path != filepath.Join(configRoot, "stores", "catalog") || paths.Runtime.Path != filepath.Join(configRoot, "runtime") || paths.Workspace.Path != filepath.Join(configRoot, "workspace") {
		t.Fatalf("incorrect anchored paths: %+v", paths)
	}
	t.Chdir(t.TempDir())
	after, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Runtime != after.Runtime || paths.Workspace != after.Workspace || paths.CatalogStore != after.CatalogStore {
		t.Fatal("path resolution changed with the working directory")
	}
	files, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatal("passive path reporting created files")
	}
}

func TestPrimaryFileCannotRelocateItsConfigurationRoot(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	configRoot := filepath.Join(home, "config")
	if err := os.Mkdir(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(t.TempDir(), "replacement")
	if err := os.WriteFile(filepath.Join(configRoot, "config.yaml"), []byte("config_dir: "+replacement+"\ndata_dir: "+filepath.Join(home, "custom-data")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := resolveProductPaths(config)
	if err != nil {
		t.Fatal(err)
	}
	if paths.Roots[productpaths.Config].Path != configRoot || paths.Configuration.Path != filepath.Join(configRoot, "config.yaml") {
		t.Fatal("configuration file relocated its own authority")
	}
	if paths.Roots[productpaths.Data].Path != filepath.Join(home, "custom-data") {
		t.Fatal("file data-root override did not apply")
	}
}

func TestMalformedOptionalPrimaryConfigurationFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	directory := filepath.Join(home, "config")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config.yaml"), []byte("invalid: [private-sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("malformed default configuration was ignored")
	}
}

func TestGoPathOptionsBypassAmbientConfiguration(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("HOME", "")
	t.Setenv("STARMAP_HOME", "invalid-relative")
	chosen := t.TempDir()
	a, err := New("test", "test", "test", "test", WithConfig(&Config{PathValues: map[string]string{"STARMAP_HOME": chosen}}))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Roots[productpaths.Data].Path != filepath.Join(chosen, "data") {
		t.Fatal("explicit Go paths did not override the environment")
	}
}

func nativeRoot(t *testing.T, root productpaths.Root) string {
	t.Helper()
	path, err := productpaths.UserDefaults(productpaths.Starmap)(root)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImplicitNewRootsRefuseLegacyStateWithoutWriting(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacy := filepath.Join(home, ".starmap", "state", "catalog")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(legacy, "current")
	if err := os.WriteFile(marker, []byte("old-generation"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Runtime(t.Context()); err == nil {
		t.Fatal("startup abandoned legacy state")
	}
	if _, err := os.Stat(paths.Baselines.Path); !os.IsNotExist(err) {
		t.Fatal("migration refusal created new baseline storage")
	}
	contents, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "old-generation" {
		t.Fatal("migration refusal changed legacy state")
	}
}

func TestCommandPathFlagsAndDotenvKeepOrigins(t *testing.T) {
	clearCatalogEnvironment(t)
	for _, setting := range pathSettings() {
		t.Setenv(setting.name, "")
		if err := os.Unsetenv(setting.name); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("STARMAP_HOME", "invalid-relative")
	directory := t.TempDir()
	chosen := filepath.Join(directory, "chosen")
	data := filepath.Join(directory, "data")
	file := filepath.Join(directory, "local.env")
	if err := os.WriteFile(file, []byte("STARMAP_DATA_DIR="+data+"\nSTARMAP_CATALOG_STORE_PATH=stores/catalog\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := NewForCommand("test", "test", "test", "test")
	if err := a.Execute(t.Context(), []string{"--env-file", file, "--home", chosen, "version"}); err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Roots[productpaths.Config].Path != filepath.Join(chosen, "config") || paths.Roots[productpaths.Data].Path != data || paths.Roots[productpaths.Data].Origin != "dotenv:"+file {
		t.Fatal("node path selection lost its authority")
	}
	if paths.CatalogStore.Path != filepath.Join(chosen, "config", "stores", "catalog") {
		t.Fatal("dotenv leaf path lost the configuration anchor")
	}
	if _, present := os.LookupEnv("STARMAP_DATA_DIR"); present {
		t.Fatal("dotenv root escaped into process authority")
	}
}

func TestFrozenConfigurationRootDoesNotResolveAgain(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	frozen := config.configRoot
	t.Setenv("STARMAP_HOME", "invalid-relative")
	for _, name := range []string{"STARMAP_DATA_DIR", "STARMAP_STATE_ROOT", "STARMAP_CACHE_DIR"} {
		t.Setenv(name, t.TempDir())
	}
	paths, err := resolveProductPaths(config)
	if err != nil {
		t.Fatal(err)
	}
	if paths.Roots[productpaths.Config] != frozen {
		t.Fatal("configuration root changed after file selection")
	}
}

func TestGoPathOptionsRejectUnknownNamesWithoutValues(t *testing.T) {
	clearCatalogEnvironment(t)
	config := &Config{PathValues: map[string]string{"STARMAP_HOME": t.TempDir(), "STARMAP_DAT_DIR": "private-path-sentinel"}}
	_, err := resolveProductPaths(config)
	if err == nil {
		t.Fatal("unknown node path setting was ignored")
	}
	if strings.Contains(err.Error(), "private-path-sentinel") {
		t.Fatal("path validation exposed an input value")
	}
}

func TestApplicationBindsConfiguredRuntimeOwner(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	t.Setenv("STARMAP_DEPLOYMENT_ID", "startup-production")
	t.Setenv("STARMAP_INSTANCE_ID", "gateway-a")
	application, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}}))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := application.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.DeploymentID != "startup-production" || paths.InstanceID != "gateway-a" {
		t.Fatal("path report lost ownership identity")
	}
	if _, err := application.Runtime(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := application.closeRuntime(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(paths.Runtime.Path, "owner.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if record["product"] != "starmap" || record["deployment"] != paths.DeploymentID || record["instance"] != paths.InstanceID {
		t.Fatal("runtime owner differs from application configuration")
	}
	t.Setenv("STARMAP_DEPLOYMENT_ID", "different-deployment")
	if _, err := application.Runtime(t.Context()); err == nil {
		t.Fatal("application reused another deployment's runtime state")
	}
}

func TestSourceDirectoryReportUsesConfiguredCacheRoot(t *testing.T) {
	clearCatalogEnvironment(t)
	root := t.TempDir()
	t.Setenv("STARMAP_HOME", root)
	cache := filepath.Join(root, "selected-cache")
	t.Setenv("STARMAP_CACHE_DIR", cache)
	a, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	directories, err := a.SourceDirectories()
	if err != nil {
		t.Fatal(err)
	}
	if directories.Cache != cache || directories.Checkouts != filepath.Join(cache, "sources") || paths.SourceCache.Path != filepath.Join(cache, "models.dev") || paths.SourceCheckout.Path != filepath.Join(cache, "sources", "models.dev-git") {
		t.Fatalf("source report differs from composition: %+v, %+v", directories, paths)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("source path report created files")
	}
}
