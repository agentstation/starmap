//go:build darwin || linux

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurationPrivateAccessBeforeParsing(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	file := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte("remote_server_api_key: private-access-sentinel\n")
	if err := os.WriteFile(file, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(file); err == nil {
		t.Error("configuration accepted an exposed credential file")
	} else if strings.Contains(err.Error(), "private-access-sentinel") {
		t.Fatal("configuration error disclosed a credential")
	} else if !strings.Contains(err.Error(), file) {
		t.Fatal("configuration error omitted the selected path")
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatal("configuration changed existing permissions")
	}
	got, err := os.ReadFile(file)
	if err != nil || string(got) != string(contents) {
		t.Fatal("configuration changed existing bytes")
	}
	if err := os.Chmod(file, 0o400); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(file); err != nil {
		t.Fatalf("read-only private configuration: %v", err)
	}
}

func TestDotenvAccessFailurePreservesEnvironment(t *testing.T) {
	t.Setenv("CSP_PRIVATE_ENV", "")
	if err := os.Unsetenv("CSP_PRIVATE_ENV"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first.env"), filepath.Join(dir, "second.env")
	for _, file := range []string{first, second} {
		if err := os.WriteFile(file, []byte("CSP_PRIVATE_ENV=private-access-sentinel\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(second, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadExplicitEnvFiles([]string{first, second}); err == nil {
		t.Error("dotenv accepted an exposed credential file")
	} else if strings.Contains(err.Error(), "private-access-sentinel") {
		t.Fatal("dotenv error disclosed a credential")
	}
	if _, present := os.LookupEnv("CSP_PRIVATE_ENV"); present {
		t.Fatal("failed dotenv load changed the environment")
	}
}

func TestConfigurationSymlinkChecksSelectedTarget(t *testing.T) {
	dir := t.TempDir()
	target, link := filepath.Join(dir, "target.yaml"), filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(target, []byte("catalog_source: embedded\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfigurationFile(link); err != nil {
		t.Fatalf("private symlink target: %v", err)
	}
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfigurationFile(link); err == nil {
		t.Fatal("symlink bypassed configuration access checks")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfigurationFile(link); err == nil || os.IsNotExist(err) {
		t.Fatalf("dangling selection became an absent optional configuration: %v", err)
	}
}
