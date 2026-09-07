//go:build darwin || linux

package app

import (
	"os"
	"path/filepath"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
)

func TestConfigurationSymlinkChecksIntermediateTargetAncestors(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "config.yaml")
	if err := os.WriteFile(target, []byte("log_level: debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	redirect := filepath.Join(base, "redirect")
	if err := os.Mkdir(redirect, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(redirect, "hop")); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "selected.yaml")
	if err := os.Symlink(filepath.Join(redirect, "hop"), alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(redirect, 0o777); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(redirect, 0o700) })
	if _, err := readConfigurationFile(alias); err == nil {
		t.Fatal("configuration accepted an unsafe intermediate target directory")
	}
}

func TestConfigurationSymlinkDoesNotEraseTraversalAncestors(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "config.yaml")
	if err := os.WriteFile(target, []byte("log_level: debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsafe := filepath.Join(base, "unsafe")
	if err := os.Mkdir(unsafe, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unsafe, 0o777); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unsafe, 0o700) })
	alias := filepath.Join(base, "selected.yaml")
	if err := os.Symlink(unsafe+"/../config.yaml", alias); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfigurationFile(alias); err == nil {
		t.Fatal("configuration normalized away an unsafe traversal directory")
	}
}

func TestConfigurationPreservesTrustedRelativeSymlinkTraversal(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	if err := os.Mkdir("directory", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("config.yaml", []byte("log_level: debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("directory/../config.yaml", "selected.yaml"); err != nil {
		t.Fatal(err)
	}
	data, err := readConfigurationFile("selected.yaml")
	if err != nil || string(data) != "log_level: debug\n" {
		t.Fatal("trusted relative traversal did not reach the selected file", err)
	}
	if err := os.Symlink("cycle.yaml", "cycle.yaml"); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfigurationFile("cycle.yaml"); err == nil {
		t.Fatal("configuration accepted a symlink cycle")
	}
}

func TestAncestorRefusalPrecedesConfigurationAndStartup(t *testing.T) {
	clearCatalogEnvironment(t)
	ancestor := t.TempDir()
	home := filepath.Join(ancestor, "product")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(config, []byte("log_level: debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STARMAP_HOME", home)
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{
		catalogconfig.Source: "embedded", catalogconfig.AcquisitionEnabled: "false", catalogconfig.SourcePollInterval: "0s",
	}}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ancestor, 0o777); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ancestor, 0o700) })
	if _, err := readConfigurationFile(config); err == nil {
		t.Fatal("configuration read accepted an unsafe ancestor")
	}
	if _, err := a.Runtime(t.Context()); err == nil {
		_ = a.closeRuntime()
		t.Fatal("startup accepted an unsafe ancestor")
	}
	for _, name := range []string{"data", "state", "cache"} {
		if _, err := os.Stat(filepath.Join(home, name)); !os.IsNotExist(err) {
			t.Fatalf("refused startup created %s", name)
		}
	}
	if _, err := a.InspectFiles(t.Context(), 10000); err != nil {
		t.Fatal("diagnostics failed after ancestor refusal", err)
	}
	if err := os.Chmod(ancestor, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfigurationFile(config); err != nil {
		t.Fatal("explicit ancestor correction did not restore reads", err)
	}
	if _, err := a.Runtime(t.Context()); err != nil {
		t.Fatal("explicit ancestor correction did not restore startup", err)
	}
	if err := a.closeRuntime(); err != nil {
		t.Fatal(err)
	}
}
