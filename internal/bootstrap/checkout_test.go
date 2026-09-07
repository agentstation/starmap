package bootstrap

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/embedded"
)

func TestEmbeddedCatalogCheckoutPreservesManifestInputs(t *testing.T) {
	repository := filepath.Join("..", "..")
	list := exec.CommandContext(t.Context(), "git", "ls-files", "-z", "internal/embedded/catalog")
	list.Dir = repository
	paths, err := list.Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("checkout contains no tracked catalog inputs")
	}
	checkout := t.TempDir()
	command := exec.CommandContext(t.Context(), "git", "-c", "core.autocrlf=true", "checkout-index", "--prefix="+filepath.ToSlash(checkout)+"/", "-z", "--stdin")
	command.Dir = repository
	command.Stdin = bytes.NewReader(paths)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("checkout catalog: %v: %s", err, output)
	}
	count := 0
	if err := fs.WalkDir(embedded.FS, "catalog", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		want, err := embedded.FS.ReadFile(name)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(checkout, "internal", "embedded", filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			t.Errorf("checkout changed digest-bound input %s", name)
		}
		count++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Logf("verified %d catalog input files with core.autocrlf=true", count)
}
