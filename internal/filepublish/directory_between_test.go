package filepublish

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryBetweenRootsPreservesExistingDestination(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"from", "to", "from/tree", "to/tree"} {
		if err := os.Mkdir(filepath.Join(base, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	from, err := os.OpenRoot(filepath.Join(base, "from"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = from.Close() }()
	to, err := os.OpenRoot(filepath.Join(base, "to"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = to.Close() }()
	before, err := to.Stat("tree")
	if err != nil {
		t.Fatal(err)
	}
	if err := DirectoryBetweenRootsNoReplace(from, "tree", to, "tree"); !os.IsExist(err) {
		t.Fatal("existing destination", err)
	}
	after, err := to.Stat("tree")
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("destination changed", err)
	}
	if err := to.Remove("tree"); err != nil {
		t.Fatal(err)
	}
	if err := DirectoryBetweenRootsNoReplace(from, "tree", to, "tree"); err != nil {
		t.Fatal(err)
	}
	if _, err := from.Stat("tree"); !os.IsNotExist(err) {
		t.Fatal("source remains", err)
	}
	if _, err := to.Stat("tree"); err != nil {
		t.Fatal(err)
	}
}
