package workspace

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsJournalReplacementRecoversAfterEditorHandleCloses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = windows.CloseHandle(handle)
		}
	})
	catalog, identity := testCatalog(t, "new", "New Model")
	receipt, err := Project(t.Context(), path, catalog, identity)
	if err == nil || receipt.GenerationID != "" {
		t.Fatalf("replacement ignored an editor handle without delete sharing: receipt=%+v, error=%v", receipt, err)
	}
	assertWorkspaceModel(t, path, "old", "Old Model")
	_, err = ObserveInput(path)
	assertReadConflict(t, err)
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	closed = true
	result, err := Repair(t.Context(), path, catalog, identity)
	if err != nil || result.Status != RepairStatusRepaired {
		t.Fatalf("repair after editor close: %+v, %v", result, err)
	}
	assertWorkspaceModel(t, path, "new", "New Model")
	assertReplacementFinished(t, path)
}
