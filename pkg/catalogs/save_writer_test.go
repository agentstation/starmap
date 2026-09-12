package catalogs

import (
	"bytes"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestWriteYAMLRoundTripWithoutFilesystemWrites(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	builder, err := New(WithFS(testFS()))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	if err := builder.WriteYAML(func(name string, data []byte) error {
		if !filepath.IsLocal(name) {
			t.Fatalf("nonlocal emitted path: %q", name)
		}
		if _, exists := files[name]; exists {
			t.Fatalf("duplicate emitted path: %q", name)
		}
		files[name] = bytes.Clone(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("callback serialization wrote filesystem entries: %v, %v", entries, err)
	}
	memory := make(fstest.MapFS)
	for name, data := range files {
		memory[filepath.ToSlash(name)] = &fstest.MapFile{Data: data}
	}
	reloaded, err := New(WithFS(memory))
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range builder.providers.List() {
		actual, err := reloaded.Provider(provider.ID)
		if err != nil || actual.Name != provider.Name || !bytes.Equal(actual.Logo, provider.Logo) {
			t.Fatalf("emitted provider changed: %s, %v", provider.ID, err)
		}
		for id, model := range provider.Models {
			actual, err := reloaded.ProviderModel(provider.ID, id)
			if err != nil || actual.Name != model.Name {
				t.Fatalf("emitted model changed: %s, %v", id, err)
			}
		}
	}
	for _, author := range builder.authors.List() {
		actual, err := reloaded.Author(author.ID)
		if err != nil || actual.Name != author.Name || !bytes.Equal(actual.Logo, author.Logo) {
			t.Fatalf("emitted author changed: %s, %v", author.ID, err)
		}
	}
	if err := builder.SaveTo(root); err != nil {
		t.Fatal(err)
	}
	count := 0
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		want, found := files[name]
		if !found || !bytes.Equal(data, want) {
			t.Fatalf("emitted and saved bytes differ: %s", name)
		}
		count++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if count == 0 || count != len(files) {
		t.Fatalf("saved %d files, emitted %d", count, len(files))
	}
}

func TestWriteYAMLRequiresWriterAndStopsOnFailure(t *testing.T) {
	builder, err := New(WithFS(testFS()))
	if err != nil {
		t.Fatal(err)
	}
	var validation *pkgerrors.ValidationError
	if err := builder.WriteYAML(nil); !stderrors.As(err, &validation) {
		t.Fatalf("nil writer: %v", err)
	}
	fault := stderrors.New("writer stopped")
	calls := 0
	err = builder.WriteYAML(func(string, []byte) error { calls++; return fault })
	if calls != 1 || !stderrors.Is(err, fault) {
		t.Fatalf("writer failure: %d calls, %v", calls, err)
	}
}
