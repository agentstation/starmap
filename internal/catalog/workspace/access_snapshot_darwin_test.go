package workspace

import (
	"encoding/binary"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestWorkspaceACLBufferRefusesMalformedNativeMetadata(t *testing.T) {
	valid := make([]byte, 12+workspaceDarwinSecurityHeader+workspaceDarwinACLEntryBytes)
	binary.LittleEndian.PutUint32(valid[:4], uint32(len(valid)))
	binary.LittleEndian.PutUint32(valid[4:8], 8)
	binary.LittleEndian.PutUint32(valid[8:12], uint32(len(valid)-12))
	binary.LittleEndian.PutUint32(valid[12:16], workspaceDarwinSecurityMagic)
	binary.LittleEndian.PutUint32(valid[48:52], 1)
	for _, test := range []struct {
		name   string
		change func([]byte) []byte
	}{
		{"short", func(b []byte) []byte { return b[:8] }},
		{"total", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[:4], uint32(len(b)+1)); return b }},
		{"offset", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[4:8], 0xffffffff); return b }},
		{"length", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[8:12], 0xffffffff); return b }},
		{"magic", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[12:16], 0); return b }},
		{"count", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[48:52], 2); return b }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := workspaceDarwinACLBytes(test.change(slices.Clone(valid))); err == nil {
				t.Fatal("accepted malformed native ACL metadata")
			}
		})
	}
	if _, err := workspaceDarwinACLBytes(valid); err != nil {
		t.Fatal("refused bounded ACL metadata", err)
	}
}

func addWorkspaceTestACL(t *testing.T, path string) {
	t.Helper()
	output, err := exec.CommandContext(t.Context(), "/bin/chmod", "+a", "everyone allow readattr", path).CombinedOutput()
	if err != nil {
		t.Fatalf("add native ACL: %v: %s", err, output)
	}
}

func TestTreeSnapshotDetectsNativeACLChange(t *testing.T) {
	for _, name := range []string{".", "note.txt"} {
		t.Run(name, func(t *testing.T) {
			path := t.TempDir()
			if err := os.WriteFile(filepath.Join(path, "note.txt"), []byte("note"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := snapshotTree(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			addWorkspaceTestACL(t, filepath.Join(path, name))
			after, err := snapshotTree(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if sameTree(before, after) {
				t.Fatal("snapshot omitted a native ACL change")
			}
		})
	}
}

func TestJournalRecoveryPreservesACLChangedBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	next, identity := testCatalog(t, "new", "New")
	fault := stderrors.New("stop after backup")
	_, err := (projector{journalReplacement: true, afterReplacementPhase: func(phase replacementPhase) error {
		if phase == replacementBackupMoved {
			return fault
		}
		return nil
	}}).project(t.Context(), path, next, identity, InputExpectation{})
	if !stderrors.Is(err, fault) {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	record, err := readReplacementRecord(root, path)
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(filepath.Dir(path), record.Backup)
	addWorkspaceTestACL(t, backup)
	_, err = Repair(t.Context(), path, next, identity)
	assertReadConflict(t, err)
	assertWorkspaceModel(t, backup, "old", "Old")
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("recovery installed a candidate after backup access changed", err)
	}
}
