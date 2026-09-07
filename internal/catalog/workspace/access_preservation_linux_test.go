package workspace

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestWorkspaceReplacementPreservesNativeACLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	models := filepath.Join(path, "providers", "test-provider", "models")
	data := make([]byte, 4+5*8)
	binary.LittleEndian.PutUint32(data, 2)
	for index, entry := range [][3]uint32{{1, 7, 0xffffffff}, {2, 4, 65534}, {4, 5, 0xffffffff}, {16, 5, 0xffffffff}, {32, 5, 0xffffffff}} {
		offset := 4 + index*8
		binary.LittleEndian.PutUint16(data[offset:], uint16(entry[0]))
		binary.LittleEndian.PutUint16(data[offset+2:], uint16(entry[1]))
		binary.LittleEndian.PutUint32(data[offset+4:], entry[2])
	}
	if err := unix.Setxattr(models, "system.posix_acl_default", data, 0); err != nil {
		t.Fatal(err)
	}
	if err := unix.Setxattr(path, "system.posix_acl_access", data, 0); err != nil {
		t.Fatal(err)
	}
	reference := filepath.Join(models, "acl-reference")
	if err := os.WriteFile(reference, nil, fileMode); err != nil {
		t.Fatal(err)
	}
	wantNew := readWorkspaceACL(t, reference)
	if err := os.Remove(reference); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	next, identity := testCatalog(t, "new", "New")
	if _, err := (projector{journalReplacement: true}).project(t.Context(), path, next, identity, InputExpectation{}); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	oldAccess := make(map[string]string)
	for _, entry := range before.Entries {
		oldAccess[entry.Path] = entry.AccessSHA256
	}
	for _, entry := range after.Entries {
		if want, ok := oldAccess[entry.Path]; ok && want != entry.AccessSHA256 {
			t.Errorf("%s access changed", entry.Path)
		}
	}
	if !bytes.Equal(wantNew, readWorkspaceACL(t, filepath.Join(models, "new.yaml"))) {
		t.Fatal("new model did not inherit the original default ACL")
	}
	assertReplacementFinished(t, path)
}

func readWorkspaceACL(t *testing.T, path string) []byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	acl, err := nativeEntryACL(file)
	if err != nil {
		t.Fatal(err)
	}
	return acl
}
