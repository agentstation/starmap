package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

func TestWorkspaceEmptyLockedRecords(t *testing.T) {
	path := t.TempDir()
	name := ".commit.lock"
	if err := os.WriteFile(filepath.Join(path, name), nil, fileMode); err != nil {
		t.Fatal(err)
	}
	writer := flock.New(filepath.Join(path, name))
	t.Cleanup(func() { _ = writer.Close() })
	if locked, err := writer.TryLock(); err != nil || !locked {
		t.Fatalf("acquire writer: %v, %v", locked, err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	empty := sha256.Sum256(nil)
	want := hex.EncodeToString(empty[:])
	for _, limit := range []int64{0, 1, replacementJournalMax} {
		data, record, err := readWorkspaceRecord(root, name, limit)
		if err != nil || data == nil || len(data) != 0 || record.entry.Size != 0 || record.entry.SHA256 != want {
			t.Fatalf("read locked empty record with limit %d: %q, %+v, %v", limit, data, record, err)
		}
		if record.identity == "" || record.entry.AccessSHA256 == "" {
			t.Fatal("empty record omitted identity or access evidence")
		}
	}
	scanner := treeScanner{ctx: t.Context(), root: root, identities: map[string]string{}}
	entry, err := scanner.entry(name)
	if err != nil || entry.Size != 0 || entry.SHA256 != want || scanner.identities[name] == "" || entry.AccessSHA256 == "" {
		t.Fatalf("inspect locked empty entry: %+v, %v", entry, err)
	}
	contender := flock.New(filepath.Join(path, name))
	t.Cleanup(func() { _ = contender.Close() })
	if locked, err := contender.TryLock(); err != nil || locked {
		t.Fatalf("read changed writer exclusion: %v, %v", locked, err)
	}
	if err := writer.Unlock(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, name), []byte("changed"), fileMode); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readWorkspaceRecord(root, name, 0); err == nil {
		t.Fatal("zero byte limit accepted changed contents")
	}
	changed, err := scanner.entry(name)
	if err != nil || changed.SHA256 == want || changed.Size != int64(len("changed")) {
		t.Fatalf("inspection missed changed contents: %+v, %v", changed, err)
	}
}
