//go:build darwin || linux

package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoveryStateRequiresPrivateAccess(t *testing.T) {
	config := Config{StateDirectory: t.TempDir(), Repository: "owner/catalog", Channel: "catalog"}
	store, err := newStateStore(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.save(State{Repository: config.Repository, Channel: config.Channel, Sequence: 7}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Dir(store.path), store.path} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Errorf("discovery state has nonprivate permissions: %s, %v", path, err)
		}
	}
}

func TestDiscoveryStateRefusesExposedExistingDirectory(t *testing.T) {
	config := Config{StateDirectory: t.TempDir(), Repository: "owner/catalog", Channel: "catalog"}
	directory := filepath.Join(config.StateDirectory, stateDirectoryName)
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := newStateStore(config); err == nil {
		t.Fatal("exposed discovery directory accepted")
	}
}

func TestDiscoveryStateRefusesLinkedDirectory(t *testing.T) {
	config := Config{StateDirectory: t.TempDir(), Repository: "owner/catalog", Channel: "catalog"}
	if err := os.Symlink(t.TempDir(), filepath.Join(config.StateDirectory, stateDirectoryName)); err != nil {
		t.Fatal(err)
	}
	if _, err := newStateStore(config); err == nil {
		t.Fatal("linked discovery directory accepted")
	}
}
