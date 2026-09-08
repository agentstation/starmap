//go:build darwin || linux

package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeRefusesExposedDirectoryBeforeWrites(t *testing.T) {
	t.Parallel()
	for _, mode := range []os.FileMode{0o750, 0o705, 0o777} {
		t.Run(mode.String(), func(t *testing.T) {
			directory := privateRuntimeDirectory(t)
			if err := os.Chmod(directory, mode); err != nil {
				t.Fatal(err)
			}
			connected, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false))
			if connected != nil {
				_ = connected.Close()
			}
			if err == nil {
				t.Error("runtime accepted group or public directory permissions")
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Error("refusal wrote runtime files")
			}
			info, err := os.Stat(directory)
			if err != nil || info.Mode().Perm() != mode {
				t.Error("refusal changed operator permissions")
			}
		})
	}
}

func TestRuntimeRefusesExposedLockBeforeWrites(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	file := filepath.Join(directory, directoryLockName)
	if err := os.WriteFile(file, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	connected, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false))
	if connected != nil {
		_ = connected.Close()
	}
	if err == nil {
		t.Error("runtime accepted a publicly readable lock file")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Error("refusal wrote runtime files")
	}
	after, err := os.ReadFile(file)
	if err != nil || string(after) != "preserve" {
		t.Error("refusal changed lock bytes")
	}
}

func TestRuntimeRefusesExposedIdentityFilesAndRecovers(t *testing.T) {
	t.Parallel()
	for _, name := range []string{ownerRecordName, instanceSeedFileName} {
		t.Run(name, func(t *testing.T) {
			directory := privateRuntimeDirectory(t)
			first := openTestRuntime(t, WithStateDirectory(directory))
			if err := first.Close(); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(directory, name)
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(file, 0o640); err != nil {
				t.Fatal(err)
			}
			connected, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false))
			if connected != nil {
				_ = connected.Close()
			}
			if err == nil {
				t.Fatal("runtime accepted exposed identity permissions")
			}
			after, err := os.ReadFile(file)
			if err != nil || string(before) != string(after) {
				t.Fatal("refusal changed identity bytes")
			}
			if err := os.Chmod(file, 0o600); err != nil {
				t.Fatal(err)
			}
			_ = openTestRuntime(t, WithStateDirectory(directory))
		})
	}
}
