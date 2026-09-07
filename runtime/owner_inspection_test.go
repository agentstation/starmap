package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerInspectionPreservesActiveRuntime(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	connected := openTestRuntime(t, WithStateDirectory(directory))
	owner := DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "default"}
	recordPath := filepath.Join(directory, ownerRecordName)
	before, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	seedPath := filepath.Join(directory, instanceSeedFileName)
	seed, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	status, err := InspectDirectoryOwnerRecord(t.Context(), directory, owner, "")
	want := OwnerRecordMatches
	if !ownerRecordInspectionSupported {
		want = OwnerRecordUnverified
	}
	if err != nil || status != want {
		t.Fatalf("inspection = %q, %v; want %q", status, err, want)
	}
	after, err := os.ReadFile(recordPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("inspection changed the owner record")
	}
	afterSeed, err := os.ReadFile(seedPath)
	if err != nil || !bytes.Equal(seed, afterSeed) {
		t.Fatal("inspection changed the instance seed")
	}
	afterEntries, err := os.ReadDir(directory)
	if err != nil || len(entries) != len(afterEntries) {
		t.Fatal("inspection changed the directory inventory")
	}
	for index := range entries {
		if entries[index].Name() != afterEntries[index].Name() {
			t.Fatal("inspection replaced a directory entry")
		}
	}
	other, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false))
	if other != nil {
		_ = other.Close()
	}
	if err == nil {
		t.Fatal("inspection released the active runtime lock")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerInspectionBindingAndInvalidRecords(t *testing.T) {
	t.Parallel()
	owner := DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "default"}
	canonical, err := encodeOwnerRecord(owner, "configured-identity")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		raw      []byte
		owner    DirectoryOwner
		identity string
		want     OwnerRecordStatus
	}{
		{"matching", canonical, owner, "configured-identity", OwnerRecordMatches},
		{"product", canonical, DirectoryOwner{Product: "starport", Deployment: "local", Instance: "default"}, "configured-identity", OwnerRecordConflict},
		{"deployment", canonical, DirectoryOwner{Product: "starmap", Deployment: "team", Instance: "default"}, "configured-identity", OwnerRecordConflict},
		{"instance", canonical, DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "other"}, "configured-identity", OwnerRecordConflict},
		{"identity", canonical, owner, "other", OwnerRecordConflict},
		{"malformed", []byte("invalid"), owner, "", OwnerRecordConflict},
		{"oversized", bytes.Repeat([]byte("x"), ownerRecordMaxBytes+1), owner, "", OwnerRecordConflict},
		{"absent", nil, owner, "", OwnerRecordAbsent},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			file := filepath.Join(directory, ownerRecordName)
			if test.raw != nil {
				if err := os.WriteFile(file, test.raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			status, err := InspectDirectoryOwnerRecord(t.Context(), directory, test.owner, test.identity)
			want := test.want
			if !ownerRecordInspectionSupported {
				want = OwnerRecordUnverified
			}
			if err != nil || status != want {
				t.Fatalf("inspection = %q, %v; want %q", status, err, want)
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			if test.raw == nil {
				if len(entries) != 0 {
					t.Fatal("inspection created files")
				}
			} else {
				after, err := os.ReadFile(file)
				if err != nil || !bytes.Equal(test.raw, after) || len(entries) != 1 {
					t.Fatal("inspection changed existing files")
				}
			}
		})
	}
}

func TestOwnerInspectionRefusesLinksAndMissingDirectory(t *testing.T) {
	t.Parallel()
	owner := DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "default"}
	directory := filepath.Join(t.TempDir(), "absent")
	status, err := InspectDirectoryOwnerRecord(t.Context(), directory, owner, "")
	want := OwnerRecordAbsent
	if !ownerRecordInspectionSupported {
		want = OwnerRecordUnverified
	}
	if err != nil || status != want {
		t.Fatalf("missing directory = %q, %v", status, err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("inspection created a missing directory")
	}
	directory = t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(directory, ownerRecordName)); err != nil {
		t.Skipf("native symbolic links unavailable: %v", err)
	}
	status, err = InspectDirectoryOwnerRecord(t.Context(), directory, owner, "")
	want = OwnerRecordConflict
	if !ownerRecordInspectionSupported {
		want = OwnerRecordUnverified
	}
	if err != nil || status != want {
		t.Fatalf("linked record = %q, %v", status, err)
	}
}

func TestOwnerInspectionValidatesBeforeFilesystemAccess(t *testing.T) {
	t.Parallel()
	owner := DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "default"}
	directory := filepath.Join(t.TempDir(), "absent")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := InspectDirectoryOwnerRecord(ctx, directory, owner, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled inspection = %v", err)
	}
	if _, err := InspectDirectoryOwnerRecord(t.Context(), "relative", owner, ""); err == nil {
		t.Fatal("relative directory accepted")
	}
	if _, err := InspectDirectoryOwnerRecord(t.Context(), directory, DirectoryOwner{}, ""); err == nil {
		t.Fatal("invalid owner accepted")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("invalid inspection created files")
	}
}
