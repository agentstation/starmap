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

type providerModelIdentityManifest struct {
	SchemaVersion int                           `yaml:"schema_version"`
	Records       []providerModelIdentityRecord `yaml:"records"`
}

type providerModelIdentityRecord struct {
	Path            string `yaml:"path"`
	Provider        string `yaml:"provider"`
	ProviderModelID string `yaml:"provider_model_id"`
	Model           string `yaml:"model"`
	Status          string `yaml:"status"`
	Evidence        string `yaml:"evidence"`
}

func TestEmbeddedProviderModelsMatchReviewedIdentityMap(t *testing.T) {
	manifest := loadProviderModelIdentityManifest(t)
	if manifest.SchemaVersion != 1 {
		t.Fatalf("manifest schema version = %d, want 1", manifest.SchemaVersion)
	}
	if len(manifest.Records) != 610 {
		t.Fatalf("manifest records = %d, want 610", len(manifest.Records))
	}

	seen := make(map[string]struct{}, len(manifest.Records))
	linked, unlinked := 0, 0
	for _, record := range manifest.Records {
		if _, duplicate := seen[record.Path]; duplicate {
			t.Fatalf("duplicate provider model path %q", record.Path)
		}
		seen[record.Path] = struct{}{}
		if record.Evidence == "" {
			t.Fatalf("%s has no identity evidence or exclusion reason", record.Path)
		}
		data, err := fs.ReadFile(embedded.FS, "catalog/"+record.Path)
		if err != nil {
			t.Fatalf("read %s: %v", record.Path, err)
		}
		var model catalogs.Model
		if err := yaml.Unmarshal(data, &model); err != nil {
			t.Fatalf("decode %s: %v", record.Path, err)
		}
		parts := strings.Split(record.Path, "/")
		if len(parts) < 4 || parts[0] != "providers" || parts[1] != record.Provider ||
			model.ID != record.ProviderModelID {
			t.Fatalf("%s identity does not match record %#v and model %q", record.Path, record, model.ID)
		}
		if string(model.ModelRef) != record.Model {
			t.Fatalf("%s model reference = %q, want %q", record.Path, model.ModelRef, record.Model)
		}
		switch record.Status {
		case "linked":
			linked++
			identity := strings.SplitN(record.Model, "/", 2)
			if len(identity) != 2 || identity[0] == "" || identity[1] == "" {
				t.Fatalf("%s has invalid canonical model %q", record.Path, record.Model)
			}
			target := "catalog/authors/" + identity[0] + "/models/" + identity[1] + ".yaml"
			if _, err := fs.Stat(embedded.FS, target); err != nil {
				t.Fatalf("%s references missing %s: %v", record.Path, target, err)
			}
		case "unlinked":
			unlinked++
			if record.Model != "" || model.ModelRef != "" {
				t.Fatalf("%s unlinked record has model %q", record.Path, record.Model)
			}
		default:
			t.Fatalf("%s has unknown status %q", record.Path, record.Status)
		}
	}
	if linked != 610 || unlinked != 0 {
		t.Fatalf("identity disposition = %d linked, %d unlinked; want 610/0", linked, unlinked)
	}

	providerFiles := 0
	err := fs.WalkDir(embedded.FS, "catalog/providers", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.Contains(path, "/models/") && strings.HasSuffix(path, ".yaml") {
			providerFiles++
			relative := strings.TrimPrefix(path, "catalog/")
			if _, found := seen[relative]; !found {
				t.Fatalf("provider model %q has no reviewed identity disposition", relative)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk provider models: %v", err)
	}
	if providerFiles != len(manifest.Records) {
		t.Fatalf("provider files = %d, manifest records = %d", providerFiles, len(manifest.Records))
	}
}

func TestEmbeddedAuthoredModelsAreCoveredByHistoricalOrServingIdentityReview(t *testing.T) {
	historical := loadAuthorCorpusManifest(t)
	serving := loadProviderModelIdentityManifest(t)
	reviewed := make(map[string]struct{}, len(historical.Records)+len(serving.Records))
	for _, record := range historical.Records {
		reviewed[record.Path] = struct{}{}
	}
	for _, record := range serving.Records {
		if record.Status != "linked" {
			continue
		}
		identity := strings.SplitN(record.Model, "/", 2)
		if len(identity) != 2 {
			t.Fatalf("%s has invalid canonical model %q", record.Path, record.Model)
		}
		reviewed["authors/"+identity[0]+"/models/"+identity[1]+".yaml"] = struct{}{}
	}

	actual := make(map[string]struct{}, len(reviewed))
	err := fs.WalkDir(embedded.FS, "catalog/authors", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.Contains(path, "/models/") || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		relative := strings.TrimPrefix(path, "catalog/")
		actual[relative] = struct{}{}
		if _, found := reviewed[relative]; !found {
			t.Fatalf("authored model %q has neither historical nor serving identity review", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded author models: %v", err)
	}
	for path := range reviewed {
		if _, found := actual[path]; !found {
			t.Fatalf("reviewed authored model %q is not embedded", path)
		}
	}
}

func loadProviderModelIdentityManifest(t testing.TB) providerModelIdentityManifest {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "reviews", "P5_PROVIDER_MODEL_IDENTITY_MAP_2026-07-28.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read provider identity manifest: %v", err)
	}
	var manifest providerModelIdentityManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode provider identity manifest: %v", err)
	}
	return manifest
}
