package workspace

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorkspaceReplacementPreservesNativeACLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".", "providers.yaml", "providers/test-provider/models"} {
		addWorkspaceTestACL(t, filepath.Join(path, name))
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
	assertReplacementFinished(t, path)
}

func TestWorkspaceNewFilesInheritNativeACLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	models := filepath.Join(path, "providers", "test-provider", "models")
	data, err := exec.CommandContext(t.Context(), "/bin/chmod", "+a", "everyone allow readattr,file_inherit,directory_inherit", models).CombinedOutput()
	if err != nil {
		t.Fatalf("set inheritance: %v: %s", err, data)
	}
	reference := filepath.Join(models, "acl-reference")
	if err := os.WriteFile(reference, nil, fileMode); err != nil {
		t.Fatal(err)
	}
	want := readWorkspaceACL(t, reference)
	if err := os.Remove(reference); err != nil {
		t.Fatal(err)
	}
	next, identity := testCatalog(t, "new", "New")
	if _, err := Project(t.Context(), path, next, identity); err != nil {
		t.Fatal(err)
	}
	got := readWorkspaceACL(t, filepath.Join(models, "new.yaml"))
	if len(want) == 0 || !bytes.Equal(want, got) {
		t.Fatal("new model did not inherit the original directory ACL")
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
