package bootstrap

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
)

const embeddedAuthorModelCount = 322

type authorCorpusManifest struct {
	SchemaVersion int                  `yaml:"schema_version"`
	Records       []authorCorpusRecord `yaml:"records"`
}

type authorCorpusRecord struct {
	Path                 string `yaml:"path"`
	Author               string `yaml:"author"`
	Slug                 string `yaml:"slug"`
	CanonicalModel       string `yaml:"canonical_model"`
	Disposition          string `yaml:"disposition"`
	ProviderIDMatchCount int    `yaml:"provider_id_match_count"`
}

func TestEmbeddedAuthorModelCorpusHasExactReviewedDisposition(t *testing.T) {
	manifest := loadAuthorCorpusManifest(t)
	if manifest.SchemaVersion != 1 {
		t.Fatalf("manifest schema version = %d, want 1", manifest.SchemaVersion)
	}
	if len(manifest.Records) != embeddedAuthorModelCount {
		t.Fatalf("manifest records = %d, want %d", len(manifest.Records), embeddedAuthorModelCount)
	}

	reviewed := make(map[string]authorCorpusRecord, len(manifest.Records))
	withoutExactProviderID := 0
	for _, record := range manifest.Records {
		if record.Disposition != "keep" {
			t.Fatalf("%s disposition = %q, want keep", record.Path, record.Disposition)
		}
		if _, duplicate := reviewed[record.Path]; duplicate {
			t.Fatalf("duplicate manifest path %q", record.Path)
		}
		reviewed[record.Path] = record
		if record.ProviderIDMatchCount == 0 {
			withoutExactProviderID++
		}
	}
	if withoutExactProviderID != 121 {
		t.Fatalf("records without exact provider ID = %d, want 121", withoutExactProviderID)
	}

	reviewedPaths := make(map[string]struct{}, embeddedAuthorModelCount)
	err := fs.WalkDir(embedded.FS, "catalog/authors", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.Contains(path, "/models/") || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, readErr := fs.ReadFile(embedded.FS, path)
		if readErr != nil {
			return readErr
		}
		var model catalogs.Model
		if decodeErr := yaml.Unmarshal(data, &model); decodeErr != nil {
			return decodeErr
		}
		relative := strings.TrimPrefix(path, "catalog/")
		assertAuthoredModelRecord(t, relative, model)

		record, found := reviewed[relative]
		if !found {
			return nil
		}
		parts := strings.Split(relative, "/")
		if record.Author != parts[1] || record.Slug != model.ID ||
			record.CanonicalModel != record.Author+"/"+record.Slug {
			t.Fatalf("manifest identity for %q = %#v, model ID %q", relative, record, model.ID)
		}
		reviewedPaths[relative] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded author models: %v", err)
	}
	if len(reviewedPaths) != embeddedAuthorModelCount {
		t.Fatalf("reviewed embedded author models = %d, want %d", len(reviewedPaths), embeddedAuthorModelCount)
	}
	for path := range reviewed {
		if _, found := reviewedPaths[path]; !found {
			t.Fatalf("reviewed author model %q is not embedded", path)
		}
	}
}

func loadAuthorCorpusManifest(t testing.TB) authorCorpusManifest {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "reviews", "P5_AUTHOR_MODEL_CORPUS_MAP_2026-07-28.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read author corpus manifest: %v", err)
	}
	var manifest authorCorpusManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode author corpus manifest: %v", err)
	}
	return manifest
}

func assertAuthoredModelRecord(t testing.TB, path string, model catalogs.Model) {
	t.Helper()
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[0] != "authors" || parts[2] != "models" {
		t.Fatalf("invalid author model path %q", path)
	}
	if len(model.Authors) == 0 || string(model.Authors[0].ID) != parts[1] {
		t.Fatalf("%s primary author = %#v, want %q", path, model.Authors, parts[1])
	}
	if model.Status != "" || model.Pricing != nil || model.Limits != nil || len(model.Modes) != 0 {
		t.Fatalf("%s contains provider-serving status, price, limits, or modes", path)
	}
	for source := range model.Extensions {
		if source != "models.dev" {
			t.Fatalf("%s contains provider extension %q", path, source)
		}
	}
}
