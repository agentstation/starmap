package bootstrap

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedAuthorAndProviderModelIntegrity(t *testing.T) {
	authorData, err := fs.ReadFile(embedded.FS, "catalog/authors.yaml")
	if err != nil {
		t.Fatalf("read authors.yaml: %v", err)
	}
	var authors []catalogs.Author
	if err := yaml.Unmarshal(authorData, &authors); err != nil {
		t.Fatalf("decode authors.yaml: %v", err)
	}
	authorSet := make(map[catalogs.AuthorID]bool, len(authors))
	for _, author := range authors {
		authorSet[author.ID] = true
	}

	authorModels := make(map[string]bool)
	err = fs.WalkDir(embedded.FS, "catalog/authors", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.Contains(path, "/models/") || !strings.HasSuffix(path, ".yaml") {
			return walkErr
		}
		data, err := fs.ReadFile(embedded.FS, path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var model catalogs.Model
		if err := yaml.Unmarshal(data, &model); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		parts := strings.Split(path, "/")
		if len(parts) < 5 || !authorSet[catalogs.AuthorID(parts[2])] {
			t.Fatalf("%s references undeclared author", path)
		}
		slug := strings.TrimSuffix(parts[len(parts)-1], ".yaml")
		authorModels[parts[2]+"/"+slug] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk authors: %v", err)
	}
	if len(authorModels) == 0 {
		t.Fatal("embedded catalog has no author models")
	}

	providerModelCount := 0
	err = fs.WalkDir(embedded.FS, "catalog/providers", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.Contains(path, "/models/") || !strings.HasSuffix(path, ".yaml") {
			return walkErr
		}
		data, err := fs.ReadFile(embedded.FS, path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var model catalogs.Model
		if err := yaml.Unmarshal(data, &model); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		providerModelCount++
		if ref := string(model.ModelRef); ref != "" && !authorModels[ref] {
			t.Fatalf("%s references missing canonical model %q", path, ref)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk providers: %v", err)
	}
	if providerModelCount == 0 {
		t.Fatal("embedded catalog has no provider models")
	}
}
