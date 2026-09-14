package privatefiles

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

func TestPrivateEmptyRecordReadWhileExclusivelyLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	directory, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile("owner.lock", nil, ".empty-"); err != nil {
		t.Fatal(err)
	}
	owner := flock.New(filepath.Join(path, "owner.lock"))
	t.Cleanup(func() { _ = owner.Close() })
	if locked, err := owner.TryLock(); err != nil || !locked {
		t.Fatalf("acquire owner lock: %v, %v", locked, err)
	}
	for _, limit := range []int64{0, 1, 4096} {
		data, err := directory.ReadFile("owner.lock", limit)
		if err != nil || data == nil || len(data) != 0 {
			t.Fatalf("read empty locked file with limit %d: %v, %v", limit, data, err)
		}
	}
	competing := flock.New(filepath.Join(path, "owner.lock"))
	t.Cleanup(func() { _ = competing.Close() })
	if locked, err := competing.TryLock(); err != nil || locked {
		t.Fatalf("reader changed writer exclusion: %v, %v", locked, err)
	}
}

func TestPrivateEmptyRecordReadRejectsGrowth(t *testing.T) {
	directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile("empty", nil, ".empty-"); err != nil {
		t.Fatal(err)
	}
	root, err := directory.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	calls := 0
	inspect := func(root *os.Root, name string) (fs.FileInfo, error) {
		calls++
		if calls == 2 {
			if err := root.WriteFile(name, []byte("changed"), FileMode); err != nil {
				return nil, err
			}
		}
		return recordInfo(root, name)
	}
	if _, err := readCheckedFile(root, "empty", 4096, inspect); err == nil {
		t.Fatal("empty read accepted content added during validation")
	}
	if calls != 2 {
		t.Fatalf("read omitted final validation: %d inspections", calls)
	}
}
